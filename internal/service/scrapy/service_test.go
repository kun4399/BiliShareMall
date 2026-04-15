package scrapy

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/domain"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
	_ "github.com/mattn/go-sqlite3"
)

type mockMarketClient struct {
	fn func(ctx context.Context, req bilihttp.MarketListRequest) (domain.MailListResponse, error)
}

func (m *mockMarketClient) ListMarketItems(ctx context.Context, _ *bilihttp.BiliSession, req bilihttp.MarketListRequest) (domain.MailListResponse, error) {
	if m.fn == nil {
		return domain.MailListResponse{}, errors.New("mock not configured")
	}
	return m.fn(ctx, req)
}

type mockNotifier struct {
	mu      sync.Mutex
	calls   int
	lastErr error
}

func (m *mockNotifier) SendMarkdown(_ context.Context, _ string, _ string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.lastErr
}

func TestServiceSupportsConcurrentTasksAndIndependentStop(t *testing.T) {
	db := newScrapyTestDatabase(t)
	insertScrapyTask(t, db, 1, "sku-a")
	insertScrapyTask(t, db, 2, "sku-b")

	svc := NewService(db, nil)
	svc.requestInterval = 10 * time.Millisecond
	svc.restartRoundDelay = 10 * time.Millisecond
	svc.retryDelay = 15 * time.Millisecond
	svc.notifier = &mockNotifier{}

	var callMu sync.Mutex
	callCount := map[string]int{}
	svc.marketFn = func() (marketClient, error) {
		return &mockMarketClient{
			fn: func(_ context.Context, payload bilihttp.MarketListRequest) (domain.MailListResponse, error) {
				callMu.Lock()
				callCount[payload.CategoryFilter]++
				callMu.Unlock()
				return fakeListResponseWithNext(nil), nil
			},
		}, nil
	}

	if err := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask(1) error: %v", err)
	}
	if err := svc.StartTask(2, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask(2) error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		ids := svc.GetRunningTaskIds()
		return len(ids) == 2 && slices.Contains(ids, 1) && slices.Contains(ids, 2)
	})

	if err := svc.DoneTask(1); err != nil {
		t.Fatalf("DoneTask(1) error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		ids := svc.GetRunningTaskIds()
		return len(ids) == 1 && ids[0] == 2
	})

	if err := svc.DoneTask(2); err != nil {
		t.Fatalf("DoneTask(2) error: %v", err)
	}
	waitUntil(t, 2*time.Second, func() bool {
		return len(svc.GetRunningTaskIds()) == 0
	})
}

func TestServiceRetriesRequestErrorAndKeepsRunning(t *testing.T) {
	db := newScrapyTestDatabase(t)
	insertScrapyTask(t, db, 1, "sku-a")

	var eventsMu sync.Mutex
	events := make([]ScrapyRetryEvent, 0)
	svc := NewService(db, func(eventName string, payload any) {
		if eventName != "scrapy_retry_wait" {
			return
		}
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, payload.(ScrapyRetryEvent))
	})
	svc.requestInterval = 10 * time.Millisecond
	svc.restartRoundDelay = 10 * time.Millisecond
	svc.retryDelay = 20 * time.Millisecond
	svc.notifier = &mockNotifier{}

	var countMu sync.Mutex
	callCount := 0
	svc.marketFn = func() (marketClient, error) {
		return &mockMarketClient{
			fn: func(_ context.Context, _ bilihttp.MarketListRequest) (domain.MailListResponse, error) {
				countMu.Lock()
				defer countMu.Unlock()
				callCount++
				if callCount == 1 {
					return domain.MailListResponse{}, errors.New("request failed: timeout")
				}
				return fakeListResponseWithNext(nil), nil
			},
		}, nil
	}

	if err := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		return len(events) >= 1
	})

	waitUntil(t, 2*time.Second, func() bool {
		countMu.Lock()
		defer countMu.Unlock()
		return callCount >= 2
	})

	if err := svc.DoneTask(1); err != nil {
		t.Fatalf("DoneTask error: %v", err)
	}
}

