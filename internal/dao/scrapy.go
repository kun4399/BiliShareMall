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
	C2CItemsID      int64  `json:"c2cItemsId"`
	Type            int    `json:"type"`
	C2CItemsName    string `json:"c2cItemsName"`
	TotalItemsCount int    `json:"totalItemsCount"`
	Price           int    `json:"price"`
	ShowPrice       string `json:"showPrice"`
	ShowMarketPrice string `json:"showMarketPrice"`
	UID             string `json:"uid"`
	PaymentTime     int    `json:"paymentTime"`
	IsMyPublish     bool   `json:"isMyPublish"`
	Uface           string `json:"uface"`
	Uname           string `json:"uname"`
}

func (d *Database) CreateCSCItem(item *CSCItem) (int64, error) {
	result, err := d.Db.ExecContext(context.Background(),
		"INSERT or IGNORE  INTO c2c_items (c2c_items_id, type, c2c_items_name, total_items_count, price, show_price, show_market_price, uid, payment_time, is_my_publish, uface, uname) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		item.C2CItemsID, item.Type, item.C2CItemsName, item.TotalItemsCount, item.Price, item.ShowPrice, item.ShowMarketPrice, item.UID, item.PaymentTime, item.IsMyPublish, item.Uface, item.Uname)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *Database) SaveMailListToDB(response *domain.MailListResponse) int64 {
	sum := int64(0)
	for _, item := range response.Data.Data {
		scrapyItem := CSCItem{
			C2CItemsID:      item.C2CItemsID,
			Type:            item.Type,
			C2CItemsName:    item.C2CItemsName,
			TotalItemsCount: item.TotalItemsCount,
			Price:           item.Price,
			ShowPrice:       item.ShowPrice,
			ShowMarketPrice: item.ShowMarketPrice,
			UID:             item.UID,
			PaymentTime:     item.PaymentTime,
			IsMyPublish:     item.IsMyPublish,
			Uface:           item.Uface,
			Uname:           item.Uname,
		}

		rows, err := d.CreateCSCItem(&scrapyItem)
		if err != nil {
			log.Error().Err(err).Msg("CreateCSCItem failed")
		}
		sum += rows
	}
	return sum
}
