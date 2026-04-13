package scrapy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type DingTalkNotifier interface {
	SendMarkdown(ctx context.Context, webhook, title, text string) error
}

type HTTPDingTalkNotifier struct {
	client *http.Client
}

func NewHTTPDingTalkNotifier() *HTTPDingTalkNotifier {
	return &HTTPDingTalkNotifier{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *HTTPDingTalkNotifier) SendMarkdown(ctx context.Context, webhook, title, text string) error {
	body, err := json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func buildDingTalkMarkdown(name, displayPrice, link string) string {
	safeName := strings.TrimSpace(name)
	if safeName == "" {
		safeName = "未知商品"
	}
	safePrice := strings.TrimSpace(displayPrice)
	if safePrice == "" {
		safePrice = "-"
	}
	return fmt.Sprintf("### 市集助手\n- 商品：%s\n- 价格：%s 元\n- 链接：[查看商品](%s)", safeName, safePrice, link)
}
