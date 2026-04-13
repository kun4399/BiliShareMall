package dao

import (
	"context"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	"github.com/rs/zerolog/log"
	"time"
)

type ScrapyItem struct {
	Id                  int64     `json:"id"`
	PriceFilter         string    `json:"priceFilter"`
	PriceFilterLabel    string    `json:"priceFilterLabel"`
	DiscountFilter      string    `json:"discountFilter"`
	DiscountFilterLabel string    `json:"discountFilterLabel"`
	Product             string    `json:"product"`
	ProductName         string    `json:"productName"`
	Nums                int       `json:"nums"`
	Order               string    `json:"order"`
	IncreaseNumber      int       `json:"increaseNumber"`
	NextToken           *string   `json:"nextToken"`
	CreateTime          time.Time `json:"createTime"`
}

func (d *Database) CreateScrapyItem(item ScrapyItem) (int64, error) {
	result, err := d.Db.ExecContext(
		context.Background(),
		`INSERT INTO scrapy_items (
			price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, nums, increase_number, next_token, create_time, "order"
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.PriceFilter,
		item.PriceFilterLabel,
		item.DiscountFilter,
		item.DiscountFilterLabel,
		item.Product,
		item.ProductName,
		item.Nums,
		item.IncreaseNumber,
		item.NextToken,
		item.CreateTime,
		item.Order,
	)
	if err != nil {
		return -1, err
	}
	return result.LastInsertId()
}

func (d *Database) UpdateScrapyItem(item *ScrapyItem) (int64, error) {
	result, err := d.Db.ExecContext(
		context.Background(),
		`UPDATE scrapy_items SET
			price_filter = ?, price_filter_label = ?, discount_filter = ?, discount_filter_label = ?,
			product = ?, product_name = ?, nums = ?, increase_number = ?, next_token = ?, create_time = ?, "order" = ?
		 WHERE id = ?`,
		item.PriceFilter,
		item.PriceFilterLabel,
		item.DiscountFilter,
		item.DiscountFilterLabel,
		item.Product,
		item.ProductName,
		item.Nums,
		item.IncreaseNumber,
		item.NextToken,
		item.CreateTime,
		item.Order,
		item.Id,
	)
	if err != nil {
		return -1, err
	}
	return result.RowsAffected()
}

func (d *Database) ReadScrapyItem(id int) (ScrapyItem, error) {
	var item ScrapyItem
	err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT
			price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, nums, increase_number, next_token, create_time, "order"
		FROM scrapy_items WHERE id = ?`,
		id,
	).Scan(
		&item.PriceFilter,
		&item.PriceFilterLabel,
		&item.DiscountFilter,
		&item.DiscountFilterLabel,
		&item.Product,
		&item.ProductName,
		&item.Nums,
		&item.IncreaseNumber,
		&item.NextToken,
		&item.CreateTime,
		&item.Order,
	)
	item.Id = int64(id)
	return item, err
}

func (d *Database) DeleteScrapyItem(id int) error {
	_, err := d.Db.ExecContext(context.Background(), "DELETE FROM scrapy_items WHERE id = ?", id)
	return err
}

