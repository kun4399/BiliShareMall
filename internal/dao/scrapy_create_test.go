package dao

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateScrapyItemPersistsRuntimeColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scrapy_create.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}
	t.Cleanup(func() {
		_ = rawDB.Close()
	})

	if _, err := rawDB.Exec(`
CREATE TABLE scrapy_items
(
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    price_filter             TEXT NOT NULL,
    price_filter_label       TEXT NOT NULL,
    discount_filter          TEXT NOT NULL,
    discount_filter_label    TEXT NOT NULL,
    product                  TEXT NOT NULL,
    product_name             TEXT NOT NULL,
    account_id               INTEGER NOT NULL DEFAULT 0,
    request_interval_seconds REAL NOT NULL DEFAULT 3,
    nums                     INTEGER,
    increase_number          INTEGER,
    next_token               TEXT,
    create_time              DATETIME,
    "order"                  TEXT
)`); err != nil {
		t.Fatalf("create scrapy_items table error: %v", err)
	}

	db := &Database{Db: rawDB}
	nextToken := "token-1"
	now := time.Now().UTC()
	id, err := db.CreateScrapyItem(ScrapyItem{
		PriceFilter:         "",
		PriceFilterLabel:    "不限",
		DiscountFilter:      "",
		DiscountFilterLabel: "不限",
		Product:             "31212",
		ProductName:         "测试类型",
		AccountID:           9,
		RequestIntervalSec:  0.4,
		Nums:                0,
		IncreaseNumber:      0,
		NextToken:           &nextToken,
		CreateTime:          now,
		Order:               "TIME_DESC",
	})
	if err != nil {
		t.Fatalf("CreateScrapyItem error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive insert id, got %d", id)
	}

	var accountID int64
	var interval float64
	var order string
	var token string
	if err := rawDB.QueryRow(`
SELECT account_id, request_interval_seconds, "order", next_token
FROM scrapy_items
WHERE id = ?`, id).Scan(&accountID, &interval, &order, &token); err != nil {
		t.Fatalf("query inserted scrapy_item error: %v", err)
	}
	if accountID != 9 {
		t.Fatalf("expected account_id 9, got %d", accountID)
	}
	if interval != 0.4 {
		t.Fatalf("expected request_interval_seconds 0.4, got %v", interval)
	}
	if order != "TIME_DESC" {
		t.Fatalf("expected order TIME_DESC, got %q", order)
	}
	if token != nextToken {
		t.Fatalf("expected next_token %q, got %q", nextToken, token)
	}
}
