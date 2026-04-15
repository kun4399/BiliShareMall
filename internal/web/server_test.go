package web

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcore "github.com/kun4399/BiliShareMall/internal/app"
	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/events"
)

type stubAPI struct {
	bus                     *events.Bus
	sharedSession           appcore.SharedLoginSession
	resolvedCookie          string
	lastResolvedHeader      string
	lastDetailCookie        string
	lastRuntimeConfigCookie string
	accounts                []appcore.LoginAccount
}

func (s *stubAPI) GetLoginKeyAndUrl() appcore.LoginInfo { return appcore.LoginInfo{} }
func (s *stubAPI) VerifyLogin(loginKey string) appcore.VerifyLoginResponse {
	return appcore.VerifyLoginResponse{}
}
func (s *stubAPI) GetSharedLoginSession() appcore.SharedLoginSession {
	return s.sharedSession
}
func (s *stubAPI) ClearSharedLoginSession() error {
	s.sharedSession = appcore.SharedLoginSession{}
	return nil
}
func (s *stubAPI) ListLoginAccounts() []appcore.LoginAccount {
	return append([]appcore.LoginAccount(nil), s.accounts...)
}
func (s *stubAPI) DeleteLoginAccount(id int64) error {
	filtered := make([]appcore.LoginAccount, 0, len(s.accounts))
	for _, item := range s.accounts {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	s.accounts = filtered
	return nil
}
func (s *stubAPI) ClearAllLoginAccounts() error {
	s.accounts = []appcore.LoginAccount{}
	return nil
}
func (s *stubAPI) ResolveLoginCookie(cookieHeader string) string {
	s.lastResolvedHeader = cookieHeader
	if cookieHeader != "" {
		return cookieHeader
	}
	return s.resolvedCookie
}
func (s *stubAPI) ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) (appcore.C2CItemGroupListVO, error) {
	return appcore.C2CItemGroupListVO{
		Items: []appcore.C2CItemGroupVO{
			{SkuID: 1001, C2CItemsName: "测试商品", ReferencePriceLabel: "参考价 129.00 元"},
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
	s.lastDetailCookie = cookieStr
	return appcore.C2CItemDetailListVO{}, nil
}
func (s *stubAPI) ReadAllScrapyItems() []dao.ScrapyItem {
	return []dao.ScrapyItem{{Id: 9, ProductName: "测试"}}
}
func (s *stubAPI) DeleteScrapyItem(id int) error { return nil }
func (s *stubAPI) CreateScrapyItem(item dao.ScrapyItem) int64 {
	return 42
}
func (s *stubAPI) UpdateScrapyTaskConfig(taskID int, accountID int64, requestIntervalSeconds float64) error {
	return nil
}
func (s *stubAPI) StartTask(taskID int, cookies string) error { return nil }
func (s *stubAPI) DoneTask(taskID int) error                  { return nil }
func (s *stubAPI) GetRunningTaskIds() []int                   { return []int{1, 2} }
func (s *stubAPI) GetMarketRuntimeConfig(cookieStr string) appcore.MarketRuntimeConfig {
	s.lastRuntimeConfigCookie = cookieStr
	return appcore.MarketRuntimeConfig{}
}
func (s *stubAPI) GetMonitorConfig() appcore.MonitorConfig {
	return appcore.MonitorConfig{
		Webhook: "https://example.com/webhook",
		Rules: []appcore.MonitorRule{
			{ID: 1, SkuID: 1001, SkuName: "测试商品", MinPrice: 100, MaxPrice: 200, Enabled: true},
		},
	}
}
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
	if !strings.Contains(body, `"referencePriceLabel":"参考价 129.00 元"`) {
		t.Fatalf("expected response body to contain referencePriceLabel, got %s", body)
	}
}

func TestLoginSessionEndpoint(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).sharedSession = appcore.SharedLoginSession{
		LoggedIn:  true,
		UpdatedAt: 1710000000000,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"loggedIn":true`) {
		t.Fatalf("expected loggedIn in body, got %s", body)
	}
}

func TestClearLoginSessionEndpoint(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).sharedSession = appcore.SharedLoginSession{LoggedIn: true}

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/session", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if server.api.(*stubAPI).sharedSession.LoggedIn {
		t.Fatal("expected shared session to be cleared")
	}
}

func TestListLoginAccountsEndpoint(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).accounts = []appcore.LoginAccount{
		{ID: 1, UID: "1001", AccountName: "测试账号", LoggedIn: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/accounts", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"accountName":"测试账号"`) {
		t.Fatalf("expected account in body, got %s", body)
	}
}

func TestDeleteLoginAccountEndpoint(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).accounts = []appcore.LoginAccount{
		{ID: 1, UID: "1001", AccountName: "A"},
		{ID: 2, UID: "1002", AccountName: "B"},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/accounts/1", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if len(server.api.(*stubAPI).accounts) != 1 || server.api.(*stubAPI).accounts[0].ID != 2 {
		t.Fatalf("expected account #1 removed, got %+v", server.api.(*stubAPI).accounts)
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

func TestUpdateScrapyTaskConfigEndpoint(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/scrapy/tasks/42/config", strings.NewReader(`{"accountId":1,"requestIntervalSeconds":0.5}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
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

func TestMonitorConfigEndpointIncludesResolvedSkuName(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/monitor/config", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"skuName":"测试商品"`) {
		t.Fatalf("expected response body to contain skuName, got %s", recorder.Body.String())
	}
}

func TestCatalogDetailEndpointFallsBackToSharedCookie(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).resolvedCookie = "SESSDATA=shared"

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/items/1001?page=1&pageSize=10&sortOption=1", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if server.api.(*stubAPI).lastResolvedHeader != "" {
		t.Fatalf("expected empty request header cookie, got %q", server.api.(*stubAPI).lastResolvedHeader)
	}
	if server.api.(*stubAPI).lastDetailCookie != "SESSDATA=shared" {
		t.Fatalf("expected shared cookie fallback, got %q", server.api.(*stubAPI).lastDetailCookie)
	}
}

func TestCatalogDetailEndpointPrefersRequestCookie(t *testing.T) {
	server := newTestServer(t)
	server.api.(*stubAPI).resolvedCookie = "SESSDATA=shared"

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/items/1001?page=1&pageSize=10&sortOption=1", nil)
	req.Header.Set(biliCookieHeader, "SESSDATA=request")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if server.api.(*stubAPI).lastDetailCookie != "SESSDATA=request" {
		t.Fatalf("expected request cookie, got %q", server.api.(*stubAPI).lastDetailCookie)
	}
}

func TestImageProxyRejectsMissingURL(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/image", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestImageProxyRejectsDisallowedHost(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/image?url=https://example.com/image.png", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestImageProxyStreamsAllowedImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, "png-data")
	}))
	defer upstream.Close()

	server := newTestServer(t)
	server.imageHTTP = &http.Client{
		Transport: rewriteHostTransport(t, upstream),
	}

	targetURL := strings.Replace(upstream.URL, "127.0.0.1", "img0.hdslb.com", 1)
	req := httptest.NewRequest(http.MethodGet, "/api/assets/image?url="+neturl.QueryEscape(targetURL), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Body.String() != "png-data" {
		t.Fatalf("unexpected proxy body: %s", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("unexpected content type: %s", got)
	}
}

func rewriteHostTransport(t *testing.T, upstream *httptest.Server) http.RoundTripper {
	t.Helper()

	dialAddr := strings.TrimPrefix(upstream.URL, "http://")
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, dialAddr)
	}
	return base
}
