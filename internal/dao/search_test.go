package dao

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mikumifa/BiliShareMall/internal/domain"
)

func TestSaveMailListToDBUpsertsAndGroupsBySku(t *testing.T) {
	db := newTestDatabase(t)

	first := mustMarketListResponse(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"data": [
				{
					"c2cItemsId": 101,
					"type": 1,
					"c2cItemsName": "罗马仕移动电源",
					"detailDtoList": [
						{
							"blindBoxId": 1,
							"itemsId": 1001,
							"skuId": 9001,
							"name": "罗马仕移动电源 20000mAh",
							"img": "//img-a.png",
							"marketPrice": 12900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 2,
					"price": 6000,
					"showPrice": "60",
					"showMarketPrice": "129",
					"uid": "41***9",
					"paymentTime": 1710000000000,
					"isMyPublish": false,
					"uface": "face-a",
					"uname": "卖家A"
				},
				{
					"c2cItemsId": 102,
					"type": 1,
					"c2cItemsName": "罗马仕移动电源",
					"detailDtoList": [
						{
							"blindBoxId": 1,
							"itemsId": 1001,
							"skuId": 9001,
							"name": "罗马仕移动电源 20000mAh",
							"img": "//img-a.png",
							"marketPrice": 12900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 2,
					"price": 7500,
					"showPrice": "75",
					"showMarketPrice": "129",
					"uid": "52***1",
					"paymentTime": 1710003600000,
					"isMyPublish": false,
					"uface": "face-b",
					"uname": "卖家B"
				},
				{
					"c2cItemsId": 201,
					"type": 1,
					"c2cItemsName": "漫展徽章",
					"detailDtoList": [
						{
							"blindBoxId": 2,
							"itemsId": 2001,
							"skuId": 9002,
							"name": "漫展徽章礼盒",
							"img": "//img-b.png",
							"marketPrice": 4900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 1,
					"price": 3000,
					"showPrice": "30",
					"showMarketPrice": "49",
					"uid": "11***8",
					"paymentTime": 1710007200000,
					"isMyPublish": false,
					"uface": "face-c",
					"uname": "卖家C"
				}
			]
		}
	}`)

	second := mustMarketListResponse(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"data": [
				{
					"c2cItemsId": 101,
					"type": 1,
					"c2cItemsName": "罗马仕移动电源",
					"detailDtoList": [
						{
							"blindBoxId": 1,
							"itemsId": 1001,
							"skuId": 9001,
							"name": "罗马仕移动电源 20000mAh",
							"img": "//img-a-2.png",
							"marketPrice": 12900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 2,
					"price": 5800,
					"showPrice": "58",
					"showMarketPrice": "129",
					"uid": "41***9",
					"paymentTime": 1710010800000,
					"isMyPublish": false,
					"uface": "face-a",
					"uname": "卖家A-更新",
					"saleStatus": 1
				}
			]
		}
	}`)

	if rows := db.SaveMailListToDB(&first); rows != 3 {
		t.Fatalf("expected 3 rows affected on first save, got %d", rows)
	}
	if rows := db.SaveMailListToDB(&second); rows != 1 {
		t.Fatalf("expected 1 row affected on second save, got %d", rows)
	}

	groups, total, err := db.ReadC2CItemGroups(1, 10, "罗马仕", 1, -1, -1, -1, -1)
	if err != nil {
		t.Fatalf("ReadC2CItemGroups error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 grouped result, got %d", total)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 grouped item, got %d", len(groups))
	}
	if groups[0].SkuID != 9001 {
		t.Fatalf("expected sku 9001, got %d", groups[0].SkuID)
	}
	if groups[0].MinPrice != 5800 {
		t.Fatalf("expected grouped min price 5800, got %d", groups[0].MinPrice)
	}
	if groups[0].LatestPublishTime != 1710010800000 {
		t.Fatalf("expected latest publish time to use updated row, got %d", groups[0].LatestPublishTime)
	}
	if groups[0].DetailImg != "//img-a-2.png" {
		t.Fatalf("expected representative image to update, got %s", groups[0].DetailImg)
	}

	details, detailTotal, err := db.ReadC2CItemDetailsBySku(9001, 1, 10, 3, "")
	if err != nil {
		t.Fatalf("ReadC2CItemDetailsBySku error: %v", err)
	}
	if detailTotal != 2 {
		t.Fatalf("expected 2 detail rows, got %d", detailTotal)
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 detail items, got %d", len(details))
	}
	if details[0].Price != 5800 {
		t.Fatalf("expected price ascending order, got first price %d", details[0].Price)
	}
	if details[0].SellerName != "卖家A-更新" {
		t.Fatalf("expected upserted seller name, got %s", details[0].SellerName)
	}
	if details[0].NormalizedStatus != StatusSoldOut {
		t.Fatalf("expected normalized sold-out status, got %s", details[0].NormalizedStatus)
	}

	filtered, filteredTotal, err := db.ReadC2CItemDetailsBySku(9001, 1, 10, 1, StatusSoldOut)
	if err != nil {
		t.Fatalf("ReadC2CItemDetailsBySku filtered error: %v", err)
	}
	if filteredTotal != 1 || len(filtered) != 1 {
		t.Fatalf("expected one sold-out item, got total=%d len=%d", filteredTotal, len(filtered))
	}
}

func newTestDatabase(t *testing.T) *Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "bsm.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}

	db := &Database{Db: rawDB}
	if err := db.Init(testSchemaSQL); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func mustMarketListResponse(t *testing.T, raw string) domain.MailListResponse {
	t.Helper()

	var response domain.MailListResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("unmarshal response error: %v", err)
	}
	return response
}

const testSchemaSQL = `
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
);`
