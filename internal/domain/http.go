package domain

type MarketFilterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type MarketRuntimeConfig struct {
	Categories      []MarketFilterOption `json:"categories"`
	Sorts           []MarketFilterOption `json:"sorts"`
	PriceFilters    []MarketFilterOption `json:"priceFilters"`
	DiscountFilters []MarketFilterOption `json:"discountFilters"`
	Source          string               `json:"source"`
	Message         string               `json:"message"`
}

type MarketItem struct {
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
	UspaceJumpURL   any    `json:"uspaceJumpUrl"`
	Uface           string `json:"uface"`
	Uname           string `json:"uname"`
	Status          *int   `json:"status,omitempty"`
	SaleStatus      *int   `json:"saleStatus,omitempty"`
	DetailDtoList   []struct {
		BlindBoxID  int    `json:"blindBoxId"`
		ItemsID     int    `json:"itemsId"`
		SkuID       int    `json:"skuId"`
		Name        string `json:"name"`
		Img         string `json:"img"`
		MarketPrice int    `json:"marketPrice"`
		Type        int    `json:"type"`
		IsHidden    bool   `json:"isHidden"`
	} `json:"detailDtoList"`
}

// MailListResponse keeps the historical name because it is referenced in DAO code.
type MailListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Data   []MarketItem `json:"data"`
		NextID *string      `json:"nextId"`
	} `json:"data"`
	Errtag int `json:"errtag"`
}

type MarketNavbarResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ShowSearchInput bool                 `json:"showSearchInput"`
		ShowFilters     bool                 `json:"showFilters"`
		CategoryItems   []MarketFilterOption `json:"categoryItems"`
		SortItems       []MarketFilterOption `json:"sortItems"`
		PriceItems      []MarketFilterOption `json:"priceItems"`
		DiscountItems   []MarketFilterOption `json:"discountItems"`
	} `json:"data"`
}

type CheckResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OrderItems struct {
			C2CItemsID int64 `json:"c2cItemsId"`
			Price      int   `json:"price"`
		} `json:"orderItems"`
	} `json:"data"`
}
