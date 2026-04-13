package app

import (
	"testing"

	"github.com/kun4399/BiliShareMall/internal/dao"
)

func TestGetMarketRuntimeConfigReturnsFallbackWithoutCookie(t *testing.T) {
	a := &App{}
	config := a.GetMarketRuntimeConfig("")
	if len(config.Categories) == 0 {
		t.Fatal("expected categories in fallback config")
	}
	if len(config.Sorts) == 0 {
		t.Fatal("expected sorts in fallback config")
	}
}

func TestCreateScrapyItemStructCarriesFilterFields(t *testing.T) {
	item := dao.ScrapyItem{
		Product:             "2312",
		ProductName:         "手办",
		Order:               "TIME_DESC",
		PriceFilter:         "10000-20000",
		PriceFilterLabel:    "100-200元",
		DiscountFilter:      "50-70",
		DiscountFilterLabel: "50-70%",
	}
	if item.PriceFilter == "" || item.DiscountFilter == "" {
		t.Fatal("expected non-empty filter fields")
	}
}
