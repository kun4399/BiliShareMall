package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/domain"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
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
	SkuID               int64  `json:"skuId"`
	C2CItemsName        string `json:"c2cItemsName"`
	DetailImg           string `json:"detailImg"`
	ItemCount           int    `json:"itemCount"`
	ReferencePriceMin   int    `json:"referencePriceMin"`
	ReferencePriceMax   int    `json:"referencePriceMax"`
	ReferencePriceLabel string `json:"referencePriceLabel"`
	FirstSeenTime       int64  `json:"firstSeenTime"`
	LatestPublishTime   int64  `json:"latestPublishTime"`
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
	C2CItemsID    int64   `json:"c2cItemsId"`
	SkuID         int64   `json:"skuId"`
	Price         float64 `json:"price"`
	ShowPrice     string  `json:"showPrice"`
	SellerName    string  `json:"sellerName"`
	SellerUID     string  `json:"sellerUID"`
	PublishTime   int64   `json:"publishTime"`
	FirstSeenTime int64   `json:"firstSeenTime"`
	Status        string  `json:"status"`
	Link          string  `json:"link"`
}

type Service struct {
	d                  *dao.Database
	c                  *cache.Cache
	emit               EventEmitter
	statusResolver     StatusResolver
	statusRefreshSlots chan struct{}
	statusRefreshMu    sync.Mutex
	statusRefreshing   map[int64]struct{}
}

type EventEmitter func(eventName string, payload any)

type StatusResolver func(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error)

type StatusesChangedEvent struct {
	SkuID     int64 `json:"skuId"`
	UpdatedAt int64 `json:"updatedAt"`
}

const (
	statusRefreshMaxAge  = 5 * time.Minute
	statusRefreshWorkers = 2
)

func NewService(database *dao.Database, c *cache.Cache, emitters ...EventEmitter) *Service {
	var emit EventEmitter
	if len(emitters) > 0 {
		emit = emitters[0]
	}
	service := &Service{
		d:                  database,
		c:                  c,
		emit:               emit,
		statusRefreshSlots: make(chan struct{}, statusRefreshWorkers),
		statusRefreshing:   make(map[int64]struct{}),
	}
	service.statusResolver = service.resolveItemStatus
	return service
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
			SkuID:               item.SkuID,
			C2CItemsName:        item.C2CItemsName,
			DetailImg:           item.DetailImg,
			ItemCount:           item.ItemCount,
			ReferencePriceMin:   item.ReferencePriceMin,
			ReferencePriceMax:   item.ReferencePriceMax,
			ReferencePriceLabel: formatReferencePriceLabel(item.ReferencePriceMin, item.ReferencePriceMax),
			FirstSeenTime:       item.FirstSeenTime,
			LatestPublishTime:   item.LatestPublishTime,
		})
	}

	return C2CItemGroupListVO{
		Items:       result,
		Total:       total,
		TotalPages:  calcTotalPages(total, pageSize),
		CurrentPage: page,
	}, nil
}

