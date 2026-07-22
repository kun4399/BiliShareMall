package http

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSendRequestReturnsErrorWhenDoFails(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("network unreachable")
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendRequestReturnsDecodeErrorWhenBodyIsEmpty(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendRequestIdentifiesHTMLResponse(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
					Body:       io.NopCloser(strings.NewReader("<html><title>verify</title></html>")),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upstream returned non-JSON content") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendRequestRejectsHTTPErrorBeforeDecoding(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusPreconditionFailed,
					Body:       io.NopCloser(strings.NewReader("<html>blocked</html>")),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	if err == nil || !strings.Contains(err.Error(), "HTTP 412") {
		t.Fatalf("expected HTTP status error, got %v", err)
	}
}

func TestSendRequestReturnsTypedRateLimitError(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"90"}},
					Body:       io.NopCloser(strings.NewReader("<html>Too many requests</html>")),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected HTTPStatusError, got %T: %v", err, err)
	}
	if statusErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", statusErr.StatusCode)
	}
	if statusErr.RetryAfter != 90*time.Second {
		t.Fatalf("expected retry-after 90s, got %s", statusErr.RetryAfter)
	}
	if strings.Contains(err.Error(), "<html>") {
		t.Fatalf("rate limit error should not expose HTML body: %v", err)
	}
}

func TestSendRequestRecognizesRateLimitHTMLWithSuccessfulHTTPStatus(t *testing.T) {
	client := &BiliClient{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Body:       io.NopCloser(strings.NewReader("<!doctype html><div>错误号: 429</div><div>请求过于频繁</div>")),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp any
	err := client.SendRequest(POST, "https://example.com", map[string]interface{}{"k": "v"}, &resp)
	if !IsRateLimitError(err) {
		t.Fatalf("expected rate limit error, got %T: %v", err, err)
	}
	if RetryAfter(err) != initialRateLimitCooldown {
		t.Fatalf("expected default cooldown %s, got %s", initialRateLimitCooldown, RetryAfter(err))
	}
}