func TestServiceEmitsRoundFinishedAndRestartsFromFirstPage(t *testing.T) {
	db := newScrapyTestDatabase(t)
	insertScrapyTask(t, db, 1, "sku-a")

	var eventMu sync.Mutex
	rounds := 0
	var transitionMu sync.Mutex
	sawSecondPage := false
	sawRestartFromFirstPage := false
	svc := NewService(db, func(eventName string, payload any) {
		if eventName != "scrapy_round_finished" {
			return
		}
		if payload.(ScrapyRoundEvent).TaskID != 1 {
			return
		}
		eventMu.Lock()
		rounds++
		eventMu.Unlock()
	})
	svc.requestInterval = 10 * time.Millisecond
	svc.restartRoundDelay = 10 * time.Millisecond
	svc.retryDelay = 20 * time.Millisecond
	svc.notifier = &mockNotifier{}

	var countMu sync.Mutex
	callCount := 0
	svc.marketFn = func() (marketClient, error) {
		return &mockMarketClient{
			fn: func(_ context.Context, payload bilihttp.MarketListRequest) (domain.MailListResponse, error) {
				countMu.Lock()
				callCount++
				currentCount := callCount
				countMu.Unlock()
				transitionMu.Lock()
				if payload.NextID != nil && *payload.NextID == "page-2" {
					sawSecondPage = true
				}
				if sawSecondPage && payload.NextID == nil && currentCount >= 3 {
					sawRestartFromFirstPage = true
				}
				transitionMu.Unlock()

				if currentCount%2 == 1 {
					next := "page-2"
					return fakeListResponseWithNext(&next), nil
				}
				return fakeListResponseWithNext(nil), nil
			},
		}, nil
	}

	if err := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		eventMu.Lock()
		defer eventMu.Unlock()
		return rounds >= 1
	})
	waitUntil(t, 2*time.Second, func() bool {
		transitionMu.Lock()
		defer transitionMu.Unlock()
		return sawSecondPage && sawRestartFromFirstPage
	})

	if err := svc.DoneTask(1); err != nil {
		t.Fatalf("DoneTask error: %v", err)
	}
}

