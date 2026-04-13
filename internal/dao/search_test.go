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
					"status": 2,
					"saleStatus": 2
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

func TestSaveMailListToDBKeepsStatusWhenRawFieldsMissing(t *testing.T) {
	db := newTestDatabase(t)

	withRaw := mustMarketListResponse(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"data": [
				{
					"c2cItemsId": 301,
					"type": 1,
					"c2cItemsName": "测试商品",
					"detailDtoList": [
						{
							"blindBoxId": 3,
							"itemsId": 3001,
							"skuId": 9301,
							"name": "测试商品",
							"img": "//img-c.png",
							"marketPrice": 9900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 1,
					"price": 5000,
					"showPrice": "50",
					"showMarketPrice": "99",
					"uid": "88***8",
					"paymentTime": 1710020000000,
					"isMyPublish": false,
					"uface": "face-raw",
					"uname": "卖家Raw",
					"status": 2,
					"saleStatus": 2
				}
			]
		}
	}`)

	withoutRaw := mustMarketListResponse(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"data": [
				{
					"c2cItemsId": 301,
					"type": 1,
					"c2cItemsName": "测试商品",
					"detailDtoList": [
						{
							"blindBoxId": 3,
							"itemsId": 3001,
							"skuId": 9301,
							"name": "测试商品",
							"img": "//img-c-2.png",
							"marketPrice": 9900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 1,
					"price": 4900,
					"showPrice": "49",
					"showMarketPrice": "99",
					"uid": "88***8",
					"paymentTime": 1710023600000,
					"isMyPublish": false,
					"uface": "face-raw",
					"uname": "卖家Raw-更新"
				}
			]
		}
	}`)

	if rows := db.SaveMailListToDB(&withRaw); rows != 1 {
		t.Fatalf("expected 1 row affected on first save, got %d", rows)
	}
	if rows := db.SaveMailListToDB(&withoutRaw); rows != 1 {
		t.Fatalf("expected 1 row affected on second save, got %d", rows)
	}

	details, total, err := db.ReadC2CItemDetailsBySku(9301, 1, 10, 1, "")
	if err != nil {
		t.Fatalf("ReadC2CItemDetailsBySku error: %v", err)
	}
	if total != 1 || len(details) != 1 {
		t.Fatalf("expected exactly one detail row, total=%d len=%d", total, len(details))
	}

	if details[0].RawStatus == nil || *details[0].RawStatus != 2 {
		t.Fatalf("expected raw_status to be preserved as 2, got %+v", details[0].RawStatus)
	}
	if details[0].RawSaleStatus == nil || *details[0].RawSaleStatus != 2 {
		t.Fatalf("expected raw_sale_status to be preserved as 2, got %+v", details[0].RawSaleStatus)
	}
	if details[0].NormalizedStatus != StatusSoldOut {
		t.Fatalf("expected normalized status to remain sold-out, got %s", details[0].NormalizedStatus)
	}
}

func TestReadC2CItemDetailsBySkuUsesFirstSeenTimeAndSortsByIt(t *testing.T) {
	db := newTestDatabase(t)

	response := mustMarketListResponse(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"data": [
				{
					"c2cItemsId": 401,
					"type": 1,
					"c2cItemsName": "时间排序商品",
					"detailDtoList": [
						{
							"blindBoxId": 4,
							"itemsId": 4001,
							"skuId": 9401,
							"name": "时间排序商品",
							"img": "//img-time-a.png",
							"marketPrice": 15900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 2,
					"price": 8800,
					"showPrice": "88",
					"showMarketPrice": "159",
					"uid": "90***1",
					"paymentTime": 1710030000000,
					"isMyPublish": false,
					"uface": "face-time-a",
					"uname": "卖家时间A"
				},
				{
					"c2cItemsId": 402,
					"type": 1,
					"c2cItemsName": "时间排序商品",
					"detailDtoList": [
						{
							"blindBoxId": 4,
							"itemsId": 4001,
							"skuId": 9401,
							"name": "时间排序商品",
							"img": "//img-time-b.png",
							"marketPrice": 15900,
							"type": 0,
							"isHidden": false
						}
					],
					"totalItemsCount": 2,
					"price": 9100,
					"showPrice": "91",
					"showMarketPrice": "159",
					"uid": "90***2",
					"paymentTime": 1710033600000,
					"isMyPublish": false,
					"uface": "face-time-b",
					"uname": "卖家时间B"
				}
			]
		}
	}`)

	if rows := db.SaveMailListToDB(&response); rows != 2 {
		t.Fatalf("expected 2 rows affected, got %d", rows)
	}

	if _, err := db.Db.Exec(`UPDATE c2c_items SET publish_time = 0 WHERE sku_id = ?`, 9401); err != nil {
		t.Fatalf("failed to clear publish_time: %v", err)
	}

	if _, err := db.Db.Exec(`
		UPDATE c2c_items
		SET created_at = CASE c2c_items_id
			WHEN 401 THEN '2026-01-01 08:00:00'
			WHEN 402 THEN '2026-01-02 08:00:00'
		END
		WHERE c2c_items_id IN (401, 402)
	`); err != nil {
		t.Fatalf("failed to set created_at for test rows: %v", err)
	}

	desc, total, err := db.ReadC2CItemDetailsBySku(9401, 1, 10, 1, "")
	if err != nil {
		t.Fatalf("ReadC2CItemDetailsBySku desc error: %v", err)
	}
	if total != 2 || len(desc) != 2 {
		t.Fatalf("expected 2 detail rows for desc sort, total=%d len=%d", total, len(desc))
	}
	if desc[0].C2CItemsID != 402 || desc[1].C2CItemsID != 401 {
		t.Fatalf("expected desc sort by firstSeenTime to be [402, 401], got [%d, %d]", desc[0].C2CItemsID, desc[1].C2CItemsID)
	}
	if desc[0].FirstSeenTime == 0 || desc[1].FirstSeenTime == 0 {
		t.Fatalf("expected non-zero firstSeenTime from created_at, got [%d, %d]", desc[0].FirstSeenTime, desc[1].FirstSeenTime)
	}
	if desc[0].FirstSeenTime <= desc[1].FirstSeenTime {
		t.Fatalf("expected desc firstSeenTime order, got [%d, %d]", desc[0].FirstSeenTime, desc[1].FirstSeenTime)
	}

	asc, totalAsc, err := db.ReadC2CItemDetailsBySku(9401, 1, 10, 2, "")
	if err != nil {
		t.Fatalf("ReadC2CItemDetailsBySku asc error: %v", err)
	}
	if totalAsc != 2 || len(asc) != 2 {
		t.Fatalf("expected 2 detail rows for asc sort, total=%d len=%d", totalAsc, len(asc))
	}
	if asc[0].C2CItemsID != 401 || asc[1].C2CItemsID != 402 {
		t.Fatalf("expected asc sort by firstSeenTime to be [401, 402], got [%d, %d]", asc[0].C2CItemsID, asc[1].C2CItemsID)
	}
	if asc[0].FirstSeenTime >= asc[1].FirstSeenTime {
		t.Fatalf("expected asc firstSeenTime order, got [%d, %d]", asc[0].FirstSeenTime, asc[1].FirstSeenTime)
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
