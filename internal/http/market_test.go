package http

import (
	"reflect"
	"testing"

	"github.com/kun4399/BiliShareMall/internal/domain"
)

func TestDefaultMarketRuntimeConfigMatchesCurrentMarketNavbar(t *testing.T) {
	config := DefaultMarketRuntimeConfig()

	expectedCategories := []domain.MarketFilterOption{
		{Label: "手办", Value: "2312"},
		{Label: "模型", Value: "2066"},
		{Label: "周边", Value: "2331"},
		{Label: "3C", Value: "2273"},
		{Label: "福袋", Value: "fudai_cate_id"},
	}
	expectedSorts := []domain.MarketFilterOption{
		{Label: "综合", Value: "TIME_DESC"},
		{Label: "价格升序", Value: "PRICE_ASC"},
		{Label: "价格倒序", Value: "PRICE_DESC"},
	}
	expectedPrices := []domain.MarketFilterOption{
		{Label: "不限", Value: ""},
		{Label: "20以下", Value: "0-2000"},
		{Label: "20-30", Value: "2000-3000"},
		{Label: "30-50", Value: "3000-5000"},
		{Label: "50-100", Value: "5000-10000"},
		{Label: "100-200", Value: "10000-20000"},
		{Label: "200以上", Value: "20000-0"},
	}
	expectedDiscounts := []domain.MarketFilterOption{
		{Label: "不限", Value: ""},
		{Label: "3折以下", Value: "0-30"},
		{Label: "3-5折", Value: "30-50"},
		{Label: "5-7折", Value: "50-70"},
		{Label: "7折以上", Value: "70-100"},
	}

	if !reflect.DeepEqual(config.Categories, expectedCategories) {
		t.Fatalf("unexpected bundled categories: %#v", config.Categories)
	}
	if !reflect.DeepEqual(config.Sorts, expectedSorts) {
		t.Fatalf("unexpected bundled sorts: %#v", config.Sorts)
	}
	if !reflect.DeepEqual(config.PriceFilters, expectedPrices) {
		t.Fatalf("unexpected bundled price filters: %#v", config.PriceFilters)
	}
	if !reflect.DeepEqual(config.DiscountFilters, expectedDiscounts) {
		t.Fatalf("unexpected bundled discount filters: %#v", config.DiscountFilters)
	}
}
