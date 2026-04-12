package http

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
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