func (s *Service) GetC2CItemNameBySku(skuID int64) (string, error) {
	if skuID <= 0 {
		return "", nil
	}
	cacheStore := s.c
	if cacheStore == nil {
		cacheStore = cache.New(5*time.Minute, 10*time.Minute)
		s.c = cacheStore
	}
	cacheKey := fmt.Sprintf("sku-name:%d", skuID)
	if cached, found := cacheStore.Get(cacheKey); found {
		return cached.(string), nil
	}

	meta, err := s.d.GetC2CItemGroupMeta(skuID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if meta.C2CItemsName != "" {
		cacheStore.SetDefault(cacheKey, meta.C2CItemsName)
	}
	return meta.C2CItemsName, nil
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

	items, total, err := s.d.ReadC2CItemDetailsBySku(skuID, page, pageSize, sortOption, statusFilter)
	if err != nil {
		log.Error().Err(err).Int64("skuId", skuID).Msg("failed to list item details")
		return C2CItemDetailListVO{}, err
	}

	result := make([]C2CItemDetailVO, 0, len(items))
	for _, item := range items {
		result = append(result, C2CItemDetailVO{
			C2CItemsID:    item.C2CItemsID,
			SkuID:         item.SkuID,
			Price:         float64(item.Price) / 100,
			ShowPrice:     item.ShowPrice,
			SellerName:    item.SellerName,
			SellerUID:     item.SellerUID,
			PublishTime:   item.PublishTime,
			FirstSeenTime: item.FirstSeenTime,
			Status:        item.NormalizedStatus,
			Link:          buildItemLink(item.C2CItemsID),
		})
	}

	response := C2CItemDetailListVO{
		SkuID:        meta.SkuID,
		C2CItemsName: meta.C2CItemsName,
		DetailImg:    meta.DetailImg,
		Items:        result,
		Total:        total,
		TotalPages:   calcTotalPages(total, pageSize),
		CurrentPage:  page,
	}

	// Remote status checks used to run serially on this request path. A page of
	// ten records could therefore withhold the entire JSON response for minutes
	// when the upstream was slow. Serve the persisted snapshot immediately and
	// refresh stale statuses with bounded background workers instead.
	s.scheduleStatusRefresh(skuID, items, cookieStr)

	return response, nil
}

func (s *Service) scheduleStatusRefresh(skuID int64, items []dao.CSCItem, cookieStr string) {
	s.statusRefreshMu.Lock()
	if _, exists := s.statusRefreshing[skuID]; exists {
		s.statusRefreshMu.Unlock()
		return
	}
	s.statusRefreshing[skuID] = struct{}{}
	s.statusRefreshMu.Unlock()

	go s.refreshStatuses(skuID, items, cookieStr)
}

func (s *Service) refreshStatuses(skuID int64, visibleItems []dao.CSCItem, cookieStr string) {
	s.statusRefreshSlots <- struct{}{}
	defer func() { <-s.statusRefreshSlots }()
	defer func() {
		s.statusRefreshMu.Lock()
		delete(s.statusRefreshing, skuID)
		s.statusRefreshMu.Unlock()
	}()

	items, err := s.d.ReadC2CItemDetailsNeedingStatusRefresh(skuID, time.Now().Add(-statusRefreshMaxAge))
	if err != nil {
		log.Error().Err(err).Int64("skuId", skuID).Msg("failed to load stale item statuses")
		return
	}
	items = prioritizeStatusRefreshItems(items, visibleItems)
	if len(items) == 0 {
		return
	}

	ctx := bilihttp.WithMarketRequestPriority(context.Background(), bilihttp.MarketRequestLowPriority)
	for _, item := range items {
		status, err := s.statusResolver(ctx, item, cookieStr)
		if err != nil {
			log.Warn().Err(err).Int64("itemId", item.C2CItemsID).Msg("failed to refresh item status")
			continue
		}
		if err := s.d.UpdateC2CItemStatus(item.C2CItemsID, status, time.Now()); err != nil {
			log.Error().Err(err).Int64("itemId", item.C2CItemsID).Msg("failed to persist refreshed item status")
			continue
		}
		if status != item.NormalizedStatus && s.emit != nil {
			s.emit("catalog_statuses_changed", StatusesChangedEvent{
				SkuID:     skuID,
				UpdatedAt: time.Now().UnixMilli(),
			})
		}
	}
}

func prioritizeStatusRefreshItems(items, visibleItems []dao.CSCItem) []dao.CSCItem {
	if len(items) < 2 || len(visibleItems) == 0 {
		return items
	}

	staleByID := make(map[int64]dao.CSCItem, len(items))
	for _, item := range items {
		staleByID[item.C2CItemsID] = item
	}

	prioritized := make([]dao.CSCItem, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, visible := range visibleItems {
		item, exists := staleByID[visible.C2CItemsID]
		if !exists {
			continue
		}
		prioritized = append(prioritized, item)
		seen[item.C2CItemsID] = struct{}{}
	}
	for _, item := range items {
		if _, exists := seen[item.C2CItemsID]; exists {
			continue
		}
		prioritized = append(prioritized, item)
	}
	return prioritized
}

func (s *Service) resolveItemStatus(ctx context.Context, item dao.CSCItem, cookieStr string) (string, error) {
	detailStatus, detailErr := s.checkItemStatusFromDetail(ctx, item.C2CItemsID, cookieStr)
	if detailErr == nil {
		return detailStatus, nil
	}

	if !bilihttp.ParseBiliSession(cookieStr).IsLoggedIn() {
		return "", detailErr
	}

	canBuy, fallbackErr := s.checkItemStatus(ctx, item.C2CItemsID, item.Price, cookieStr)
	if fallbackErr == nil {
		return dao.NormalizeMarketStatus(item.RawStatus, item.RawSaleStatus, &canBuy), nil
	}

	return "", fmt.Errorf("detail check failed: %w; fallback check failed: %v", detailErr, fallbackErr)
}

func (s *Service) checkItemStatusFromDetail(ctx context.Context, id int64, cookieStr string) (string, error) {
	cacheStore := s.c
	if cacheStore == nil {
		cacheStore = cache.New(5*time.Minute, 10*time.Minute)
		s.c = cacheStore
	}

	cacheKey := fmt.Sprintf("detail-status:%d", id)
	if result, found := cacheStore.Get(cacheKey); found {
		return result.(string), nil
	}

	client, err := bilihttp.NewBiliClient()
	if err != nil {
		return "", err
	}

	session := bilihttp.ParseBiliSession(cookieStr)
	resp, err := client.QueryC2CItemDetail(ctx, session, id)
	if err != nil {
		return "", err
	}
	status, err := normalizeDetailStatus(id, resp.Data)
	if err != nil {
		return "", err
	}
	cacheStore.Set(cacheKey, status, cache.DefaultExpiration)
	time.Sleep(150 * time.Millisecond)
	return status, nil
}

func normalizeDetailStatus(itemID int64, detail domain.C2CItemDetailStatus) (string, error) {
	if detail.C2CItemsID != 0 && detail.C2CItemsID != itemID {
		return "", fmt.Errorf("detail response item id mismatch: got %d, want %d", detail.C2CItemsID, itemID)
	}
	if detail.PublishStatus == nil && detail.Status == nil && detail.SaleStatus == nil {
		if strings.TrimSpace(detail.DropReason) == "" {
			return "", errors.New("detail response did not contain a status signal")
		}
		status := dao.NormalizeMarketStatusFromDetail(nil, nil, nil, detail.DropReason)
		if status == dao.StatusOnSale {
			return "", fmt.Errorf("detail response contained an unrecognized drop reason: %q", detail.DropReason)
		}
		return status, nil
	}
	return dao.NormalizeMarketStatusFromDetail(
		detail.PublishStatus,
		detail.Status,
		detail.SaleStatus,
		detail.DropReason,
	), nil
}

func (s *Service) checkItemStatus(ctx context.Context, id int64, price int, cookieStr string) (bool, error) {
	cacheStore := s.c
	if cacheStore == nil {
		cacheStore = cache.New(5*time.Minute, 10*time.Minute)
		s.c = cacheStore
	}

	cacheKey := fmt.Sprintf("check:%d:%d", id, price)
	if result, found := cacheStore.Get(cacheKey); found {
		return result.(bool), nil
	}

	client, err := bilihttp.NewBiliClient()
	if err != nil {
		return false, err
	}

	resp, err := client.CheckC2CItem(ctx, bilihttp.ParseBiliSession(cookieStr), id, price)
	if err != nil {
		var apiErr *bilihttp.APIError
		if errors.As(err, &apiErr) && isExpectedOrderInfoBusinessCode(apiErr.Code) {
			cacheStore.Set(cacheKey, false, cache.DefaultExpiration)
			time.Sleep(150 * time.Millisecond)
			return false, nil
		}
		return false, err
	}

	canBuy := resp.Code == 0
	cacheStore.Set(cacheKey, canBuy, cache.DefaultExpiration)
	time.Sleep(150 * time.Millisecond)
	return canBuy, nil
}

func isExpectedOrderInfoBusinessCode(code int) bool {
	return code >= 60000000 && code < 70000000
}

func calcTotalPages(total, pageSize int) int {
	if total <= 0 {
		return 1
	}
	return (total + pageSize - 1) / pageSize
}

func formatReferencePriceLabel(min, max int) string {
	if min <= 0 && max <= 0 {
		return "参考价待补充"
	}

	formatPrice := func(value int) string {
		return fmt.Sprintf("%.2f", float64(value)/100)
	}

	if min > 0 && max > 0 && min != max {
		return fmt.Sprintf("参考价 %s - %s 元", formatPrice(min), formatPrice(max))
	}

	value := max
	if min > value {
		value = min
	}
	return fmt.Sprintf("参考价 %s 元", formatPrice(value))
}

func buildItemLink(c2cItemsID int64) string {
	return fmt.Sprintf("https://mall.bilibili.com/neul-next/index.html?page=magic-market_detail&noTitleBar=1&itemsId=%d", c2cItemsID)
}
