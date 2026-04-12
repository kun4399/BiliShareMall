package http

import (
	"context"
	"io"
	nethttp "net/http"
	neturl "net/url"
	"strings"
	"testing"
)

func TestDoJSONInjectsQueryAndHeaders(t *testing.T) {
	var gotCookie string
	var gotOrigin string
	var gotQuery string
	var gotBody string

	client := &BiliClient{
		httpClient: &nethttp.Client{
			Transport: roundTripFunc(func(req *nethttp.Request) (*nethttp.Response, error) {
				gotCookie = req.Header.Get("Cookie")
				gotOrigin = req.Header.Get("Origin")
				gotQuery = req.URL.RawQuery
				body, _ := io.ReadAll(req.Body)
				gotBody = string(body)
				return &nethttp.Response{
					StatusCode: nethttp.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"code":0}`)),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	var resp struct {
		Code int `json:"code"`
	}
	err := client.DoJSON(
		context.Background(),
		POST,
		"https://example.com/resource",
		mustQuery("csrf", "token"),
		map[string]any{"foo": "bar"},
		map[string]string{"Cookie": "SESSDATA=abc", "Origin": "https://mall.bilibili.com"},
		&resp,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCookie != "SESSDATA=abc" {
		t.Fatalf("unexpected cookie header: %s", gotCookie)
	}
	if gotOrigin != "https://mall.bilibili.com" {
		t.Fatalf("unexpected origin header: %s", gotOrigin)
	}
	if gotQuery != "csrf=token" {
		t.Fatalf("unexpected query: %s", gotQuery)
	}
	if !strings.Contains(gotBody, `"foo":"bar"`) {
		t.Fatalf("unexpected request body: %s", gotBody)
	}
	if resp.Code != 0 {
		t.Fatalf("unexpected response code: %d", resp.Code)
	}
}

func TestMapLoginPollResult(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		status  LoginStatus
		wantErr bool
	}{
		{name: "pending", code: 86101, status: LoginStatusPending},
		{name: "scanned", code: 86090, status: LoginStatusScanned},
		{name: "expired", code: 86038, status: LoginStatusExpired},
		{name: "confirmed", code: 0, status: LoginStatusConfirmed},
		{name: "error", code: 99999, status: LoginStatusError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapLoginPollResult(qrPollResponse{Data: struct {
				Code         int    `json:"code"`
				Message      string `json:"message"`
				URL          string `json:"url"`
				RefreshToken string `json:"refresh_token"`
			}{Code: tt.code, Message: tt.name}})
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tt.status {
				t.Fatalf("unexpected status: %s", result.Status)
			}
		})
	}
}

func TestParseBiliSessionExtractsCSRF(t *testing.T) {
	session := ParseBiliSession("SESSDATA=abc; bili_jct=token; DedeUserID=1")
	if !session.IsLoggedIn() {
		t.Fatal("expected session to be logged in")
	}
	if session.CSRF() != "token" {
		t.Fatalf("unexpected csrf: %s", session.CSRF())
	}
}

func TestCheckC2CItemUsesSessionAndPrice(t *testing.T) {
	var gotQuery string
	var gotBody string

	client := &BiliClient{
		httpClient: &nethttp.Client{
			Transport: roundTripFunc(func(req *nethttp.Request) (*nethttp.Response, error) {
				gotQuery = req.URL.RawQuery
				body, _ := io.ReadAll(req.Body)
				gotBody = string(body)
				return &nethttp.Response{
					StatusCode: nethttp.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"message":"ok"}`)),
				}, nil
			}),
		},
		headers: map[string]string{},
	}

	session := ParseBiliSession("SESSDATA=abc; DedeUserID=1; bili_jct=token; buvid3=b3; buvid4=b4")
	resp, err := client.CheckC2CItem(context.Background(), session, 123, 4567)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "platform=h5" {
		t.Fatalf("unexpected query: %s", gotQuery)
	}
	if !strings.Contains(gotBody, `"c2cItemsId":123`) || !strings.Contains(gotBody, `"price":4567`) {
		t.Fatalf("unexpected request body: %s", gotBody)
	}
	if resp.Code != 0 {
		t.Fatalf("unexpected response code: %d", resp.Code)
	}
}

func mustQuery(key, value string) neturl.Values {
	values := neturl.Values{}
	values.Set(key, value)
	return values
}
