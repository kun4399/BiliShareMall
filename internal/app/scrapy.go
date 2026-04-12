package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	bilihttp "github.com/mikumifa/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type TaskRequest struct {
	taskId  int
	cookies string
	cancel  context.CancelFunc
}

var wg sync.WaitGroup
var nowRunTask TaskRequest

type MarketFilterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type MarketRuntimeConfig struct {
	Categories      []MarketFilterOption `json:"categories"`
	Sorts           []MarketFilterOption `json:"sorts"`
	PriceFilters    []MarketFilterOption `json:"priceFilters"`
	DiscountFilters []MarketFilterOption `json:"discountFilters"`
	Source          string               `json:"source"`
	Message         string               `json:"message"`
}

func (a *App) ReadAllScrapyItems() []dao.ScrapyItem {
	items, err := a.d.ReadAllScrapyItems()
	if err != nil {
		log.Error().Err(err).Msg("error reading scrapy items")
		return []dao.ScrapyItem{}
	}
	return items
}

func (a *App) DeleteScrapyItem(id int) error {
	if err := a.d.DeleteScrapyItem(id); err != nil {
		log.Error().Err(err).Msg("error deleting scrapy item")
		return err
	}
	return nil
}

func (a *App) CreateScrapyItem(item dao.ScrapyItem) int64 {
	item.CreateTime = time.Now()
	id, err := a.d.CreateScrapyItem(item)
	if err != nil {
		log.Error().Err(err).Msg("error creating scrapy item")
		return id
	}
	return id
}

func (a *App) scrapyLoop(taskID int, ctx context.Context) {
	defer wg.Done()

	scrapyItem, err := a.d.ReadScrapyItem(taskID)
	if err != nil {
		runtime.EventsEmit(a.ctx, "scrapyItem_get_failed", scrapyItem.Id)
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Info().Any("scrapyItem", scrapyItem).Msg("scrapy task canceled")
			return
		default:
			nowRunTask.taskId = taskID
			err := a.scrapyTask(&scrapyItem)
			if err != nil {
				log.Error().Err(err).Msg("scrapy task failed")
				runtime.EventsEmit(a.ctx, "scrapy_failed", scrapyItem.Id)
				return
			}
			if scrapyItem.NextToken == nil {
				runtime.EventsEmit(a.ctx, "scrapy_finished", scrapyItem.Id)
				return
			}
			time.Sleep(3 * time.Second)
		}
	}
}

func (a *App) StartTask(taskID int, cookies string) error {
	if nowRunTask.cancel != nil {
		nowRunTask.cancel()
	}
	wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	nowRunTask = TaskRequest{taskId: taskID, cookies: cookies, cancel: cancel}
	wg.Add(1)
	go a.scrapyLoop(taskID, ctx)
	return nil
}

func (a *App) DoneTask(taskID int) error {
	if taskID != nowRunTask.taskId {
		return fmt.Errorf("task not running")
	}
	nowRunTask.cancel()
	return nil
}

func (a *App) GetNowRunTaskId() int {
	return nowRunTask.taskId
}

func (a *App) GetMarketRuntimeConfig(cookieStr string) MarketRuntimeConfig {
	client, err := bilihttp.NewBiliClient()
	if err != nil {
		log.Error().Err(err).Msg("failed to create market client")
		return toAppMarketRuntimeConfig(bilihttp.DefaultMarketRuntimeConfig())
	}
	config, err := client.GetMarketRuntimeConfig(context.Background(), bilihttp.ParseBiliSession(cookieStr))
	if err != nil {
		log.Warn().Err(err).Msg("failed to load remote market config, using fallback")
		fallback := bilihttp.DefaultMarketRuntimeConfig()
		fallback.Message = err.Error()
		return toAppMarketRuntimeConfig(fallback)
	}
	return toAppMarketRuntimeConfig(config)
}

func (a *App) scrapyTask(item *dao.ScrapyItem) error {
	client, err := bilihttp.NewBiliClient()
	if err != nil {
		return err
	}

	session := bilihttp.ParseBiliSession(nowRunTask.cookies)
	resp, err := client.ListMarketItems(context.Background(), session, bilihttp.MarketListRequest{
		SortType:        item.Order,
		NextID:          item.NextToken,
		PriceFilters:    []string{item.PriceFilter},
		DiscountFilters: []string{item.DiscountFilter},
		CategoryFilter:  item.Product,
	})
	if err != nil {
		if bilihttp.IsAPIErrorKind(err, bilihttp.ErrKindRateLimited) {
			runtime.EventsEmit(a.ctx, "scrapy_wait", 5)
			time.Sleep(5 * time.Second)
			return nil
		}
		return err
	}

	item.NextToken = resp.Data.NextID
	item.Nums++
	item.IncreaseNumber += int(a.d.SaveMailListToDB(&resp))
	if _, err = a.d.UpdateScrapyItem(item); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "updateScrapyItem", item)
	return nil
}

func toAppMarketRuntimeConfig(config domain.MarketRuntimeConfig) MarketRuntimeConfig {
	return MarketRuntimeConfig{
		Categories:      toAppFilterOptions(config.Categories),
		Sorts:           toAppFilterOptions(config.Sorts),
		PriceFilters:    toAppFilterOptions(config.PriceFilters),
		DiscountFilters: toAppFilterOptions(config.DiscountFilters),
		Source:          config.Source,
		Message:         config.Message,
	}
}

func toAppFilterOptions(options []domain.MarketFilterOption) []MarketFilterOption {
	result := make([]MarketFilterOption, 0, len(options))
	for _, option := range options {
		result = append(result, MarketFilterOption{
			Label: option.Label,
			Value: option.Value,
		})
	}
	return result
}
