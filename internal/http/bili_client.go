package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type BiliClient struct {
	httpClient  *nethttp.Client
	headers     map[string]string
	rateLimiter *marketRateLimiter
}

const (
	POST = "POST"
	GET  = "GET"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

func NewBiliClient() (*BiliClient, error) {
	headers := map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      defaultUserAgent,
	}
	transport := &HeaderTransport{
		headers: headers,
		rt:      nethttp.DefaultTransport,
	}

	return &BiliClient{
		httpClient: &nethttp.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
		headers:     headers,
		rateLimiter: defaultMarketRateLimiter,
	}, nil
}

type HeaderTransport struct {
	headers map[string]string
	rt      nethttp.RoundTripper
}

func (t *HeaderTransport) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	for key, value := range t.headers {
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	return t.rt.RoundTrip(req)
}

// SendRequest keeps backwards compatibility with the older request helper.
func (c *BiliClient) SendRequest(method, rawURL string, data map[string]interface{}, respObjRef any) error {
	return c.DoJSON(context.Background(), method, rawURL, nil, data, nil, respObjRef)
}

func (c *BiliClient) DoJSON(
	ctx context.Context,
	method string,
	rawURL string,
	query neturl.Values,
	data any,
	headers map[string]string,
	respObjRef any,
) error {
	var body io.Reader
	if data != nil {
		dataStr, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}
		body = bytes.NewBuffer(dataStr)
	}

	req, err := nethttp.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}
	for key, value := range headers {
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	rateLimitIdentity, err := c.rateLimiter.Wait(ctx, req)
	if err != nil {
		return fmt.Errorf("request canceled while waiting for a safe request slot: %w", err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if res == nil || res.Body == nil {
		return fmt.Errorf("request failed: empty response body")
	}
	defer res.Body.Close()

	resp, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	trimmed := strings.TrimSpace(string(resp))
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(res.Header.Get("Content-Type"), ";")[0]))
	if res.StatusCode == nethttp.StatusTooManyRequests || looksLikeRateLimitPage(trimmed) {
		retryAfter := parseRetryAfter(res.Header.Get("Retry-After"), time.Now())
		retryAfter = c.rateLimiter.Penalize(rateLimitIdentity, retryAfter)
		return &HTTPStatusError{
			StatusCode: nethttp.StatusTooManyRequests,
			RetryAfter: retryAfter,
			Message:    "too many requests",
		}
	}
	if res.StatusCode < nethttp.StatusOK || res.StatusCode >= nethttp.StatusMultipleChoices {
		return &HTTPStatusError{
			StatusCode: res.StatusCode,
			RetryAfter: parseRetryAfter(res.Header.Get("Retry-After"), time.Now()),
			Message:    responseSnippet(resp),
		}
	}
	c.rateLimiter.MarkSuccess(rateLimitIdentity)
	if respObjRef == nil {
		return nil
	}
	if strings.HasPrefix(trimmed, "<") || contentType == "text/html" || contentType == "application/xhtml+xml" {
		log.Warn().
			Int("status", res.StatusCode).
			Str("contentType", contentType).
			Str("text", responseSnippet(resp)).
			Msg("upstream returned non-JSON response")
		return fmt.Errorf(
			"failed to decode response: upstream returned non-JSON content (status %d, content-type %q): %s",
			res.StatusCode,
			contentType,
			responseSnippet(resp),
		)
	}
	if err = json.Unmarshal(resp, respObjRef); err != nil {
		log.Error().Str("text", responseSnippet(resp)).Msg("error response text")
		return fmt.Errorf("failed to decode response: %w (body: %s)", err, responseSnippet(resp))
	}
	return nil
}

func looksLikeRateLimitPage(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "too many requests") ||
		strings.Contains(body, "错误号: 429") ||
		strings.Contains(body, "错误：429") ||
		strings.Contains(body, "请求过于频繁")
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := nethttp.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}

func responseSnippet(body []byte) string {
	const maxLength = 300
	text := strings.TrimSpace(string(body))
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

func (c *BiliClient) StoreHeader(key, value string) {
	c.headers[key] = value
}

func (c *BiliClient) HTTPClient() *nethttp.Client {
	return c.httpClient
}
