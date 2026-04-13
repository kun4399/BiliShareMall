package dao

import "strings"

const (
	StatusOnSale  = "在售"
	StatusOffSale = "下架"
	StatusSoldOut = "已售出"
)

func NormalizeMarketStatus(rawStatus, rawSaleStatus *int, canBuy *bool) string {
	if canBuy != nil {
		if *canBuy {
			return StatusOnSale
		}
		if isSoldOut(rawStatus, rawSaleStatus) {
			return StatusSoldOut
		}
		return StatusOffSale
	}

	if isSoldOut(rawStatus, rawSaleStatus) {
		return StatusSoldOut
	}
	if isOffSale(rawStatus) {
		return StatusOffSale
	}
	return StatusOnSale
}

func isSoldOut(rawStatus, rawSaleStatus *int) bool {
	if rawSaleStatus != nil && *rawSaleStatus > 1 {
		return true
	}
	return rawStatus != nil && *rawStatus > 1
}

func isOffSale(rawStatus *int) bool {
	return rawStatus != nil && *rawStatus > 0
}

func NormalizeMarketStatusFromDetail(publishStatus, rawStatus, rawSaleStatus *int, dropReason string) string {
	if publishStatus != nil {
		switch *publishStatus {
		case 1:
			return StatusOnSale
		case 2:
			if hasSoldOutSignal(dropReason, rawStatus, rawSaleStatus) {
				return StatusSoldOut
			}
			return StatusOffSale
		}
	}

	if hasSoldOutSignal(dropReason, rawStatus, rawSaleStatus) {
		return StatusSoldOut
	}
	if hasOffSaleSignal(dropReason, rawStatus) {
		return StatusOffSale
	}
	return NormalizeMarketStatus(rawStatus, rawSaleStatus, nil)
}

func hasSoldOutSignal(dropReason string, rawStatus, rawSaleStatus *int) bool {
	if containsAny(dropReason, "售出", "已售", "成交", "售罄") {
		return true
	}
	if rawSaleStatus != nil && *rawSaleStatus > 1 {
		return true
	}
	return rawStatus != nil && *rawStatus > 1
}

func hasOffSaleSignal(dropReason string, rawStatus *int) bool {
	if containsAny(dropReason, "下架", "到期") {
		return true
	}
	return isOffSale(rawStatus)
}

func containsAny(value string, tokens ...string) bool {
	if value == "" {
		return false
	}
	normalized := strings.TrimSpace(value)
	for _, token := range tokens {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
