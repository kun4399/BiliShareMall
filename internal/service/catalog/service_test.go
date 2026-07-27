package catalog

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/domain"
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

func TestStatusRefreshCoversAllStaleItemsAndSkipsFreshItems(t *testing.T) {
	database := newCatalogTestDatabase(t)
	for id := int64(1); id <= 3; id++ {
		createCatalogTestItem(t, database, id, 9101, dao.StatusOnSale)
	}
	if _, err := database.Db.Exec(
		`UPDATE c2c_items SET status_checked_at = ? WHERE c2c_items_id = 2`,
		time.Now(),
	); err != nil {
		t.Fatalf("mark item fresh error: %v", err)
	}

	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration))
	calls := make(chan int64, 3)
	svc.statusResolver = func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			return "", errors.New("background status refresh must not expire while queued")
		}
		calls <- item.C2CItemsID
		return dao.StatusSoldOut, nil
	}

	result, err := svc.ListC2CItemDetailBySku(9101, 1, 1, 1, dao.StatusOnSale, "")
	if err != nil {
		t.Fatalf("ListC2CItemDetailBySku error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].C2CItemsID != 3 {
		t.Fatalf("expected current page to contain item 3, got %+v", result.Items)
	}

	gotCalls := []int64{
		receiveItemID(t, calls),
		receiveItemID(t, calls),
	}
	if gotCalls[0] != 3 {
		t.Fatalf("expected visible stale item to be refreshed first, got calls %v", gotCalls)
	}
	if !containsItemID(gotCalls, 1) || !containsItemID(gotCalls, 3) {
		t.Fatalf("expected all stale items across pages, got calls %v", gotCalls)
	}
	select {
	case unexpected := <-calls:
		t.Fatalf("fresh item should have been skipped, got item %d", unexpected)
	case <-time.After(50 * time.Millisecond):
	}

	waitForCatalogCondition(t, func() bool {
		var refreshed int
		err := database.Db.QueryRow(
			`SELECT COUNT(*) FROM c2c_items
			WHERE c2c_items_id IN (1, 3)
				AND normalized_status = ?
				AND status_checked_at IS NOT NULL`,
			dao.StatusSoldOut,
		).Scan(&refreshed)
		return err == nil && refreshed == 2
	})

	var freshStatus string
	if err := database.Db.QueryRow(
		`SELECT normalized_status FROM c2c_items WHERE c2c_items_id = 2`,
	).Scan(&freshStatus); err != nil {
		t.Fatalf("read fresh item error: %v", err)
	}
	if freshStatus != dao.StatusOnSale {
		t.Fatalf("expected fresh item to remain unchanged, got %q", freshStatus)
	}
}

func TestStatusRefreshPersistsEachItemBeforeWholeJobCompletes(t *testing.T) {
	database := newCatalogTestDatabase(t)
	createCatalogTestItem(t, database, 11, 9201, dao.StatusOnSale)
	createCatalogTestItem(t, database, 12, 9201, dao.StatusOnSale)

	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration))
	secondStarted := make(chan struct{})
	releaseSecond := make(chan struct{})
	var callCount atomic.Int32
	svc.statusResolver = func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
		if callCount.Add(1) == 2 {
			close(secondStarted)
			<-releaseSecond
		}
		return dao.StatusSoldOut, nil
	}

	if _, err := svc.ListC2CItemDetailBySku(9201, 1, 1, 1, "", ""); err != nil {
		t.Fatalf("ListC2CItemDetailBySku error: %v", err)
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second status refresh did not start")
	}

	var firstStatus string
	var firstCheckedAt sql.NullTime
	if err := database.Db.QueryRow(
		`SELECT normalized_status, status_checked_at FROM c2c_items WHERE c2c_items_id = 12`,
	).Scan(&firstStatus, &firstCheckedAt); err != nil {
		t.Fatalf("read first refreshed item error: %v", err)
	}
	if firstStatus != dao.StatusSoldOut || !firstCheckedAt.Valid {
		t.Fatalf("expected first item persisted before job completion, got status=%q checked=%v", firstStatus, firstCheckedAt.Valid)
	}

	close(releaseSecond)
	waitForCatalogCondition(t, func() bool {
		var status string
		err := database.Db.QueryRow(
			`SELECT normalized_status FROM c2c_items WHERE c2c_items_id = 11`,
		).Scan(&status)
		return err == nil && status == dao.StatusSoldOut
	})
}