func (d *Database) ReadAllScrapyItems() ([]ScrapyItem, error) {
	rows, err := d.Db.QueryContext(
		context.Background(),
		`SELECT
			id, price_filter, price_filter_label, discount_filter, discount_filter_label,
			product, product_name, nums, increase_number, next_token, create_time, "order"
		FROM scrapy_items`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ScrapyItem, 0)
	for rows.Next() {
		var item ScrapyItem
		if err := rows.Scan(
			&item.Id,
			&item.PriceFilter,
			&item.PriceFilterLabel,
			&item.DiscountFilter,
			&item.DiscountFilterLabel,
			&item.Product,
			&item.ProductName,
			&item.Nums,
			&item.IncreaseNumber,
			&item.NextToken,
			&item.CreateTime,
			&item.Order,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type CSCItem struct {
	C2CItemsID       int64  `json:"c2cItemsId"`
	Type             int    `json:"type"`
	C2CItemsName     string `json:"c2cItemsName"`
	DetailName       string `json:"detailName"`
	DetailImg        string `json:"detailImg"`
	SkuID            int64  `json:"skuId"`
	ItemsID          int64  `json:"itemsId"`
	TotalItemsCount  int    `json:"totalItemsCount"`
	Price            int    `json:"price"`
	ShowPrice        string `json:"showPrice"`
	ShowMarketPrice  string `json:"showMarketPrice"`
	SellerUID        string `json:"sellerUid"`
	SellerName       string `json:"sellerName"`
	PaymentTime      int64  `json:"paymentTime"`
	PublishTime      int64  `json:"publishTime"`
	IsMyPublish      bool   `json:"isMyPublish"`
	Uface            string `json:"uface"`
	RawStatus        *int   `json:"rawStatus,omitempty"`
	RawSaleStatus    *int   `json:"rawSaleStatus,omitempty"`
	NormalizedStatus string `json:"normalizedStatus"`
}

func (d *Database) CreateCSCItem(item *CSCItem) (int64, error) {
	result, err := d.Db.ExecContext(context.Background(),
		`INSERT INTO c2c_items (
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(c2c_items_id) DO UPDATE SET
			type = excluded.type,
			c2c_items_name = excluded.c2c_items_name,
			detail_name = excluded.detail_name,
			detail_img = excluded.detail_img,
			sku_id = excluded.sku_id,
			items_id = excluded.items_id,
			total_items_count = excluded.total_items_count,
			price = excluded.price,
			show_price = excluded.show_price,
			show_market_price = excluded.show_market_price,
			seller_uid = excluded.seller_uid,
			seller_name = excluded.seller_name,
			payment_time = excluded.payment_time,
			publish_time = excluded.publish_time,
			is_my_publish = excluded.is_my_publish,
			uface = excluded.uface,
			raw_status = COALESCE(excluded.raw_status, c2c_items.raw_status),
			raw_sale_status = COALESCE(excluded.raw_sale_status, c2c_items.raw_sale_status),
			normalized_status = CASE
				WHEN excluded.raw_status IS NULL AND excluded.raw_sale_status IS NULL THEN c2c_items.normalized_status
				ELSE excluded.normalized_status
			END,
			updated_at = CURRENT_TIMESTAMP`,
		item.C2CItemsID,
		item.Type,
		item.C2CItemsName,
		item.DetailName,
		item.DetailImg,
		item.SkuID,
		item.ItemsID,
		item.TotalItemsCount,
		item.Price,
		item.ShowPrice,
		item.ShowMarketPrice,
		item.SellerUID,
		item.SellerName,
		item.PaymentTime,
		item.PublishTime,
		item.IsMyPublish,
		item.Uface,
		item.RawStatus,
		item.RawSaleStatus,
		item.NormalizedStatus,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *Database) SaveMailListToDB(response *domain.MailListResponse) int64 {
	sum := int64(0)
	for _, item := range response.Data.Data {
		detailName, detailImg, skuID, itemsID := pickMarketDetail(item)
		scrapyItem := CSCItem{
			C2CItemsID:       item.C2CItemsID,
			Type:             item.Type,
			C2CItemsName:     item.C2CItemsName,
			DetailName:       detailName,
			DetailImg:        detailImg,
			SkuID:            skuID,
			ItemsID:          itemsID,
			TotalItemsCount:  item.TotalItemsCount,
			Price:            item.Price,
			ShowPrice:        item.ShowPrice,
			ShowMarketPrice:  item.ShowMarketPrice,
			SellerUID:        item.UID,
			SellerName:       item.Uname,
			PaymentTime:      int64(item.PaymentTime),
			PublishTime:      int64(item.PaymentTime),
			IsMyPublish:      item.IsMyPublish,
			Uface:            item.Uface,
			RawStatus:        item.Status,
			RawSaleStatus:    item.SaleStatus,
			NormalizedStatus: NormalizeMarketStatus(item.Status, item.SaleStatus, nil),
		}

		rows, err := d.CreateCSCItem(&scrapyItem)
		if err != nil {
			log.Error().Err(err).Msg("CreateCSCItem failed")
		}
		sum += rows
	}
	return sum
}

func pickMarketDetail(item domain.MarketItem) (detailName, detailImg string, skuID, itemsID int64) {
	if len(item.DetailDtoList) == 0 {
		return "", "", 0, 0
	}
	detail := item.DetailDtoList[0]
	return detail.Name, detail.Img, int64(detail.SkuID), int64(detail.ItemsID)
}
