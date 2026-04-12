package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

type BiliClient struct {
	httpClient *http.Client
	headers    map[string]string
}

const (
	POST = "POST"
	GET  = "GET"
)

func NewBiliClient() (*BiliClient, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"user-agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
	}
	transport := &HeaderTransport{
		headers: headers,
		rt:      http.DefaultTransport,
	}

	return &BiliClient{
		httpClient: &http.Client{Transport: transport},
		headers:    headers,
	}, nil
}

type HeaderTransport struct {
	headers map[string]string
	rt      http.RoundTripper
}

func (t *HeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.rt.RoundTrip(req)
}

// SendRequest body json
func (c *BiliClient) SendRequest(method, url string, data map[string]interface{}, respObjRef any) error {
	dataStr, _ := json.Marshal(data)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(dataStr))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
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
	err = json.Unmarshal(resp, respObjRef)
	if err != nil {
		log.Error().Str("text", string(resp)).Msg("error response text")
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
func (c *BiliClient) StoreHeader(key, value string) {
	c.headers[key] = value
}
