package dao

import (
	"database/sql"
	"strings"
)

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