func TestServiceStartTaskResetsNumsAndRoundCount(t *testing.T) {
	db := newScrapyTestDatabase(t)
	now := time.Now()
	next := "page-2"
	_, err := db.Db.Exec(
		`INSERT INTO scrapy_items(
			id, price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, account_id, request_interval_seconds, nums, increase_number, next_token, create_time, "order"
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "", "不限", "", "不限", "sku-a", "sku-a", 0, 0, 9, 4, next, now, "TIME_DESC",
	)
	if err != nil {
		t.Fatalf("insert scrapy task failed: %v", err)
	}

	svc := NewService(db, nil)
	svc.requestInterval = 10 * time.Millisecond
	svc.restartRoundDelay = 10 * time.Millisecond
	svc.retryDelay = 20 * time.Millisecond
	svc.notifier = &mockNotifier{}

	requestStarted := make(chan struct{}, 1)
	releaseRequest := make(chan struct{})
	svc.marketFn = func() (marketClient, error) {
		return &mockMarketClient{
			fn: func(ctx context.Context, _ bilihttp.MarketListRequest) (domain.MailListResponse, error) {
				select {
				case requestStarted <- struct{}{}:
				default:
				}

				select {
				case <-ctx.Done():
					return domain.MailListResponse{}, ctx.Err()
				case <-releaseRequest:
					return fakeListResponseWithNext(nil), nil
				}
			},
		}, nil
	}

	if err := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		ids := svc.GetRunningTaskIds()
		return len(ids) == 1 && ids[0] == 1
	})
	waitUntil(t, 2*time.Second, func() bool {
		select {
		case <-requestStarted:
			return true
		default:
			return false
		}
	})

	item, err := db.ReadScrapyItem(1)
	if err != nil {
		t.Fatalf("ReadScrapyItem error: %v", err)
	}
	if item.Nums != 0 {
		t.Fatalf("expected nums reset to 0, got %d", item.Nums)
	}
	if item.IncreaseNumber != 0 {
		t.Fatalf("expected increase_number(reset round count) to 0, got %d", item.IncreaseNumber)
	}
	if item.NextToken != nil {
		t.Fatalf("expected next_token reset to nil, got %v", *item.NextToken)
	}

	if err := svc.DoneTask(1); err != nil {
		t.Fatalf("DoneTask error: %v", err)
	}
	close(releaseRequest)
	waitUntil(t, 2*time.Second, func() bool {
		return len(svc.GetRunningTaskIds()) == 0
	})
}

func TestServiceStartTaskFailsWhenBoundAccountMissing(t *testing.T) {
	db := newScrapyTestDatabase(t)
	now := time.Now()
	_, err := db.Db.Exec(
		`INSERT INTO scrapy_items(
			id, price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, account_id, request_interval_seconds, nums, increase_number, next_token, create_time, "order"
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "", "不限", "", "不限", "sku-a", "sku-a", 9999, 3, 0, 0, "", now, "TIME_DESC",
	)
	if err != nil {
		t.Fatalf("insert scrapy task failed: %v", err)
	}

	svc := NewService(db, nil)
	startErr := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x")
	if startErr == nil {
		t.Fatal("expected StartTask to fail when bound account is missing")
	}
	if !strings.Contains(startErr.Error(), "bound account not found") {
		t.Fatalf("expected missing account error, got %v", startErr)
	}
}

func TestServiceUpdateScrapyTaskConfigPersistsDecimalInterval(t *testing.T) {
	db := newScrapyTestDatabase(t)
	insertScrapyTask(t, db, 1, "sku-a")
	result, err := db.Db.Exec(
		`INSERT INTO auth_accounts(uid, account_name, cookies) VALUES(?, ?, ?)`,
		"1001",
		"账号A",
		"SESSDATA=a;DedeUserID=1001;bili_jct=x",
	)
	if err != nil {
		t.Fatalf("insert auth account failed: %v", err)
	}
	accountID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read auth account id failed: %v", err)
	}

	svc := NewService(db, nil)
	if err := svc.UpdateScrapyTaskConfig(1, accountID, 0.1); err != nil {
		t.Fatalf("UpdateScrapyTaskConfig error: %v", err)
	}

	item, err := db.ReadScrapyItem(1)
	if err != nil {
		t.Fatalf("ReadScrapyItem error: %v", err)
	}
	if item.AccountID != accountID {
		t.Fatalf("expected account id %d, got %d", accountID, item.AccountID)
	}
	if item.RequestIntervalSec != 0.1 {
		t.Fatalf("expected interval 0.1, got %v", item.RequestIntervalSec)
	}
}

func TestServiceCountsCompletedRoundsInsteadOfChangedRows(t *testing.T) {
	db := newScrapyTestDatabase(t)
	insertScrapyTask(t, db, 1, "sku-a")

	var eventMu sync.Mutex
	rounds := 0
	svc := NewService(db, func(eventName string, payload any) {
		if eventName != "scrapy_round_finished" {
			return
		}
		if payload.(ScrapyRoundEvent).TaskID != 1 {
			return
		}
		eventMu.Lock()
		rounds++
		eventMu.Unlock()
	})
	svc.requestInterval = 10 * time.Millisecond
	svc.restartRoundDelay = 10 * time.Millisecond
	svc.retryDelay = 20 * time.Millisecond
	svc.notifier = &mockNotifier{}

	svc.marketFn = func() (marketClient, error) {
		return &mockMarketClient{
			fn: func(_ context.Context, _ bilihttp.MarketListRequest) (domain.MailListResponse, error) {
				// Always finish a round with the same item.
				// First round inserts 1 row, later rounds insert 0 rows.
				// Round count should still keep increasing.
				return fakeListResponseWithNext(nil), nil
			},
		}, nil
	}

	if err := svc.StartTask(1, "SESSDATA=a;DedeUserID=1;bili_jct=x"); err != nil {
		t.Fatalf("StartTask error: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		eventMu.Lock()
		defer eventMu.Unlock()
		return rounds >= 2
	})

	if err := svc.DoneTask(1); err != nil {
		t.Fatalf("DoneTask error: %v", err)
	}
	waitUntil(t, 2*time.Second, func() bool {
		return len(svc.GetRunningTaskIds()) == 0
	})

	item, err := db.ReadScrapyItem(1)
	if err != nil {
		t.Fatalf("ReadScrapyItem error: %v", err)
	}
	if item.IncreaseNumber < 2 {
		t.Fatalf("expected round count >= 2, got %d", item.IncreaseNumber)
	}
}

