package dao

import (
	"context"
	"database/sql"
	"strings"
	"time"
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
	offset := (page - 1) * pageSize

	baseQuery := `
		WITH grouped AS (
			SELECT
				sku_id,
				COALESCE(MAX(NULLIF(detail_name, '')), MAX(c2c_items_name)) AS c2c_items_name,
				MIN(price) AS min_price,
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
			COALESCE(rep.detail_img, '') AS detail_img,
			grouped.item_count,
			grouped.min_price,
			COALESCE(grouped.reference_price_min, 0) AS reference_price_min,
			COALESCE(grouped.reference_price_max, 0) AS reference_price_max,
			grouped.first_seen_time,
			grouped.latest_publish_time
		FROM grouped
		LEFT JOIN c2c_items rep ON rep.c2c_items_id = (
			SELECT c2c_items_id
			FROM c2c_items rep2
			WHERE rep2.sku_id = grouped.sku_id
			ORDER BY
				CASE WHEN COALESCE(rep2.detail_img, '') != '' THEN 0 ELSE 1 END,
				COALESCE(rep2.publish_time, 0) DESC,
				rep2.updated_at DESC,
				rep2.c2c_items_id DESC
			LIMIT 1
		)
	`
	countQuery := `SELECT COUNT(*) FROM grouped`

	conditions, args := buildGroupConditions(filterName, startTime, endTime, fromPrice, toPrice)
	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		listQuery += whereClause
		countQuery += whereClause
	}

	listQuery += " " + buildGroupSort(sortOption) + " LIMIT ? OFFSET ?"

	var totalCount int
	if err := d.Db.QueryRowContext(context.Background(), baseQuery+countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := d.Db.QueryContext(context.Background(), baseQuery+listQuery, queryArgs...)
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

	return items, totalCount, nil
}

func (d *Database) EnsureC2CItemReferencePriceColumn() error {
	rows, err := d.Db.QueryContext(context.Background(), `PRAGMA table_info(c2c_items)`)
	if err != nil {
		return err
	}
	defer rows.Close()

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

func (d *Database) UpdateC2CItemStatus(c2cItemsID int64, normalizedStatus string, checkedAt time.Time) error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`UPDATE c2c_items
		SET normalized_status = ?, status_checked_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE c2c_items_id = ?`,
		normalizedStatus,
		checkedAt,
		c2cItemsID,
	)
	return err
}

func (d *Database) DeleteCSCItem(c2cItemsID int64) error {
	_, err := d.Db.ExecContext(context.Background(), "DELETE FROM c2c_items WHERE c2c_items_id = ?", c2cItemsID)
	return err
}
