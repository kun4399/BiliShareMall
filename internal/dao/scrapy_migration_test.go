package dao

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestEnsureScrapyItemTaskRuntimeColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy_scrapy.db")
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
)`); err != nil {
		t.Fatalf("create legacy scrapy_items error: %v", err)
	}
	if _, err := rawDB.Exec(`
INSERT INTO scrapy_items(
    price_filter, price_filter_label, discount_filter, discount_filter_label,
    product, product_name, nums, increase_number, next_token, create_time, "order"
) VALUES('', '不限', '', '不限', 'p', 'name', 0, 0, '', CURRENT_TIMESTAMP, 'TIME_DESC')`); err != nil {
		t.Fatalf("insert legacy scrapy row error: %v", err)
	}

	db := &Database{Db: rawDB}
	if err := db.EnsureScrapyItemTaskRuntimeColumns(); err != nil {
		t.Fatalf("EnsureScrapyItemTaskRuntimeColumns error: %v", err)
	}

	hasAccountID, err := tableColumnExists(rawDB, "scrapy_items", "account_id")
	if err != nil {
		t.Fatalf("check account_id column error: %v", err)
	}
	if !hasAccountID {
		t.Fatal("expected account_id column")
	}
	hasInterval, err := tableColumnExists(rawDB, "scrapy_items", "request_interval_seconds")
	if err != nil {
		t.Fatalf("check request_interval_seconds column error: %v", err)
	}
	if !hasInterval {
		t.Fatal("expected request_interval_seconds column")
	}

	var seconds float64
	if err := rawDB.QueryRow(`SELECT request_interval_seconds FROM scrapy_items LIMIT 1`).Scan(&seconds); err != nil {
		t.Fatalf("query request_interval_seconds error: %v", err)
	}
	if seconds != 3 {
		t.Fatalf("expected default request interval 3, got %v", seconds)
	}
}