func TestTrySendMonitorAlertsMatchesAnyDetailSkuAndRecordsSentEvent(t *testing.T) {
	db := newScrapyTestDatabase(t)
	svc := NewService(db, nil)
	notify := &mockNotifier{}
	svc.notifier = notify

	if err := db.SaveMonitorConfig(dao.MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []dao.MonitorRule{
			{SkuID: 2002, MinPrice: 1000, MaxPrice: 2000, Enabled: true, Remark: "multi-sku"},
		},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig error: %v", err)
	}
	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	ruleID := config.Rules[0].ID

	item := domain.MarketItem{
		C2CItemsID:   9001,
		C2CItemsName: "商品A",
		Price:        1500,
		ShowPrice:    "15.00",
		DetailDtoList: []struct {
			BlindBoxID  int    `json:"blindBoxId"`
			ItemsID     int    `json:"itemsId"`
			SkuID       int    `json:"skuId"`
			Name        string `json:"name"`
			Img         string `json:"img"`
			MarketPrice int    `json:"marketPrice"`
			Type        int    `json:"type"`
			IsHidden    bool   `json:"isHidden"`
		}{
			{SkuID: 1001, Name: "规格-1"},
			{SkuID: 2002, Name: "规格-2"},
		},
	}

	svc.trySendMonitorAlerts(7, []domain.MarketItem{item})

	if notify.calls != 1 {
		t.Fatalf("expected notifier called once, got %d", notify.calls)
	}
	var sentCount int
	if err := db.Db.QueryRow(
		`SELECT COUNT(*) FROM monitor_alert_history WHERE rule_id = ? AND c2c_items_id = ? AND sent = 1`,
		ruleID,
		item.C2CItemsID,
	).Scan(&sentCount); err != nil {
		t.Fatalf("query monitor_alert_history error: %v", err)
	}
	if sentCount != 1 {
		t.Fatalf("expected sent history row count 1, got %d", sentCount)
	}

	events, err := db.ReadMonitorAlertEventsByRule(ruleID, 10)
	if err != nil {
		t.Fatalf("ReadMonitorAlertEventsByRule error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one alert event, got %d", len(events))
	}
	if events[0].Status != monitorAlertStatusSent {
		t.Fatalf("expected sent status, got %s", events[0].Status)
	}
	if events[0].SkuID != 2002 {
		t.Fatalf("expected sku 2002, got %d", events[0].SkuID)
	}
}

