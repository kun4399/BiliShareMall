package scrapy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDingTalkNotifierSendsMarkdownPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	notifier := NewHTTPDingTalkNotifier()
	err := notifier.SendMarkdown(context.Background(), server.URL, "市集助手", "### 市集助手\n- 商品：测试\n- 价格：10 元")
	if err != nil {
		t.Fatalf("SendMarkdown error: %v", err)
	}

	if payload["msgtype"] != "markdown" {
		t.Fatalf("expected msgtype markdown, got %v", payload["msgtype"])
	}
	markdown, ok := payload["markdown"].(map[string]any)
	if !ok {
		t.Fatalf("expected markdown object, got %+v", payload["markdown"])
	}
	if markdown["title"] != "市集助手" {
		t.Fatalf("expected markdown title 市集助手, got %v", markdown["title"])
	}
	if markdown["text"] == "" {
		t.Fatal("expected markdown text to be non-empty")
	}
}

func TestHTTPDingTalkNotifierReturnsErrorWhenErrCodeNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"keywords not in content"}`))
	}))
	defer server.Close()

	notifier := NewHTTPDingTalkNotifier()
	err := notifier.SendMarkdown(context.Background(), server.URL, "市集助手", "### 市集助手")
	if err == nil {
		t.Fatal("expected SendMarkdown to fail when errcode is non-zero")
	}
}
