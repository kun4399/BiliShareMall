package http

import (
	"context"
	"fmt"
	neturl "net/url"

	"github.com/mikumifa/BiliShareMall/internal/domain"
)

const (
	marketBaseURL = "https://mall.bilibili.com/mall-magic-c"
	marketReferer = "https://mall.bilibili.com/neul-next/index.html?page=magic-market_index&noTitleBar=1"
)

type MarketListRequest struct {
	SortType        string   `json:"sortType"`
	NextID          *string  `json:"nextId"`
	PriceFilters    []string `json:"priceFilters"`
	DiscountFilters []string `json:"discountFilters"`
	CategoryFilter  string   `json:"categoryFilter"`
}

func DefaultMarketRuntimeConfig() domain.MarketRuntimeConfig {
	return domain.MarketRuntimeConfig{
		Categories: []domain.MarketFilterOption{
			{Label: "全部", Value: ""},
			{Label: "手办", Value: "2312"},
			{Label: "模型", Value: "2066"},
			{Label: "周边", Value: "2331"},
			{Label: "3C", Value: "2273"},
			{Label: "福袋", Value: "fudai_cate_id"},
		},
		Sorts: []domain.MarketFilterOption{
			{Label: "最新", Value: "TIME_DESC"},
			{Label: "价格升序", Value: "PRICE_ASC"},
			{Label: "价格降序", Value: "PRICE_DESC"},
		},
		PriceFilters: []domain.MarketFilterOption{
			{Label: "不限", Value: ""},
			{Label: "0-50元", Value: "0-5000"},
			{Label: "50-100元", Value: "5000-10000"},
			{Label: "100-200元", Value: "10000-20000"},
			{Label: "200-500元", Value: "20000-50000"},
			{Label: "500元以上", Value: "50000-99999999"},
		},
		DiscountFilters: []domain.MarketFilterOption{
			{Label: "不限", Value: ""},
			{Label: "0-30%", Value: "0-30"},
			{Label: "30-50%", Value: "30-50"},
			{Label: "50-70%", Value: "50-70"},
			{Label: "70-100%", Value: "70-100"},
		},
		Source:  "fallback",
		Message: "using bundled fallback config",
	}
}

func (c *BiliClient) marketHeaders(session *BiliSession, referer string) map[string]string {
	headers := map[string]string{
		"Origin":  "https://mall.bilibili.com",
		"Referer": referer,
	}
	if session != nil && session.CookieHeader() != "" {
		headers["Cookie"] = session.CookieHeader()
	}
	return headers
}

func (c *BiliClient) GetMarketRuntimeConfig(ctx context.Context, session *BiliSession) (domain.MarketRuntimeConfig, error) {
	fallback := DefaultMarketRuntimeConfig()
	if session == nil {
		return fallback, nil
	}
	if err := session.EnsureFingerprint(ctx, c); err != nil {
		return fallback, err
	}

	query := neturl.Values{}
	query.Set("csrf", session.CSRF())

	var resp domain.MarketNavbarResponse
	err := c.DoJSON(ctx, GET, marketBaseURL+"/internet/c2c/v2/navbar", query, nil, c.marketHeaders(session, marketReferer), &resp)
	if err != nil {
		return fallback, err
	}
	if apiErr := classifyMarketError(resp.Code, resp.Message); apiErr != nil {
		return fallback, apiErr
	}

	return domain.MarketRuntimeConfig{
		Categories:      nonEmptyOptions(resp.Data.CategoryItems, fallback.Categories),
		Sorts:           nonEmptyOptions(resp.Data.SortItems, fallback.Sorts),
		PriceFilters:    prependAllOption(nonEmptyOptions(resp.Data.PriceItems, fallback.PriceFilters)),
		DiscountFilters: prependAllOption(nonEmptyOptions(resp.Data.DiscountItems, fallback.DiscountFilters)),
		Source:          "remote",
		Message:         "loaded from market navbar",
	}, nil
}

func (c *BiliClient) ListMarketItems(ctx context.Context, session *BiliSession, req MarketListRequest) (domain.MailListResponse, error) {
	var resp domain.MailListResponse
	if session == nil {
		return resp, &APIError{Kind: ErrKindUnauthorized, Code: 83001002, Message: "missing login session"}
	}
	if err := session.EnsureFingerprint(ctx, c); err != nil {
		return resp, err
	}

	payload := map[string]any{
		"sortType":        req.SortType,
		"nextId":          req.NextID,
		"priceFilters":    normalizeFilterList(req.PriceFilters),
		"discountFilters": normalizeFilterList(req.DiscountFilters),
		"categoryFilter":  req.CategoryFilter,
		"csrf":            session.CSRF(),
	}

	err := c.DoJSON(ctx, POST, marketBaseURL+"/internet/c2c/v2/list", nil, payload, c.marketHeaders(session, marketReferer), &resp)
	if err != nil {
		return resp, err
	}
	if apiErr := classifyMarketError(resp.Code, resp.Message); apiErr != nil {
		return resp, apiErr
	}
	return resp, nil
}

func (c *BiliClient) CheckC2CItem(ctx context.Context, session *BiliSession, itemID int64, price int) (domain.CheckResponse, error) {
	var resp domain.CheckResponse
	if session == nil || !session.IsLoggedIn() {
		return resp, &APIError{Kind: ErrKindUnauthorized, Code: 83001002, Message: "login required"}
	}
	if err := session.EnsureFingerprint(ctx, c); err != nil {
		return resp, err
	}

	payload := map[string]any{
		"items": map[string]any{
			"c2cItemsId": itemID,
			"price":      price,
		},
	}
	query := neturl.Values{}
	query.Set("platform", "h5")

	err := c.DoJSON(ctx, POST, marketBaseURL+"/c2c/order/info", query, payload, c.marketHeaders(session, fmt.Sprintf("%s&itemsId=%d", marketReferer, itemID)), &resp)
	if err != nil {
		return resp, err
	}
	if apiErr := classifyMarketError(resp.Code, resp.Message); apiErr != nil {
		return resp, apiErr
	}
	return resp, nil
}

func (c *BiliClient) QueryC2CItemDetail(ctx context.Context, session *BiliSession, itemID int64) (domain.C2CItemDetailResponse, error) {
	var resp domain.C2CItemDetailResponse
	query := neturl.Values{}
	query.Set("c2cItemsId", fmt.Sprintf("%d", itemID))
	if session != nil && session.CSRF() != "" {
		query.Set("csrf", session.CSRF())
	}

	err := c.DoJSON(
		ctx,
		GET,
		marketBaseURL+"/internet/c2c/items/queryC2cItemsDetail",
		query,
		nil,
		c.marketHeaders(session, fmt.Sprintf("%s&itemsId=%d", marketReferer, itemID)),
		&resp,
	)
	if err != nil {
		return resp, err
	}
	if apiErr := classifyMarketError(resp.Code, resp.Message); apiErr != nil {
		return resp, apiErr
	}
	return resp, nil
}

func normalizeFilterList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func nonEmptyOptions(options []domain.MarketFilterOption, fallback []domain.MarketFilterOption) []domain.MarketFilterOption {
	if len(options) == 0 {
		return fallback
	}
	return options
}

func prependAllOption(options []domain.MarketFilterOption) []domain.MarketFilterOption {
	if len(options) == 0 {
		return options
	}
	if options[0].Value == "" {
		return options
	}
	return append([]domain.MarketFilterOption{{Label: "不限", Value: ""}}, options...)
}
