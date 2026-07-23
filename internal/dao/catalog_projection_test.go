package dao

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnsureCatalogReadModelBackfillsAndMaintainsAffectedSkus(t *testing.T) {
	db := newTestDatabase(t)

	if _, err := db.Db.Exec(`
		INSERT INTO c2c_items(
			c2c_items_id, c2c_items_name, detail_name, detail_img, sku_id,
			reference_price, price, publish_time, normalized_status, created_at, updated_at
		) VALUES
			(1, '商品 A', '商品 A 详情', '//a-1.png', 100, 9900, 5000, 1000, '在售', '2026-01-01', '2026-01-01'),
			(2, '商品 A', '商品 A 详情', '//a-2.png', 100, 12900, 6000, 2000, '在售', '2026-01-02', '2026-01-02'),
			(3, '商品 B', '商品 B 详情', '//b.png', 200, 15900, 7000, 3000, '在售', '2026-01-03', '2026-01-03')
	`); err != nil {
		t.Fatalf("insert legacy catalog rows: %v", err)
	}

	if err := db.EnsureCatalogReadModel(); err != nil {
		t.Fatalf("EnsureCatalogReadModel error: %v", err)
	}

	groups, total, err := db.ReadC2CItemGroups(1, 10, "", 1, -1, -1, -1, -1)
	if err != nil {
		t.Fatalf("ReadC2CItemGroups error: %v", err)
	}
	if total != 2 || len(groups) != 2 {
		t.Fatalf("expected two backfilled groups, total=%d len=%d", total, len(groups))
	}

	var groupCount, minReference, maxReference int
	if err = db.Db.QueryRow(
		`SELECT item_count, reference_price_min, reference_price_max FROM c2c_item_groups WHERE sku_id = 100`,
	).Scan(&groupCount, &minReference, &maxReference); err != nil {
		t.Fatalf("query backfilled group: %v", err)
	}
	if groupCount != 2 || minReference != 9900 || maxReference != 12900 {
		t.Fatalf("unexpected projection values count=%d range=[%d,%d]", groupCount, minReference, maxReference)
	}

	_, err = db.CreateCSCItem(&CSCItem{
		C2CItemsID:       2,
		C2CItemsName:     "商品 B",
		DetailName:       "商品 B 详情",
		DetailImg:        "//b-2.png",
		SkuID:            200,
		ReferencePrice:   18900,
		Price:            6500,
		PublishTime:      4000,
		NormalizedStatus: StatusOnSale,
	})
	if err != nil {
		t.Fatalf("move item between skus: %v", err)
	}

	if err = db.Db.QueryRow(`SELECT item_count FROM c2c_item_groups WHERE sku_id = 100`).Scan(&groupCount); err != nil {
		t.Fatalf("query old sku after move: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("expected old sku count 1, got %d", groupCount)
	}
	if err = db.Db.QueryRow(`SELECT item_count FROM c2c_item_groups WHERE sku_id = 200`).Scan(&groupCount); err != nil {
		t.Fatalf("query new sku after move: %v", err)
	}
	if groupCount != 2 {
		t.Fatalf("expected new sku count 2, got %d", groupCount)
	}

	if err = db.DeleteCSCItem(1); err != nil {
		t.Fatalf("delete final item from sku: %v", err)
	}
	var remaining int
	if err = db.Db.QueryRow(`SELECT COUNT(*) FROM c2c_item_groups WHERE sku_id = 100`).Scan(&remaining); err != nil {
		t.Fatalf("query deleted sku projection: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected empty sku projection to be deleted, got %d rows", remaining)
	}
}

func TestCatalogProjectionValidationMatchesDistinctSkuCount(t *testing.T) {
	db := newTestDatabase(t)
	if _, err := db.Db.Exec(`
		INSERT INTO c2c_items(c2c_items_id, c2c_items_name, sku_id, normalized_status)
		VALUES (1, '有效商品', 300, '在售'), (2, '无 SKU 商品', 0, '在售')
	`); err != nil {
		t.Fatalf("insert source rows: %v", err)
	}
	if err := db.EnsureCatalogReadModel(); err != nil {
		t.Fatalf("EnsureCatalogReadModel error: %v", err)
	}

	var sourceCount, projectionCount int
	if err := db.Db.QueryRow(`SELECT COUNT(DISTINCT sku_id) FROM c2c_items WHERE sku_id != 0`).Scan(&sourceCount); err != nil {
		t.Fatalf("count source groups: %v", err)
	}
	if err := db.Db.QueryRow(`SELECT COUNT(*) FROM c2c_item_groups`).Scan(&projectionCount); err != nil {
		t.Fatalf("count projected groups: %v", err)
	}
	if sourceCount != projectionCount {
		t.Fatalf("source/projection mismatch: %d != %d", sourceCount, projectionCount)
	}
}

func TestCatalogProjectionMigrationRollsBackWhenVersionUpdateFails(t *testing.T) {
	db := newTestDatabase(t)
	if _, err := db.Db.Exec(`
		CREATE TABLE version (
			id INTEGER PRIMARY KEY,
			version INTEGER NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO version(id, version) VALUES (1, 7);
		INSERT INTO c2c_items(
			c2c_items_id, c2c_items_name, detail_name, detail_img, sku_id,
			reference_price, price, publish_time, normalized_status, created_at, updated_at
		) VALUES (
			1, '回滚商品', '回滚商品', '//rollback.png', 900,
			9900, 5000, 1000, '在售', '2026-01-01', '2026-01-01'
		);
		CREATE TRIGGER reject_version_upgrade
		BEFORE UPDATE ON version
		BEGIN
			SELECT RAISE(ABORT, 'injected version failure');
		END;
	`); err != nil {
		t.Fatalf("prepare migration failure: %v", err)
	}

	if err := db.EnsureCatalogReadModelAndVersion(8); err == nil {
		t.Fatal("expected injected migration error")
	}

	var version int
	if err := db.Db.QueryRow(`SELECT version FROM version WHERE id = 1`).Scan(&version); err != nil {
		t.Fatalf("read version after rollback: %v", err)
	}
	if version != 7 {
		t.Fatalf("expected version 7 after rollback, got %d", version)
	}

	var tableCount int
	if err := db.Db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'c2c_item_groups'
	`).Scan(&tableCount); err != nil {
		t.Fatalf("inspect projection table after rollback: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("expected projection table creation and backfill to roll back")
	}

	var detailCount int
	if err := db.Db.QueryRow(`SELECT COUNT(*) FROM c2c_items`).Scan(&detailCount); err != nil {
		t.Fatalf("read source rows after rollback: %v", err)
	}
	if detailCount != 1 {
		t.Fatalf("expected source rows to remain untouched, got %d", detailCount)
	}
}

func TestCatalogProjectionMatchesLegacyAggregationForFiltersSortsAndPages(t *testing.T) {
	db := newTestDatabase(t)

	for id := 1; id <= 240; id++ {
		skuID := int64((id-1)%31 + 1)
		referencePrice := 0
		if id%5 != 0 {
			referencePrice = 1000 + (id%90)*137
		}
		detailImg := ""
		if id%4 == 0 {
			detailImg = fmt.Sprintf("//images/%d.png", id)
		}
		createdAt := fmt.Sprintf("2026-01-%02d 12:00:00", id%28+1)
		if _, err := db.Db.Exec(`
			INSERT INTO c2c_items(
				c2c_items_id, c2c_items_name, detail_name, detail_img, sku_id,
				reference_price, price, publish_time, normalized_status, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '在售', ?, ?)
		`,
			id,
			fmt.Sprintf("商品 %02d", skuID),
			fmt.Sprintf("商品 %02d", skuID),
			detailImg,
			skuID,
			referencePrice,
			500+(id%120)*83,
			int64(1700000000000+id*1000),
			createdAt,
			createdAt,
		); err != nil {
			t.Fatalf("seed equivalence row %d: %v", id, err)
		}
	}
	if err := db.EnsureCatalogReadModel(); err != nil {
		t.Fatalf("build projection: %v", err)
	}

	filters := []struct {
		name               string
		keyword            string
		startTime, endTime int64
		fromPrice, toPrice int
	}{
		{name: "all", startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "keyword", keyword: "商品 1", startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "time", startTime: 1700000100000, endTime: 1700000200000, fromPrice: -1, toPrice: -1},
		{name: "price", startTime: -1, endTime: -1, fromPrice: 20, toPrice: 90},
		{name: "combined", keyword: "商品 2", startTime: 1700000100000, endTime: -1, fromPrice: 20, toPrice: 100},
	}

	for _, filter := range filters {
		for sortOption := 1; sortOption <= 3; sortOption++ {
			for page := 1; page <= 3; page++ {
				testName := fmt.Sprintf("%s/sort-%d/page-%d", filter.name, sortOption, page)
				t.Run(testName, func(t *testing.T) {
					got, gotTotal, err := db.ReadC2CItemGroups(
						page, 7, filter.keyword, sortOption,
						filter.startTime, filter.endTime, filter.fromPrice, filter.toPrice,
					)
					if err != nil {
						t.Fatalf("read projection: %v", err)
					}
					want, wantTotal, err := readLegacyC2CItemGroups(
						db.Db, page, 7, filter.keyword, sortOption,
						filter.startTime, filter.endTime, filter.fromPrice, filter.toPrice,
					)
					if err != nil {
						t.Fatalf("read legacy aggregation: %v", err)
					}
					if gotTotal != wantTotal {
						t.Fatalf("total mismatch: projection=%d legacy=%d", gotTotal, wantTotal)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("items mismatch:\nprojection=%+v\nlegacy=%+v", got, want)
					}
				})
			}
		}
	}
}

func readLegacyC2CItemGroups(
	db *sql.DB,
	page, pageSize int,
	filterName string,
	sortOption int,
	startTime, endTime int64,
	fromPrice, toPrice int,
) ([]C2CItemGroup, int, error) {
	baseQuery := `
		WITH grouped AS (
			SELECT
				sku_id,
				COALESCE(MAX(NULLIF(detail_name, '')), MAX(c2c_items_name)) AS c2c_items_name,
				COALESCE(MIN(price), 0) AS min_price,
				MIN(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_min,
				MAX(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_max,
				MIN(COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0)) AS first_seen_time,
				MAX(COALESCE(publish_time, 0)) AS latest_publish_time,
				COUNT(*) AS item_count
			FROM c2c_items
			WHERE sku_id IS NOT NULL AND sku_id != 0
			GROUP BY sku_id
		)
	`
	listQuery := `
		SELECT
			grouped.sku_id,
			grouped.c2c_items_name,
			COALESCE((
				SELECT rep.detail_img
				FROM c2c_items rep
				WHERE rep.sku_id = grouped.sku_id
				  AND TRIM(COALESCE(rep.detail_img, '')) != ''
				ORDER BY rep.publish_time DESC, rep.updated_at DESC, rep.c2c_items_id DESC
				LIMIT 1
			), ''),
			grouped.item_count,
			grouped.min_price,
			COALESCE(grouped.reference_price_min, 0),
			COALESCE(grouped.reference_price_max, 0),
			grouped.first_seen_time,
			grouped.latest_publish_time
		FROM grouped
	`
	countQuery := `SELECT COUNT(*) FROM grouped`
	conditions, args := buildGroupConditions(filterName, startTime, endTime, fromPrice, toPrice)
	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		listQuery += whereClause
		countQuery += whereClause
	}

	switch sortOption {
	case 2:
		listQuery += " ORDER BY CASE WHEN grouped.reference_price_min > 0 THEN 0 ELSE 1 END, grouped.reference_price_min ASC, grouped.first_seen_time DESC, grouped.sku_id ASC"
	case 3:
		listQuery += " ORDER BY CASE WHEN grouped.reference_price_max > 0 THEN 0 ELSE 1 END, grouped.reference_price_max DESC, grouped.first_seen_time DESC, grouped.sku_id ASC"
	default:
		listQuery += " ORDER BY grouped.first_seen_time DESC, CASE WHEN grouped.reference_price_min > 0 THEN 0 ELSE 1 END, grouped.reference_price_min ASC, grouped.sku_id ASC"
	}
	listQuery += " LIMIT ? OFFSET ?"

	var total int
	if err := db.QueryRow(baseQuery+countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.Query(baseQuery+listQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]C2CItemGroup, 0, pageSize)
	for rows.Next() {
		var item C2CItemGroup
		if err = rows.Scan(
			&item.SkuID,
			&item.C2CItemsName,
			&item.DetailImg,
			&item.ItemCount,
			&item.MinPrice,
			&item.ReferencePriceMin,
			&item.ReferencePriceMax,
			&item.FirstSeenTime,
			&item.LatestPublishTime,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func BenchmarkReadC2CItemGroupsMillion(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "catalog-million.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		b.Fatalf("open benchmark database: %v", err)
	}
	defer rawDB.Close()

	db := &Database{Db: rawDB}
	if err = db.Init(testSchemaSQL); err != nil {
		b.Fatalf("initialize benchmark schema: %v", err)
	}
	if _, err = rawDB.Exec(`
		WITH RECURSIVE seq(x) AS (
			VALUES(1)
			UNION ALL
			SELECT x + 1 FROM seq WHERE x < 1000000
		)
		INSERT INTO c2c_items(
			c2c_items_id, c2c_items_name, detail_name, detail_img, sku_id,
			reference_price, price, publish_time, normalized_status, created_at, updated_at
		)
		SELECT
			x,
			'商品' || (((x - 1) % 100000) + 1),
			'商品' || (((x - 1) % 100000) + 1),
			CASE WHEN x % 10 = 0 THEN '//benchmark/' || x || '.png' ELSE '' END,
			((x - 1) % 100000) + 1,
			1000 + (x % 90000),
			500 + (x % 100000),
			1700000000000 + x,
			'在售',
			datetime('now', '-' || (x % 365) || ' days'),
			CURRENT_TIMESTAMP
		FROM seq
	`); err != nil {
		b.Fatalf("seed benchmark rows: %v", err)
	}
	if _, err = rawDB.Exec(`
		CREATE INDEX idx_benchmark_items_sku
		ON c2c_items(sku_id, publish_time DESC, updated_at DESC, c2c_items_id DESC)
	`); err != nil {
		b.Fatalf("index benchmark rows: %v", err)
	}
	if err = db.EnsureCatalogReadModel(); err != nil {
		b.Fatalf("build catalog projection: %v", err)
	}

	cases := []struct {
		name               string
		page               int
		keyword            string
		sortOption         int
		startTime, endTime int64
		fromPrice, toPrice int
	}{
		{name: "first-page", page: 1, sortOption: 1, startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "adjacent-page", page: 2, sortOption: 1, startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "deep-page", page: 3000, sortOption: 1, startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "keyword", page: 1, keyword: "商品999", sortOption: 1, startTime: -1, endTime: -1, fromPrice: -1, toPrice: -1},
		{name: "time-and-price", page: 1, sortOption: 2, startTime: 1700000900000, endTime: 1700001000000, fromPrice: 10, toPrice: 1000},
	}

	for _, benchmarkCase := range cases {
		benchmarkCase := benchmarkCase
		b.Run(benchmarkCase.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, _, err = db.ReadC2CItemGroups(
					benchmarkCase.page,
					24,
					benchmarkCase.keyword,
					benchmarkCase.sortOption,
					benchmarkCase.startTime,
					benchmarkCase.endTime,
					benchmarkCase.fromPrice,
					benchmarkCase.toPrice,
				); err != nil {
					b.Fatalf("read benchmark page: %v", err)
				}
			}
		})
	}
}
