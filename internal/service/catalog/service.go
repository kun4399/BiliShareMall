package catalog

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/mikumifa/BiliShareMall/internal/dao"
	bilihttp "github.com/mikumifa/BiliShareMall/internal/http"
	cache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
)

type C2CItemGroupListVO struct {
	Items       []C2CItemGroupVO `json:"items"`
	Total       int              `json:"total"`
	TotalPages  int              `json:"totalPages"`
	CurrentPage int              `json:"currentPage"`
}

type C2CItemGroupVO struct {
	SkuID             int64  `json:"skuId"`
	C2CItemsName      string `json:"c2cItemsName"`
	DetailImg         string `json:"detailImg"`
	ItemCount         int    `json:"itemCount"`
	LatestPublishTime int64  `json:"latestPublishTime"`
}

type C2CItemDetailListVO struct {
	SkuID        int64             `json:"skuId"`
	C2CItemsName string            `json:"c2cItemsName"`
	DetailImg    string            `json:"detailImg"`
	Items        []C2CItemDetailVO `json:"items"`
	Total        int               `json:"total"`
	TotalPages   int               `json:"totalPages"`
	CurrentPage  int               `json:"currentPage"`
}

type C2CItemDetailVO struct {
	C2CItemsID  int64   `json:"c2cItemsId"`
	SkuID       int64   `json:"skuId"`
	Price       float64 `json:"price"`
	ShowPrice   string  `json:"showPrice"`
	SellerName  string  `json:"sellerName"`
	SellerUID   string  `json:"sellerUID"`
	PublishTime int64   `json:"publishTime"`
	Status      string  `json:"status"`
	Link        string  `json:"link"`
}

type Service struct {
	d *dao.Database
	c *cache.Cache
}

func NewService(database *dao.Database, c *cache.Cache) *Service {
	return &Service{
		d: database,
		c: c,
	}
}

func (s *Service) ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) (ret C2CItemGroupListVO, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Any("panic", r).Bytes("stack", debug.Stack()).Msg("panic recovered in ListC2CItem")
			ret = C2CItemGroupListVO{}
			err = fmt.Errorf("search failed due to internal error")
		}
	}()

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}

	items, total, err := s.d.ReadC2CItemGroups(page, pageSize, filterName, sortOption, startTime, endTime, fromPrice, toPrice)
	if err != nil {
		log.Error().Err(err).Msg("failed to list grouped items")
		return C2CItemGroupListVO{}, err
	}

	result := make([]C2CItemGroupVO, 0, len(items))
	for _, item := range items {
		result = append(result, C2CItemGroupVO{
			SkuID:             item.SkuID,
			C2CItemsName:      item.C2CItemsName,
			DetailImg:         item.DetailImg,
			ItemCount:         item.ItemCount,
			LatestPublishTime: item.LatestPublishTime,
		})
	}

	return C2CItemGroupListVO{
		Items:       result,
		Total:       total,
		TotalPages:  calcTotalPages(total, pageSize),
		CurrentPage: page,
	}, nil
}

