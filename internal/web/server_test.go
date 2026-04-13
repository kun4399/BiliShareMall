package web

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcore "github.com/mikumifa/BiliShareMall/internal/app"
	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/events"
)

type stubAPI struct {
	bus *events.Bus
}

func (s *stubAPI) GetLoginKeyAndUrl() appcore.LoginInfo { return appcore.LoginInfo{} }
func (s *stubAPI) VerifyLogin(loginKey string) appcore.VerifyLoginResponse {
	return appcore.VerifyLoginResponse{}
}
func (s *stubAPI) ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) (appcore.C2CItemGroupListVO, error) {
	return appcore.C2CItemGroupListVO{
		Items: []appcore.C2CItemGroupVO{
			{SkuID: 1001, C2CItemsName: "测试商品"},
		},
		Total:       1,
		TotalPages:  1,
		CurrentPage: page,
	}, nil
}
func (s *stubAPI) GetC2CItemNameBySku(skuID int64) (string, error) {
	return "测试商品", nil
}
func (s *stubAPI) ListC2CItemDetailBySku(skuID int64, page, pageSize int, sortOption int, statusFilter, cookieStr string) (appcore.C2CItemDetailListVO, error) {
	return appcore.C2CItemDetailListVO{}, nil
}
func (s *stubAPI) ReadAllScrapyItems() []dao.ScrapyItem {
	return []dao.ScrapyItem{{Id: 9, ProductName: "测试"}}
}
func (s *stubAPI) DeleteScrapyItem(id int) error { return nil }
func (s *stubAPI) CreateScrapyItem(item dao.ScrapyItem) int64 {
	return 42
}
func (s *stubAPI) StartTask(taskID int, cookies string) error { return nil }
func (s *stubAPI) DoneTask(taskID int) error                  { return nil }
func (s *stubAPI) GetRunningTaskIds() []int                   { return []int{1, 2} }
func (s *stubAPI) GetMarketRuntimeConfig(cookieStr string) appcore.MarketRuntimeConfig {
	return appcore.MarketRuntimeConfig{}
}
func (s *stubAPI) GetMonitorConfig() appcore.MonitorConfig { return appcore.MonitorConfig{} }
func (s *stubAPI) SaveMonitorConfig(config appcore.MonitorConfig) error {
	return nil
}
func (s *stubAPI) ListMonitorRuleHits(limitPerRule int) []appcore.MonitorHitGroup {
	return []appcore.MonitorHitGroup{}
}
func (s *stubAPI) SubscribeEvents(buffer int) (<-chan events.Event, func(), error) {
	ch, cancel := s.bus.Subscribe(buffer)
	return ch, cancel, nil
}

func TestCatalogItemsEndpoint(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/items?page=2&pageSize=12", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"skuId":1001`) {
		t.Fatalf("expected response body to contain skuId, got %s", body)
	}
	if !strings.Contains(body, `"currentPage":2`) {
		t.Fatalf("expected response body to contain currentPage, got %s", body)
	}
}

func TestCatalogItemsRejectsInvalidPage(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/items?page=bad", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"message":"invalid page"`) {
		t.Fatalf("expected invalid page message, got %s", recorder.Body.String())
	}
}

func TestCreateScrapyTaskEndpoint(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/scrapy/tasks", strings.NewReader(`{"product":"123","productName":"测试任务"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"id":42`) {
		t.Fatalf("expected created id in body, got %s", recorder.Body.String())
	}
}

func TestEventsEndpointStreamsSSE(t *testing.T) {
	api := &stubAPI{bus: events.NewBus()}
	staticRoot := newStaticRoot(t)
	server := httptest.NewServer(NewServer(api, staticRoot).Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request error: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect events endpoint error: %v", err)
	}
	defer resp.Body.Close()

	go func() {
		time.Sleep(100 * time.Millisecond)
		api.bus.Emit("scrapy_finished", map[string]any{"taskId": 7})
	}()

	reader := bufio.NewReader(resp.Body)

	var lines []string
	deadline := time.Now().Add(2 * time.Second)
	for len(lines) < 2 && time.Now().Before(deadline) {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read sse line error: %v", readErr)
		}
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ":") {
			lines = append(lines, line)
		}
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 sse lines, got %v", lines)
	}
	if lines[0] != "event: scrapy_finished" {
		t.Fatalf("unexpected event line: %s", lines[0])
	}
	if lines[1] != `data: {"taskId":7}` {
		t.Fatalf("unexpected data line: %s", lines[1])
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(&stubAPI{bus: events.NewBus()}, newStaticRoot(t))
}

func newStaticRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	indexPath := filepath.Join(root, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>ok</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html error: %v", err)
	}
	return root
}

func TestSPAHandlerFallsBackToIndex(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/home/detail", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<html>") {
		t.Fatalf("expected index html body, got %s", recorder.Body.String())
	}
}

func TestCatalogSkuNameEndpoint(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/sku/1001/name", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	expected := fmt.Sprintf(`"name":"%s"`, "测试商品")
	if !strings.Contains(recorder.Body.String(), expected) {
		t.Fatalf("expected response body to contain %s, got %s", expected, recorder.Body.String())
	}
}
