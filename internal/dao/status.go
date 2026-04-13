package dao

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
	if rawSaleStatus != nil && *rawSaleStatus > 0 {
		return true
	}
	return rawStatus != nil && *rawStatus > 1
}

func isOffSale(rawStatus *int) bool {
	return rawStatus != nil && *rawStatus > 0
}