func TestRepeatedDetailLoadsShareOneStatusRefreshJob(t *testing.T) {
	database := newCatalogTestDatabase(t)
	createCatalogTestItem(t, database, 21, 9301, dao.StatusOnSale)

	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration))
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	svc.statusResolver = func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return dao.StatusOnSale, nil
	}

	if _, err := svc.ListC2CItemDetailBySku(9301, 1, 10, 1, "", ""); err != nil {
		t.Fatalf("first ListC2CItemDetailBySku error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("status refresh did not start")
	}
	if _, err := svc.ListC2CItemDetailBySku(9301, 1, 10, 1, "", ""); err != nil {
		t.Fatalf("second ListC2CItemDetailBySku error: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one shared status refresh job, got %d calls", got)
	}

	close(release)
	waitForCatalogCondition(t, func() bool {
		var checked int
		err := database.Db.QueryRow(
			`SELECT COUNT(*) FROM c2c_items WHERE c2c_items_id = 21 AND status_checked_at IS NOT NULL`,
		).Scan(&checked)
		return err == nil && checked == 1
	})
}

func TestFailedStatusRefreshPreservesStoredStateAndTimestamp(t *testing.T) {
	database := newCatalogTestDatabase(t)
	createCatalogTestItem(t, database, 31, 9401, dao.StatusOnSale)

	svc := NewService(database, cache.New(cache.NoExpiration, cache.NoExpiration))
	called := make(chan struct{})
	svc.statusResolver = func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
		close(called)
		return "", errors.New("upstream unavailable")
	}

	if _, err := svc.ListC2CItemDetailBySku(9401, 1, 10, 1, "", ""); err != nil {
		t.Fatalf("ListC2CItemDetailBySku error: %v", err)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("status refresh did not run")
	}
	waitForCatalogCondition(t, func() bool {
		svc.statusRefreshMu.Lock()
		defer svc.statusRefreshMu.Unlock()
		_, refreshing := svc.statusRefreshing[9401]
		return !refreshing
	})

	var status string
	var checkedAt sql.NullTime
	if err := database.Db.QueryRow(
		`SELECT normalized_status, status_checked_at FROM c2c_items WHERE c2c_items_id = 31`,
	).Scan(&status, &checkedAt); err != nil {
		t.Fatalf("read failed refresh state error: %v", err)
	}
	if status != dao.StatusOnSale || checkedAt.Valid {
		t.Fatalf("failed refresh changed stored state: status=%q checked=%v", status, checkedAt.Valid)
	}
}

func TestNormalizeDetailStatusRejectsMissingOrMismatchedSignals(t *testing.T) {
	if _, err := normalizeDetailStatus(41, domain.C2CItemDetailStatus{}); err == nil {
		t.Fatal("expected missing status signal to be rejected")
	}
	if _, err := normalizeDetailStatus(41, domain.C2CItemDetailStatus{
		DropReason: "未知原因",
	}); err == nil {
		t.Fatal("expected unrecognized drop reason to be rejected")
	}
	publishStatus := 1
	if _, err := normalizeDetailStatus(41, domain.C2CItemDetailStatus{
		C2CItemsID:    42,
		PublishStatus: &publishStatus,
	}); err == nil {
		t.Fatal("expected mismatched item id to be rejected")
	}
	status, err := normalizeDetailStatus(41, domain.C2CItemDetailStatus{
		C2CItemsID:    41,
		PublishStatus: &publishStatus,
	})
	if err != nil {
		t.Fatalf("normalize valid detail status error: %v", err)
	}
	if status != dao.StatusOnSale {
		t.Fatalf("expected on-sale status, got %q", status)
	}
	status, err = normalizeDetailStatus(41, domain.C2CItemDetailStatus{
		C2CItemsID: 41,
		DropReason: "已成交",
	})
	if err != nil {
		t.Fatalf("normalize sold-out drop reason error: %v", err)
	}
	if status != dao.StatusSoldOut {
		t.Fatalf("expected sold-out status, got %q", status)
	}
}

func createCatalogTestItem(t *testing.T, database *dao.Database, itemID, skuID int64, status string) {
	t.Helper()
	if _, err := database.CreateCSCItem(&dao.CSCItem{
		C2CItemsID:       itemID,
		C2CItemsName:     "测试商品",
		DetailName:       "测试商品",
		SkuID:            skuID,
		Price:            12900,
		ShowPrice:        "129",
		SellerUID:        "1",
		SellerName:       "卖家",
		NormalizedStatus: status,
	}); err != nil {
		t.Fatalf("insert c2c_items error: %v", err)
	}
}

func receiveItemID(t *testing.T, calls <-chan int64) int64 {
	t.Helper()
	select {
	case itemID := <-calls:
		return itemID
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status refresh")
		return 0
	}
}

func containsItemID(items []int64, target int64) bool {
	for _, itemID := range items {
		if itemID == target {
			return true
		}
	}
	return false
}

func waitForCatalogCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for catalog condition")
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