func TestTrySendMonitorAlertsOnSendFailureReleasesReservationAndStoresFailedEvent(t *testing.T) {
	db := newScrapyTestDatabase(t)

	receivedEvents := make([]MonitorHitItem, 0)
	var eventMu sync.Mutex
	svc := NewService(db, func(eventName string, payload any) {
		if eventName != "monitor_alert_result" {
			return
		}
		eventMu.Lock()
		defer eventMu.Unlock()
		receivedEvents = append(receivedEvents, payload.(MonitorHitItem))
	})
	notify := &mockNotifier{lastErr: errors.New("dingtalk rejected")}
	svc.notifier = notify

	if err := db.SaveMonitorConfig(dao.MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []dao.MonitorRule{
			{SkuID: 3003, MinPrice: 1000, MaxPrice: 2000, Enabled: true, Remark: "send-fail"},
		},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig error: %v", err)
	}
	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	ruleID := config.Rules[0].ID

	item := domain.MarketItem{
		C2CItemsID:   9011,
		C2CItemsName: "商品B",
		Price:        1800,
		ShowPrice:    "18.00",
		DetailDtoList: []struct {
			BlindBoxID  int    `json:"blindBoxId"`
			ItemsID     int    `json:"itemsId"`
			SkuID       int    `json:"skuId"`
			Name        string `json:"name"`
			Img         string `json:"img"`
			MarketPrice int    `json:"marketPrice"`
			Type        int    `json:"type"`
			IsHidden    bool   `json:"isHidden"`
		}{
			{SkuID: 3003, Name: "规格-fail"},
		},
	}

	svc.trySendMonitorAlerts(8, []domain.MarketItem{item})

	var historyCount int
	if err := db.Db.QueryRow(
		`SELECT COUNT(*) FROM monitor_alert_history WHERE rule_id = ? AND c2c_items_id = ?`,
		ruleID,
		item.C2CItemsID,
	).Scan(&historyCount); err != nil {
		t.Fatalf("query monitor_alert_history error: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("expected reservation released on failure, got history count %d", historyCount)
	}

	storedEvents, err := db.ReadMonitorAlertEventsByRule(ruleID, 10)
	if err != nil {
		t.Fatalf("ReadMonitorAlertEventsByRule error: %v", err)
	}
	if len(storedEvents) != 1 {
		t.Fatalf("expected one stored failed event, got %d", len(storedEvents))
	}
	if storedEvents[0].Status != monitorAlertStatusFailed {
		t.Fatalf("expected failed status, got %s", storedEvents[0].Status)
	}
	if !strings.Contains(storedEvents[0].ErrorMessage, "dingtalk rejected") {
		t.Fatalf("expected error message persisted, got %q", storedEvents[0].ErrorMessage)
	}

	eventMu.Lock()
	defer eventMu.Unlock()
	if len(receivedEvents) != 1 {
		t.Fatalf("expected one emitted event, got %d", len(receivedEvents))
	}
	if receivedEvents[0].Status != monitorAlertStatusFailed {
		t.Fatalf("expected emitted failed status, got %s", receivedEvents[0].Status)
	}
}

func fakeListResponseWithNext(next *string) domain.MailListResponse {
	resp := domain.MailListResponse{}
	resp.Code = 0
	resp.Data.NextID = next
	resp.Data.Data = []domain.MarketItem{
		{
			C2CItemsID:   1001,
			C2CItemsName: "测试商品",
			Price:        1000,
			ShowPrice:    "10.00",
		},
	}
	return resp
}

func insertScrapyTask(t *testing.T, db *dao.Database, id int, product string) {
	t.Helper()
	now := time.Now()
	next := ""
	_, err := db.Db.Exec(
		`INSERT INTO scrapy_items(
			id, price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, account_id, request_interval_seconds, nums, increase_number, next_token, create_time, "order"
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "", "不限", "", "不限", product, product, 0, 0, 0, 0, next, now, "TIME_DESC",
	)
	if err != nil {
		t.Fatalf("insert scrapy task failed: %v", err)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func newScrapyTestDatabase(t *testing.T) *dao.Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "scrapy.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}
	db := &dao.Database{Db: rawDB}
	if err := db.Init(testScrapySchemaSQL); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const testScrapySchemaSQL = `
CREATE TABLE scrapy_items
(
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    price_filter          TEXT NOT NULL,
    price_filter_label    TEXT NOT NULL,
    discount_filter       TEXT NOT NULL,
    discount_filter_label TEXT NOT NULL,
    product               TEXT NOT NULL,
    product_name          TEXT NOT NULL,
    account_id            INTEGER NOT NULL DEFAULT 0,
    request_interval_seconds REAL NOT NULL DEFAULT 3,
    nums                  INTEGER,
    increase_number       INTEGER,
    next_token            TEXT,
    create_time           DATETIME,
    "order"               TEXT
);

CREATE TABLE c2c_items
(
    c2c_items_id      INTEGER PRIMARY KEY,
    type              INTEGER,
    c2c_items_name    TEXT    NOT NULL,
    detail_name       TEXT,
    detail_img        TEXT,
    sku_id            INTEGER,
    items_id          INTEGER,
    reference_price   INTEGER NOT NULL DEFAULT 0,
    total_items_count INTEGER,
    price             INTEGER,
    show_price        TEXT,
    show_market_price TEXT,
    seller_uid        TEXT,
    seller_name       TEXT,
    payment_time      INTEGER,
    publish_time      INTEGER,
    is_my_publish     BOOLEAN,
    uface             TEXT,
    raw_status        INTEGER,
    raw_sale_status   INTEGER,
    normalized_status TEXT    NOT NULL DEFAULT '在售',
    status_checked_at DATETIME,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE monitor_config
(
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    webhook TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO monitor_config (id, webhook)
VALUES (1, '');

CREATE TABLE monitor_rules
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sku_id     INTEGER NOT NULL,
    min_price  INTEGER NOT NULL,
    max_price  INTEGER NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    remark     TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE monitor_alert_history
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id      INTEGER NOT NULL,
    c2c_items_id INTEGER NOT NULL,
    task_id      INTEGER,
    sent         INTEGER NOT NULL DEFAULT 0,
    sent_at      DATETIME,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rule_id, c2c_items_id)
);

CREATE TABLE monitor_alert_events
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id       INTEGER NOT NULL,
    c2c_items_id  INTEGER NOT NULL,
    task_id       INTEGER,
    sku_id        INTEGER NOT NULL DEFAULT 0,
    item_name     TEXT    NOT NULL DEFAULT '',
    price         INTEGER NOT NULL DEFAULT 0,
    show_price    TEXT    NOT NULL DEFAULT '',
    item_link     TEXT    NOT NULL DEFAULT '',
    status        TEXT    NOT NULL,
    error_message TEXT    NOT NULL DEFAULT '',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE auth_accounts
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    uid          TEXT NOT NULL UNIQUE,
    account_name TEXT NOT NULL DEFAULT '',
    cookies      TEXT NOT NULL DEFAULT '',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func TestBuildDingTalkMarkdownHasTitleLinkAndPrice(t *testing.T) {
	content := buildDingTalkMarkdown("商品A", "99.00", "https://example.com/item")
	if !strings.Contains(content, "### 市集助手") {
		t.Fatalf("expected title in markdown, got %s", content)
	}
	if !strings.Contains(content, "商品：商品A") {
		t.Fatalf("expected item name in markdown, got %s", content)
	}
	if !strings.Contains(content, "[查看商品](https://example.com/item)") {
		t.Fatalf("expected markdown link, got %s", content)
	}
}

func TestMonitorRuleConversionContainsRemark(t *testing.T) {
	daoConfig := dao.MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []dao.MonitorRule{
			{
				ID:       7,
				SkuID:    10086,
				MinPrice: 100,
				MaxPrice: 200,
				Enabled:  true,
				Remark:   "for-test",
			},
		},
	}

	serviceConfig := toMonitorConfig(daoConfig)
	if len(serviceConfig.Rules) != 1 {
		t.Fatalf("expected 1 rule after conversion, got %d", len(serviceConfig.Rules))
	}
	if serviceConfig.Rules[0].Remark != "for-test" {
		t.Fatalf("expected service rule remark to be kept, got %q", serviceConfig.Rules[0].Remark)
	}

	roundtrip := toDAOMonitorConfig(serviceConfig)
	if len(roundtrip.Rules) != 1 {
		t.Fatalf("expected 1 rule after roundtrip, got %d", len(roundtrip.Rules))
	}
	if roundtrip.Rules[0].Remark != "for-test" {
		t.Fatalf("expected dao rule remark to be kept, got %q", roundtrip.Rules[0].Remark)
	}
}
