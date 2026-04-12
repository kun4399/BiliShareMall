package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	neturl "net/url"
	"time"

	"github.com/rs/zerolog/log"
)

type BiliClient struct {
	httpClient *nethttp.Client
	headers    map[string]string
}

const (
	POST = "POST"
	GET  = "GET"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

func NewBiliClient() (*BiliClient, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/plain, */*",
		"User-Agent":   defaultUserAgent,
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
		headers: headers,
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
	if respObjRef == nil {
		return nil
	}
	if err = json.Unmarshal(resp, respObjRef); err != nil {
		log.Error().Str("text", string(resp)).Msg("error response text")
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

func (c *BiliClient) StoreHeader(key, value string) {
	c.headers[key] = value
}

func (c *BiliClient) HTTPClient() *nethttp.Client {
	return c.httpClient
}
