package dao

import (
	"context"
	"database/sql"
	"github.com/kun4399/BiliShareMall/internal/domain"
	"github.com/rs/zerolog/log"
	"time"
)

type ScrapyItem struct {
	Id                  int64     `json:"id"`
	AccountID           int64     `json:"accountId"`
	AccountName         string    `json:"accountName"`
	PriceFilter         string    `json:"priceFilter"`
	PriceFilterLabel    string    `json:"priceFilterLabel"`
	DiscountFilter      string    `json:"discountFilter"`
	DiscountFilterLabel string    `json:"discountFilterLabel"`
	Product             string    `json:"product"`
	ProductName         string    `json:"productName"`
	RequestIntervalSec  float64   `json:"requestIntervalSeconds"`
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
			product, product_name, account_id, request_interval_seconds, nums, increase_number, next_token, create_time, "order"
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.PriceFilter,
		item.PriceFilterLabel,
		item.DiscountFilter,
		item.DiscountFilterLabel,
		item.Product,
		item.ProductName,
		normalizeAccountID(item.AccountID),
		normalizeRequestIntervalSeconds(item.RequestIntervalSec),
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
			product = ?, product_name = ?, account_id = ?, request_interval_seconds = ?, nums = ?, increase_number = ?, next_token = ?, create_time = ?, "order" = ?
		 WHERE id = ?`,
		item.PriceFilter,
		item.PriceFilterLabel,
		item.DiscountFilter,
		item.DiscountFilterLabel,
		item.Product,
		item.ProductName,
		normalizeAccountID(item.AccountID),
		normalizeRequestIntervalSeconds(item.RequestIntervalSec),
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
			product, product_name, account_id, request_interval_seconds, nums, increase_number, next_token, create_time, "order"
		FROM scrapy_items WHERE id = ?`,
		id,
	).Scan(
		&item.PriceFilter,
		&item.PriceFilterLabel,
		&item.DiscountFilter,
		&item.DiscountFilterLabel,
		&item.Product,
		&item.ProductName,
		&item.AccountID,
		&item.RequestIntervalSec,
		&item.Nums,
		&item.IncreaseNumber,
		&item.NextToken,
		&item.CreateTime,
		&item.Order,
	)
	item.Id = int64(id)
	item.AccountName = d.resolveAuthAccountName(item.AccountID)
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
			i.id, i.price_filter, i.price_filter_label, i.discount_filter, i.discount_filter_label,
			i.product, i.product_name, i.account_id, i.request_interval_seconds, i.nums, i.increase_number, i.next_token, i.create_time, i."order",
			COALESCE(NULLIF(a.account_name, ''), a.uid, '') AS account_name
		FROM scrapy_items i
		LEFT JOIN auth_accounts a ON a.id = i.account_id`,
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
			&item.AccountID,
			&item.RequestIntervalSec,
			&item.Nums,
			&item.IncreaseNumber,
			&item.NextToken,
			&item.CreateTime,
			&item.Order,
			&item.AccountName,
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

func (d *Database) EnsureScrapyItemTaskRuntimeColumns() error {
	hasAccountID, err := tableColumnExists(d.Db, "scrapy_items", "account_id")
	if err != nil {
		return err
	}
	if !hasAccountID {
		if _, err := d.Db.ExecContext(
			context.Background(),
			`ALTER TABLE scrapy_items ADD COLUMN account_id INTEGER NOT NULL DEFAULT 0`,
		); err != nil {
			return err
		}
	}

	hasRequestInterval, err := tableColumnExists(d.Db, "scrapy_items", "request_interval_seconds")
	if err != nil {
		return err
	}
	if !hasRequestInterval {
		if _, err := d.Db.ExecContext(
			context.Background(),
			`ALTER TABLE scrapy_items ADD COLUMN request_interval_seconds REAL NOT NULL DEFAULT 3`,
		); err != nil {
			return err
		}
	}

	_, err = d.Db.ExecContext(
		context.Background(),
		`UPDATE scrapy_items
		SET request_interval_seconds = 3
		WHERE request_interval_seconds IS NULL OR request_interval_seconds < 0`,
	)
	return err
}

func normalizeRequestIntervalSeconds(seconds float64) float64 {
	if seconds < 0 {
		return 3
	}
	if seconds == 0 {
		return 0
	}
	return normalizeIntervalOneDecimal(seconds)
}

func normalizeIntervalOneDecimal(seconds float64) float64 {
	return float64(int(seconds*10+0.5)) / 10
}

func (d *Database) UpdateScrapyTaskConfig(id int, accountID int64, requestIntervalSeconds float64) error {
	requestIntervalSeconds = normalizeRequestIntervalSeconds(requestIntervalSeconds)
	result, err := d.Db.ExecContext(
		context.Background(),
		`UPDATE scrapy_items
		SET account_id = ?, request_interval_seconds = ?
		WHERE id = ?`,
		normalizeAccountID(accountID),
		requestIntervalSeconds,
		id,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeAccountID(accountID int64) int64 {
	if accountID < 0 {
		return 0
	}
	return accountID
}

func (d *Database) resolveAuthAccountName(accountID int64) string {
	if accountID <= 0 {
		return ""
	}
	var name string
	err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(NULLIF(account_name, ''), uid, '') FROM auth_accounts WHERE id = ?`,
		accountID,
	).Scan(&name)
	if err != nil && err != sql.ErrNoRows {
		return ""
	}
	return name
}

