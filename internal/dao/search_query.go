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
	if fromPrice != -1 || toPrice != -1 {
		conditions = append(conditions, "grouped.reference_price_min > 0", "grouped.reference_price_max > 0")
	}
	if fromPrice != -1 {
		conditions = append(conditions, "grouped.reference_price_min >= ?")
		args = append(args, fromPrice*100)
	}
	if toPrice != -1 {
		conditions = append(conditions, "grouped.reference_price_max <= ?")
		args = append(args, toPrice*100)
	}

	return conditions, args
}

func buildGroupSort(sortOption int) string {
	switch sortOption {
	case 2:
		return "ORDER BY grouped.has_reference_price DESC, grouped.reference_price_min ASC, grouped.first_seen_time DESC, grouped.sku_id ASC"
	case 3:
		return "ORDER BY grouped.has_reference_price DESC, grouped.reference_price_max DESC, grouped.first_seen_time DESC, grouped.sku_id ASC"
	default:
		return "ORDER BY grouped.first_seen_time DESC, grouped.has_reference_price DESC, grouped.reference_price_min ASC, grouped.sku_id ASC"
	}
}

func buildDetailSort(sortOption int) string {
	switch sortOption {
	case 2:
		return "ORDER BY created_at ASC, c2c_items_id ASC"
	case 3:
		return "ORDER BY price ASC, created_at DESC, c2c_items_id DESC"
	case 4:
		return "ORDER BY price DESC, created_at DESC, c2c_items_id DESC"
	default:
		return "ORDER BY created_at DESC, c2c_items_id DESC"
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
		&item.ReferencePrice,
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
		&item.FirstSeenTime,
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
	if statusCheckedAt.Valid {
		item.StatusCheckedAt = statusCheckedAt.Time
	}

	return item, nil
}
