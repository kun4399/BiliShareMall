package catalog

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	_ "github.com/mattn/go-sqlite3"
	cache "github.com/patrickmn/go-cache"
)

func TestFormatReferencePriceLabel(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
		want string
	}{
		{name: "missing", min: 0, max: 0, want: "参考价待补充"},
		{name: "single", min: 12900, max: 12900, want: "参考价 129.00 元"},
		{name: "range", min: 9900, max: 12900, want: "参考价 99.00 - 129.00 元"},
		{name: "max only", min: 0, max: 12900, want: "参考价 129.00 元"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatReferencePriceLabel(tt.min, tt.max); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGetC2CItemNameBySkuDoesNotCacheEmptyResults(t *testing.T) {
	database := newCatalogTestDatabase(t)
	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration))

	name, err := svc.GetC2CItemNameBySku(9001)
	if err != nil {
		t.Fatalf("GetC2CItemNameBySku missing error: %v", err)
	}
	if name != "" {
		t.Fatalf("expected empty name for missing sku, got %q", name)
	}

	if _, err := database.Db.Exec(
		`INSERT INTO c2c_items(c2c_items_id, c2c_items_name, detail_name, sku_id, reference_price, normalized_status)
		VALUES(1, '旧商品名', '新商品名', 9001, 12900, '在售')`,
	); err != nil {
		t.Fatalf("insert c2c_items error: %v", err)
	}

	name, err = svc.GetC2CItemNameBySku(9001)
	if err != nil {
		t.Fatalf("GetC2CItemNameBySku populated error: %v", err)
	}
	if name != "新商品名" {
		t.Fatalf("expected refreshed name %q, got %q", "新商品名", name)
	}
}

func TestListC2CItemDetailReturnsBeforeRemoteStatusRefresh(t *testing.T) {
	database := newCatalogTestDatabase(t)
	if _, err := database.CreateCSCItem(&dao.CSCItem{
		C2CItemsID:       1,
		C2CItemsName:     "测试商品",
		DetailName:       "测试商品",
		SkuID:            9001,
		Price:            12900,
		ShowPrice:        "129",
		SellerUID:        "1",
		SellerName:       "卖家",
		NormalizedStatus: "在售",
	}); err != nil {
		t.Fatalf("insert c2c_items error: %v", err)
	}

	events := make(chan StatusesChangedEvent, 1)
	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration), func(name string, payload any) {
		if name == "catalog_statuses_changed" {
			events <- payload.(StatusesChangedEvent)
		}
	})
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	svc.statusResolver = func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
			return "已售出", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	requestStarted := time.Now()
	result, err := svc.ListC2CItemDetailBySku(9001, 1, 10, 1, "", "")
	if err != nil {
		t.Fatalf("ListC2CItemDetailBySku error: %v", err)
	}
	if elapsed := time.Since(requestStarted); elapsed > 200*time.Millisecond {
		t.Fatalf("detail snapshot waited for remote refresh: %s", elapsed)
	}
	if len(result.Items) != 1 || result.Items[0].Status != "在售" {
		t.Fatalf("expected immediate persisted snapshot, got %+v", result.Items)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background status refresh did not start")
	}
	close(release)

	select {
	case event := <-events:
		if event.SkuID != 9001 {
			t.Fatalf("expected event for sku 9001, got %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("background status refresh did not publish update")
	}

	var status string
	if err := database.Db.QueryRow(`SELECT normalized_status FROM c2c_items WHERE c2c_items_id = 1`).Scan(&status); err != nil {
		t.Fatalf("read refreshed status error: %v", err)
	}
	if status != "已售出" {
		t.Fatalf("expected refreshed status, got %q", status)
	}
}

func newCatalogTestDatabase(t *testing.T) *dao.Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}

	database := &dao.Database{Db: rawDB}
	if err := database.Init(catalogTestSchemaSQL); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

const catalogTestSchemaSQL = `
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
);`