type CSCItem struct {
	C2CItemsID       int64  `json:"c2cItemsId"`
	Type             int    `json:"type"`
	C2CItemsName     string `json:"c2cItemsName"`
	DetailName       string `json:"detailName"`
	DetailImg        string `json:"detailImg"`
	SkuID            int64  `json:"skuId"`
	ItemsID          int64  `json:"itemsId"`
	ReferencePrice   int    `json:"referencePrice"`
	TotalItemsCount  int    `json:"totalItemsCount"`
	Price            int    `json:"price"`
	ShowPrice        string `json:"showPrice"`
	ShowMarketPrice  string `json:"showMarketPrice"`
	SellerUID        string `json:"sellerUid"`
	SellerName       string `json:"sellerName"`
	PaymentTime      int64  `json:"paymentTime"`
	PublishTime      int64  `json:"publishTime"`
	FirstSeenTime    int64  `json:"firstSeenTime"`
	IsMyPublish      bool   `json:"isMyPublish"`
	Uface            string `json:"uface"`
	RawStatus        *int   `json:"rawStatus,omitempty"`
	RawSaleStatus    *int   `json:"rawSaleStatus,omitempty"`
	NormalizedStatus string `json:"normalizedStatus"`
}

func (d *Database) CreateCSCItem(item *CSCItem) (int64, error) {
	result, err := d.Db.ExecContext(context.Background(),
		`INSERT INTO c2c_items (
			c2c_items_id, type, c2c_items_name, detail_name, detail_img, sku_id, items_id, reference_price,
			total_items_count, price, show_price, show_market_price, seller_uid, seller_name,
			payment_time, publish_time, is_my_publish, uface, raw_status, raw_sale_status,
			normalized_status, status_checked_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(c2c_items_id) DO UPDATE SET
			type = excluded.type,
			c2c_items_name = excluded.c2c_items_name,
			detail_name = excluded.detail_name,
			detail_img = excluded.detail_img,
			sku_id = excluded.sku_id,
			items_id = excluded.items_id,
			reference_price = excluded.reference_price,
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
		item.ReferencePrice,
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
	sum, err := d.SaveMailListToDBStrict(response)
	if err != nil {
		log.Error().Err(err).Msg("SaveMailListToDBStrict failed")
	}
	return sum
}

func (d *Database) SaveMailListToDBStrict(response *domain.MailListResponse) (int64, error) {
	sum := int64(0)
	for _, item := range response.Data.Data {
		detailName, detailImg, skuID, itemsID, referencePrice := pickMarketDetail(item)
		scrapyItem := CSCItem{
			C2CItemsID:       item.C2CItemsID,
			Type:             item.Type,
			C2CItemsName:     item.C2CItemsName,
			DetailName:       detailName,
			DetailImg:        detailImg,
			SkuID:            skuID,
			ItemsID:          itemsID,
			ReferencePrice:   referencePrice,
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
			return sum, err
		}
		sum += rows
	}
	return sum, nil
}

func pickMarketDetail(item domain.MarketItem) (detailName, detailImg string, skuID, itemsID int64, referencePrice int) {
	if len(item.DetailDtoList) == 0 {
		return "", "", 0, 0, 0
	}
	detail := item.DetailDtoList[0]
	return detail.Name, detail.Img, int64(detail.SkuID), int64(detail.ItemsID), detail.MarketPrice
}
