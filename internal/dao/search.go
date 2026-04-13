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
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at
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
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at
		FROM c2c_items
		WHERE sku_id = ?
		ORDER BY COALESCE(publish_time, 0) DESC, c2c_items_id DESC`,
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

func buildGroupConditions(filterName string, startTime, endTime int64, fromPrice, toPrice int) ([]string, []any) {
	var conditions []string
	var args []any

	if filterName != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(filterName)
		conditions = append(conditions, "grouped.c2c_items_name LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escaped+"%")
	}
	if startTime > 0 {
		conditions = append(conditions, "grouped.latest_publish_time >= ?")
		args = append(args, startTime)
	}
	if endTime > 0 {
		conditions = append(conditions, "grouped.latest_publish_time <= ?")
		args = append(args, endTime)
	}
	if fromPrice != -1 {
		conditions = append(conditions, "grouped.min_price >= ?")
		args = append(args, fromPrice*100)
	}
	if toPrice != -1 {
		conditions = append(conditions, "grouped.min_price <= ?")
		args = append(args, toPrice*100)
	}

	return conditions, args
}

func buildGroupSort(sortOption int) string {
	switch sortOption {
	case 2:
		return "ORDER BY grouped.min_price ASC, grouped.latest_publish_time DESC"
	case 3:
		return "ORDER BY grouped.min_price DESC, grouped.latest_publish_time DESC"
	default:
		return "ORDER BY grouped.latest_publish_time DESC, grouped.min_price ASC"
	}
}

func buildDetailSort(sortOption int) string {
	switch sortOption {
	case 2:
		return "ORDER BY COALESCE(publish_time, 0) ASC, c2c_items_id ASC"
	case 3:
		return "ORDER BY price ASC, COALESCE(publish_time, 0) DESC"
	case 4:
		return "ORDER BY price DESC, COALESCE(publish_time, 0) DESC"
	default:
		return "ORDER BY COALESCE(publish_time, 0) DESC, c2c_items_id DESC"
	}
}

func scanCSCItem(scanner interface {
	Scan(dest ...any) error
}) (CSCItem, error) {
	var item CSCItem
	var rawStatus sql.NullInt64
	var rawSaleStatus sql.NullInt64
	var statusCheckedAt sql.NullTime

	err := scanner.Scan(
		&item.C2CItemsID,
		&item.Type,
		&item.C2CItemsName,
		&item.DetailName,
		&item.DetailImg,
		&item.SkuID,
		&item.ItemsID,
		&item.TotalItemsCount,
		&item.Price,
		&item.ShowPrice,
		&item.ShowMarketPrice,
		&item.SellerUID,
		&item.SellerName,
		&item.PaymentTime,
		&item.PublishTime,
		&item.IsMyPublish,
		&item.Uface,
		&rawStatus,
		&rawSaleStatus,
		&item.NormalizedStatus,
		&statusCheckedAt,
	)
	if err != nil {
		return CSCItem{}, err
	}

	if rawStatus.Valid {
		value := int(rawStatus.Int64)
		item.RawStatus = &value
	}
	if rawSaleStatus.Valid {
		value := int(rawSaleStatus.Int64)
		item.RawSaleStatus = &value
	}

	return item, nil
}
