package dao

import (
	"context"
	"strings"
	"time"
)

func (d *Database) ReadCSCItems(page, pageSize int, filterName string, sortOption int, startTime, endTime *time.Time, fromPrice, toPrice int) ([]CSCItem, int, error) {
	offset := (page - 1) * pageSize

	// 设置查询语句
	queryStart := `SELECT
					c2c_items_id, type, c2c_items.c2c_items_name, total_items_count ,price, show_price, show_market_price, uid,
					payment_time, is_my_publish, uface, uname `
	countStart := `SELECT
					COUNT(*) `
	query := ` FROM c2c_items `

	// 动态构建WHERE条件
	var conditions []string
	var args []interface{}

	if filterName != "" {
		// Avoid FTS extension-specific query functions here to keep search stable across packaged runtimes.
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(filterName)
		conditions = append(conditions, "c2c_items.c2c_items_name LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escaped+"%")
	}

	//time
	if startTime != nil {
		conditions = append(conditions, "updated_at >= ?")
		args = append(args, *startTime)
	}

	if endTime != nil {
		conditions = append(conditions, "updated_at <= ?")
		args = append(args, *endTime)
	}

	if fromPrice != -1 {
		conditions = append(conditions, "price >= ?")
		args = append(args, fromPrice*100)
	}

	if toPrice != -1 {
		conditions = append(conditions, "price <= ?")
		args = append(args, toPrice*100)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	switch sortOption {
	case 1:
		query += " ORDER BY updated_at DESC"
	case 2:
		query += " ORDER BY price ASC"
	case 3:
		query += " ORDER BY price DESC"
	default:
		// 默认按 c2c_items_id 升序
		query += " ORDER BY updated_at DESC"
	}

	// 查询总记录数
	var totalCount int
	err := d.Db.QueryRowContext(context.Background(), countStart+query, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// 添加分页控制
	args = append(args, pageSize, offset)
	query += " LIMIT ? OFFSET ?"
	// 执行查询
	rows, err := d.Db.QueryContext(context.Background(), queryStart+query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// 解析结果
	items := make([]CSCItem, 0)
	for rows.Next() {
		var item CSCItem
		if err := rows.Scan(
			&item.C2CItemsID,
			&item.Type,
			&item.C2CItemsName,
			&item.TotalItemsCount,
			&item.Price,
			&item.ShowPrice,
			&item.ShowMarketPrice,
			&item.UID,
			&item.PaymentTime,
			&item.IsMyPublish,
			&item.Uface,
			&item.Uname,
		); err != nil {
			//have deleted
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, totalCount, nil
}

// DeleteCSCItem 删除商品记录
func (d *Database) DeleteCSCItem(c2cItemsID int64) error {
	_, err := d.Db.ExecContext(context.Background(),
		"DELETE FROM c2c_items WHERE c2c_items_id = ?", c2cItemsID)
	return err
}

// GetPagedItems 分页查询商品
func (d *Database) GetPagedItems(limit, offset int) ([]CSCItem, error) {
	rows, err := d.Db.QueryContext(context.Background(),
		"SELECT c2c_items_id, type, c2c_items_name, total_items_count, price, show_price, show_market_price, uid, payment_time, is_my_publish, uface, uname FROM c2c_items LIMIT ? OFFSET ?",
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CSCItem
	for rows.Next() {
		var item CSCItem
		if err := rows.Scan(&item.C2CItemsID, &item.Type, &item.C2CItemsName, &item.TotalItemsCount, &item.Price, &item.ShowPrice, &item.ShowMarketPrice, &item.UID, &item.PaymentTime, &item.IsMyPublish, &item.Uface, &item.Uname); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
