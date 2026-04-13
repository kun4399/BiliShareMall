package app

import (
	"github.com/mikumifa/BiliShareMall/internal/dao"
	scrapysvc "github.com/mikumifa/BiliShareMall/internal/service/scrapy"
)

type MarketFilterOption = scrapysvc.MarketFilterOption
type MarketRuntimeConfig = scrapysvc.MarketRuntimeConfig

func (a *App) ReadAllScrapyItems() []dao.ScrapyItem {
	return a.getScrapyService().ReadAllScrapyItems()
}

func (a *App) DeleteScrapyItem(id int) error {
	return a.getScrapyService().DeleteScrapyItem(id)
}

func (a *App) CreateScrapyItem(item dao.ScrapyItem) int64 {
	return a.getScrapyService().CreateScrapyItem(item)
}

func (a *App) StartTask(taskID int, cookies string) error {
	return a.getScrapyService().StartTask(taskID, cookies)
}

func (a *App) DoneTask(taskID int) error {
	return a.getScrapyService().DoneTask(taskID)
}

func (a *App) GetNowRunTaskId() int {
	return a.getScrapyService().GetNowRunTaskId()
}

func (a *App) GetMarketRuntimeConfig(cookieStr string) MarketRuntimeConfig {
	return a.getScrapyService().GetMarketRuntimeConfig(cookieStr)
}