func (s *Service) ListC2CItemDetailBySku(skuID int64, page, pageSize int, sortOption int, statusFilter, cookieStr string) (ret C2CItemDetailListVO, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Any("panic", r).Bytes("stack", debug.Stack()).Msg("panic recovered in ListC2CItemDetailBySku")
			ret = C2CItemDetailListVO{}
			err = fmt.Errorf("detail query failed due to internal error")
		}
	}()

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	meta, err := s.d.GetC2CItemGroupMeta(skuID)
	if err != nil {
		log.Error().Err(err).Int64("skuId", skuID).Msg("failed to load group meta")
		return C2CItemDetailListVO{}, err
	}

	if cookieStr != "" && statusFilter != "" {
		allItems, readErr := s.d.ReadAllC2CItemDetailsBySku(skuID)
		if readErr == nil {
			if _, refreshErr := s.refreshDetailStatuses(allItems, cookieStr, true); refreshErr != nil {
				log.Warn().Err(refreshErr).Int64("skuId", skuID).Msg("failed to refresh all item statuses for filtered query")
			}
		}
	}

	items, total, err := s.d.ReadC2CItemDetailsBySku(skuID, page, pageSize, sortOption, statusFilter)
	if err != nil {
		log.Error().Err(err).Int64("skuId", skuID).Msg("failed to list item details")
		return C2CItemDetailListVO{}, err
	}

	if cookieStr != "" && statusFilter == "" {
		changed, refreshErr := s.refreshDetailStatuses(items, cookieStr, true)
		if refreshErr != nil {
			log.Warn().Err(refreshErr).Int64("skuId", skuID).Msg("failed to refresh current page item statuses")
		}
		if changed {
			items, total, err = s.d.ReadC2CItemDetailsBySku(skuID, page, pageSize, sortOption, statusFilter)
			if err != nil {
				return C2CItemDetailListVO{}, err
			}
		}
	}

	result := make([]C2CItemDetailVO, 0, len(items))
	for _, item := range items {
		result = append(result, C2CItemDetailVO{
			C2CItemsID:  item.C2CItemsID,
			SkuID:       item.SkuID,
			Price:       float64(item.Price) / 100,
			ShowPrice:   item.ShowPrice,
			SellerName:  item.SellerName,
			SellerUID:   item.SellerUID,
			PublishTime: item.PublishTime,
			Status:      item.NormalizedStatus,
			Link:        buildItemLink(item.C2CItemsID),
		})
	}

	return C2CItemDetailListVO{
		SkuID:        meta.SkuID,
		C2CItemsName: meta.C2CItemsName,
		DetailImg:    meta.DetailImg,
		Items:        result,
		Total:        total,
		TotalPages:   calcTotalPages(total, pageSize),
		CurrentPage:  page,
	}, nil
}

func (s *Service) refreshDetailStatuses(items []dao.CSCItem, cookieStr string, forceRefresh bool) (bool, error) {
	changed := false
	var firstErr error

	for _, item := range items {
		canBuy, err := s.checkItemStatus(item.C2CItemsID, item.Price, cookieStr, forceRefresh)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Error().Err(err).Int64("itemId", item.C2CItemsID).Msg("failed to check item status")
			continue
		}

		status := dao.NormalizeMarketStatus(item.RawStatus, item.RawSaleStatus, &canBuy)
		if err := s.d.UpdateC2CItemStatus(item.C2CItemsID, status, time.Now()); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Error().Err(err).Int64("itemId", item.C2CItemsID).Msg("failed to update item status")
			continue
		}
		if status != item.NormalizedStatus {
			changed = true
		}
	}

	return changed, firstErr
}

func (s *Service) checkItemStatus(id int64, price int, cookieStr string, forceRefresh bool) (bool, error) {
	cacheStore := s.c
	if cacheStore == nil {
		cacheStore = cache.New(5*time.Minute, 10*time.Minute)
		s.c = cacheStore
	}

	cacheKey := fmt.Sprintf("check:%d:%d", id, price)
	if !forceRefresh {
		if result, found := cacheStore.Get(cacheKey); found {
			return result.(bool), nil
		}
	} else {
		cacheStore.Delete(cacheKey)
	}

	if result, found := cacheStore.Get(cacheKey); found {
		return result.(bool), nil
	}

	client, err := bilihttp.NewBiliClient()
	if err != nil {
		return false, err
	}

	resp, err := client.CheckC2CItem(context.Background(), bilihttp.ParseBiliSession(cookieStr), id, price)
	if err != nil {
		return false, err
	}

	canBuy := resp.Code != 60000002
	cacheStore.Set(cacheKey, canBuy, cache.DefaultExpiration)
	time.Sleep(300 * time.Millisecond)
	return canBuy, nil
}

func calcTotalPages(total, pageSize int) int {
	if total <= 0 {
		return 1
	}
	return (total + pageSize - 1) / pageSize
}

func buildItemLink(c2cItemsID int64) string {
	return fmt.Sprintf("https://mall.bilibili.com/neul-next/index.html?page=magic-market_detail&noTitleBar=1&itemsId=%d", c2cItemsID)
}
