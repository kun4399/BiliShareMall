package app

import (
	"fmt"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	"github.com/mikumifa/BiliShareMall/internal/http"
	"github.com/mikumifa/BiliShareMall/internal/util"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"runtime/debug"
	"time"
)

type C2CItemListVO struct {
	Items       []C2CItemVO `json:"items"`
	Total       int         `json:"total"`
	TotalPages  int         `json:"totalPages"`
	CurrentPage int         `json:"currentPage"`
}

type C2CItemVO struct {
	C2CItemsID      int64   `json:"c2cItemsId"`
	C2CItemsName    string  `json:"c2cItemsName"`
	TotalItemsCount int     `json:"totalItemsCount"`
	Price           float64 `json:"price"`
	ShowPrice       string  `json:"showPrice"`
}

func (a *App) ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int, used bool, cookieStr string) (ret C2CItemListVO, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Any("panic", r).Bytes("stack", debug.Stack()).Msg("panic recovered in ListC2CItem")
			ret = C2CItemListVO{}
			err = fmt.Errorf("search failed due to internal error")
		}
	}()

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	log.Info().
		Int("page", page).
		Int("pageSize", pageSize).
		Str("filterName", filterName).
		Int("sortOption", sortOption).
		Int64("startTime", startTime).
		Int64("endTime", endTime).
		Int("fromPrice", fromPrice).
		Int("toPrice", toPrice).
		Msg("Listing C2C items with parameters")

	readAndConvert := func() ([]C2CItemVO, int, error) {
		items, total, err := a.d.ReadCSCItems(page, pageSize, filterName, sortOption, util.TimestampToTime(startTime), util.TimestampToTime(endTime), fromPrice, toPrice)
		if err != nil {
			return nil, 0, err
		}
		result := make([]C2CItemVO, 0, len(items))
		for _, item := range items {
			vo := C2CItemVO{
				C2CItemsID:      item.C2CItemsID,
				C2CItemsName:    item.C2CItemsName,
				TotalItemsCount: item.TotalItemsCount,
				Price:           float64(item.Price) / 100,
				ShowPrice:       item.ShowPrice,
			}
			result = append(result, vo)
		}
		return result, total, nil
	}

	result, total, err := readAndConvert()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list items")
		return C2CItemListVO{}, err
	}

	for used && a.RemoveErrorItem(result, cookieStr) {
		result, total, err = readAndConvert()
		if err != nil {
			log.Error().Err(err).Msg("Failed to list items after removing unavailable items")
			return C2CItemListVO{}, err
		}
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return C2CItemListVO{
		Items:       result,
		Total:       total,
		TotalPages:  totalPages,
		CurrentPage: page,
	}, nil
}
func (a *App) RemoveErrorItem(items []C2CItemVO, cookieStr string) bool {
	remove := false
	for _, item := range items {
		canBuy, err := a.checkItemStatus(item.C2CItemsID, cookieStr)
		if err != nil {
			log.Printf("Failed to check item %d: %v", item.C2CItemsID, err)
			continue
		}
		if !canBuy {
			err = a.d.DeleteCSCItem(item.C2CItemsID)
			if err != nil {
				log.Printf("Failed to delete item %d: %v", item.C2CItemsID, err)
				continue
			}
			remove = true
		} else {
		}
	}

	return remove
}

func (a *App) checkItemStatus(id int64, cookiesStr string) (bool, error) {
	if result, found := a.c.Get(fmt.Sprintf("check:%d", id)); found {
		return result.(bool), nil
	}
	client, err := http.NewBiliClient()
	if err != nil {
		return false, err
	}
	client.StoreHeader("cookie", cookiesStr)
	data := map[string]interface{}{"items": map[string]any{
		"c2cItemsId": id, "price": 0,
	}}
	var resp domain.CheckResponse
	err = client.SendRequest(http.POST, "https://mall.bilibili.com/magic-c/c2c/order/info?platform=h5", data, &resp)
	if err != nil {
		return false, err
	}
	canBuy := resp.Code != 60000002
	a.c.Set(fmt.Sprintf("check:%d", id), canBuy, cache.DefaultExpiration)
	time.Sleep(1 * time.Second)
	return canBuy, nil
}
