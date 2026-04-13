package app

import (
	"github.com/kun4399/BiliShareMall/internal/dao"
	scrapysvc "github.com/kun4399/BiliShareMall/internal/service/scrapy"
)

type MarketFilterOption = scrapysvc.MarketFilterOption
type MarketRuntimeConfig = scrapysvc.MarketRuntimeConfig
type MonitorRule = scrapysvc.MonitorRule
type MonitorConfig = scrapysvc.MonitorConfig
type MonitorHitItem = scrapysvc.MonitorHitItem
type MonitorHitGroup = scrapysvc.MonitorHitGroup

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

func (a *App) GetRunningTaskIds() []int {
	return a.getScrapyService().GetRunningTaskIds()
}

func (a *App) GetMarketRuntimeConfig(cookieStr string) MarketRuntimeConfig {
	return a.getScrapyService().GetMarketRuntimeConfig(cookieStr)
}

func (a *App) GetMonitorConfig() MonitorConfig {
	return a.getScrapyService().GetMonitorConfig()
}

func (a *App) SaveMonitorConfig(config MonitorConfig) error {
	return a.getScrapyService().SaveMonitorConfig(config)
}

func (a *App) ListMonitorRuleHits(limitPerRule int) []MonitorHitGroup {
	return a.getScrapyService().ListMonitorRuleHits(limitPerRule)
}
