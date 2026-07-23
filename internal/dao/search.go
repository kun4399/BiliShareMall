package dao

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type C2CItemGroup struct {
	SkuID             int64  `json:"skuId"`
	C2CItemsName      string `json:"c2cItemsName"`
	DetailImg         string `json:"detailImg"`
	ItemCount         int    `json:"itemCount"`
	MinPrice          int    `json:"minPrice"`
	ReferencePriceMin int    `json:"referencePriceMin"`
	ReferencePriceMax int    `json:"referencePriceMax"`
	FirstSeenTime     int64  `json:"firstSeenTime"`
	LatestPublishTime int64  `json:"latestPublishTime"`
}

type C2CItemGroupMeta struct {
	SkuID        int64  `json:"skuId"`
	C2CItemsName string `json:"c2cItemsName"`
	DetailImg    string `json:"detailImg"`
}

func (d *Database) ReadC2CItemGroups(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) ([]C2CItemGroup, int, error) {
	if err := d.EnsureCatalogReadModel(); err != nil {
		return nil, 0, err
	}

	startedAt := time.Now()
	offset := (page - 1) * pageSize

	listQuery := `
		SELECT
			grouped.sku_id,
			grouped.c2c_items_name,
			grouped.detail_img,
			grouped.item_count,
			grouped.min_price,
			grouped.reference_price_min,
			grouped.reference_price_max,
			grouped.first_seen_time,
			grouped.latest_publish_time
		FROM c2c_item_groups AS grouped
	`
	countQuery := `SELECT COUNT(*) FROM c2c_item_groups AS grouped`

	conditions, args := buildGroupConditions(filterName, startTime, endTime, fromPrice, toPrice)
	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		listQuery += whereClause
		countQuery += whereClause
	}

	listQuery += " " + buildGroupSort(sortOption) + " LIMIT ? OFFSET ?"

	countStartedAt := time.Now()
	var totalCount int
	if err := d.Db.QueryRowContext(context.Background(), countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	countDuration := time.Since(countStartedAt)

	queryArgs := append(append([]any{}, args...), pageSize, offset)
	listStartedAt := time.Now()
	rows, err := d.Db.QueryContext(context.Background(), listQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]C2CItemGroup, 0)
	for rows.Next() {
		var item C2CItemGroup
		if err := rows.Scan(
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	totalDuration := time.Since(startedAt)
	if totalDuration >= 100*time.Millisecond {
		log.Warn().
			Dur("duration", totalDuration).
			Dur("countDuration", countDuration).
			Dur("listDuration", time.Since(listStartedAt)).
			Int("page", page).
			Int("pageSize", pageSize).
			Int("total", totalCount).
			Msg("slow catalog group query")
	}

	return items, totalCount, nil
}

func (d *Database) EnsureCatalogIndexes() error {
	indexes := []struct {
		columns   []string
		statement string
	}{
		{
			columns: []string{"sku_id", "publish_time", "updated_at", "c2c_items_id"},
			statement: `CREATE INDEX IF NOT EXISTS idx_c2c_items_sku_publish
				ON c2c_items(sku_id, publish_time DESC, updated_at DESC, c2c_items_id DESC)`,
		},
		{
			columns: []string{"sku_id", "created_at", "c2c_items_id"},
			statement: `CREATE INDEX IF NOT EXISTS idx_c2c_items_sku_created
				ON c2c_items(sku_id, created_at DESC, c2c_items_id DESC)`,
		},
		{
			columns: []string{"sku_id", "normalized_status", "created_at", "c2c_items_id"},
			statement: `CREATE INDEX IF NOT EXISTS idx_c2c_items_sku_status_created
				ON c2c_items(sku_id, normalized_status, created_at DESC, c2c_items_id DESC)`,
		},
	}
	for _, index := range indexes {
		available := true
		for _, column := range index.columns {
			exists, err := tableColumnExists(d.Db, "c2c_items", column)
			if err != nil {
				return err
			}
			if !exists {
				available = false
				break
			}
		}
		if !available {
			continue
		}
		if _, err := d.Db.ExecContext(context.Background(), index.statement); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) EnsureC2CItemReferencePriceColumn() error {
	rows, err := d.Db.QueryContext(context.Background(), `PRAGMA table_info(c2c_items)`)
	if err != nil {
		return err
	}

	hasReferencePrice := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "reference_price" {
			hasReferencePrice = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if !hasReferencePrice {
		if _, err := d.Db.ExecContext(
			context.Background(),
			`ALTER TABLE c2c_items ADD COLUMN reference_price INTEGER NOT NULL DEFAULT 0`,
		); err != nil {
			return err
		}
	}

	_, err = d.Db.ExecContext(
		context.Background(),
		`UPDATE c2c_items
		SET reference_price = CAST(ROUND(CAST(show_market_price AS REAL) * 100) AS INTEGER)
		WHERE COALESCE(reference_price, 0) <= 0
		  AND TRIM(COALESCE(show_market_price, '')) != ''
		  AND CAST(show_market_price AS REAL) > 0`,
	)
	return err
}

func (d *Database) GetC2CItemGroupMeta(skuID int64) (C2CItemGroupMeta, error) {
	var meta C2CItemGroupMeta
	err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT
			sku_id,
			COALESCE(NULLIF(detail_name, ''), c2c_items_name) AS c2c_items_name,
			COALESCE(detail_img, '') AS detail_img
		FROM c2c_items
		WHERE sku_id = ?
		ORDER BY
			CASE WHEN COALESCE(detail_img, '') != '' THEN 0 ELSE 1 END,
			COALESCE(publish_time, 0) DESC,
			updated_at DESC,
			c2c_items_id DESC
		LIMIT 1`,
		skuID,
	).Scan(&meta.SkuID, &meta.C2CItemsName, &meta.DetailImg)
	return meta, err
}

func (d *Database) ReadC2CItemDetailsBySku(skuID int64, page, pageSize int, sortOption int, statusFilter string) ([]CSCItem, int, error) {
	offset := (page - 1) * pageSize

	conditions := []string{"sku_id = ?"}
	args := []any{skuID}
	if statusFilter != "" {
		conditions = append(conditions, "normalized_status = ?")
		args = append(args, statusFilter)
	}
	whereClause := " WHERE " + strings.Join(conditions, " AND ")

	countQuery := "SELECT COUNT(*) FROM c2c_items" + whereClause
	var totalCount int
	if err := d.Db.QueryRowContext(context.Background(), countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id, reference_price,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at,
			COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0) AS first_seen_time
		FROM c2c_items` + whereClause + " " + buildDetailSort(sortOption) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := d.Db.QueryContext(context.Background(), query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]CSCItem, 0)
	for rows.Next() {
		item, err := scanCSCItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, totalCount, nil
}

func (d *Database) ReadAllC2CItemDetailsBySku(skuID int64) ([]CSCItem, error) {
	rows, err := d.Db.QueryContext(
		context.Background(),
		`SELECT
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id, reference_price,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at,
			COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0) AS first_seen_time
		FROM c2c_items
		WHERE sku_id = ?
		ORDER BY COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0) DESC, c2c_items_id DESC`,
		skuID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CSCItem, 0)
	for rows.Next() {
		item, err := scanCSCItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type C2CItemStatusUpdate struct {
	C2CItemsID       int64
	NormalizedStatus string
	CheckedAt        time.Time
}

func (d *Database) UpdateC2CItemStatus(c2cItemsID int64, normalizedStatus string, checkedAt time.Time) error {
	return d.UpdateC2CItemStatuses([]C2CItemStatusUpdate{{
		C2CItemsID:       c2cItemsID,
		NormalizedStatus: normalizedStatus,
		CheckedAt:        checkedAt,
	}})
}

func (d *Database) UpdateC2CItemStatuses(updates []C2CItemStatusUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	if err := d.EnsureCatalogReadModel(); err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := d.Db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	affected := make(map[int64]struct{})
	for _, update := range updates {
		var skuID sql.NullInt64
		queryErr := tx.QueryRowContext(ctx, `SELECT sku_id FROM c2c_items WHERE c2c_items_id = ?`, update.C2CItemsID).Scan(&skuID)
		if queryErr != nil && queryErr != sql.ErrNoRows {
			return queryErr
		}
		if skuID.Valid && skuID.Int64 > 0 {
			affected[skuID.Int64] = struct{}{}
		}
		if _, err = tx.ExecContext(
			ctx,
			`UPDATE c2c_items
			SET normalized_status = ?, status_checked_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE c2c_items_id = ?`,
			update.NormalizedStatus,
			update.CheckedAt,
			update.C2CItemsID,
		); err != nil {
			return err
		}
	}
	if err = refreshCatalogGroupsTx(ctx, tx, affected); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) DeleteCSCItem(c2cItemsID int64) error {
	if err := d.EnsureCatalogReadModel(); err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := d.Db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	affected := make(map[int64]struct{})
	var skuID sql.NullInt64
	queryErr := tx.QueryRowContext(ctx, `SELECT sku_id FROM c2c_items WHERE c2c_items_id = ?`, c2cItemsID).Scan(&skuID)
	if queryErr != nil && queryErr != sql.ErrNoRows {
		return queryErr
	}
	if skuID.Valid && skuID.Int64 > 0 {
		affected[skuID.Int64] = struct{}{}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM c2c_items WHERE c2c_items_id = ?", c2cItemsID); err != nil {
		return err
	}
	if err = refreshCatalogGroupsTx(ctx, tx, affected); err != nil {
		return err
	}
	return tx.Commit()
}
