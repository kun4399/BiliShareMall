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

	_ "github.com/mattn/go-sqlite3"
	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	bilihttp "github.com/mikumifa/BiliShareMall/internal/http"
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
	mu    sync.Mutex
	calls int
}

func (m *mockNotifier) SendMarkdown(_ context.Context, _ string, _ string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return nil
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
			product, product_name, nums, increase_number, next_token, create_time, "order"
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "", "不限", "", "不限", product, product, 0, 0, next, now, "TIME_DESC",
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
