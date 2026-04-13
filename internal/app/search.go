package app

import catalogsvc "github.com/kun4399/BiliShareMall/internal/service/catalog"

type C2CItemGroupListVO = catalogsvc.C2CItemGroupListVO
type C2CItemGroupVO = catalogsvc.C2CItemGroupVO
type C2CItemDetailListVO = catalogsvc.C2CItemDetailListVO
type C2CItemDetailVO = catalogsvc.C2CItemDetailVO

func (a *App) ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) (C2CItemGroupListVO, error) {
	return a.getCatalogService().ListC2CItem(page, pageSize, filterName, sortOption, startTime, endTime, fromPrice, toPrice)
}

func (a *App) GetC2CItemNameBySku(skuID int64) (string, error) {
	return a.getCatalogService().GetC2CItemNameBySku(skuID)
}

func (a *App) ListC2CItemDetailBySku(skuID int64, page, pageSize int, sortOption int, statusFilter, cookieStr string) (C2CItemDetailListVO, error) {
	return a.getCatalogService().ListC2CItemDetailBySku(skuID, page, pageSize, sortOption, statusFilter, cookieStr)
}
