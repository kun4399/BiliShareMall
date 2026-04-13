package scrapy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	bilihttp "github.com/mikumifa/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

type TaskRequest struct {
	taskID  int
	cookies string
	cancel  context.CancelFunc
}

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

type C2CItemsChangedEvent struct {
	TaskID      int   `json:"taskId"`
	ChangedRows int64 `json:"changedRows"`
	EmittedAt   int64 `json:"emittedAt"`
}

type EventEmitter func(eventName string, payload any)

type Service struct {
	d    *dao.Database
	emit EventEmitter

	wg sync.WaitGroup
	mu sync.Mutex

	nowRunTask TaskRequest
}

func NewService(database *dao.Database, emit EventEmitter) *Service {
	return &Service{
		d:    database,
		emit: emit,
	}
}

func (s *Service) ReadAllScrapyItems() []dao.ScrapyItem {
	items, err := s.d.ReadAllScrapyItems()
	if err != nil {
		log.Error().Err(err).Msg("error reading scrapy items")
		return []dao.ScrapyItem{}
	}
	return items
}

func (s *Service) DeleteScrapyItem(id int) error {
	if err := s.d.DeleteScrapyItem(id); err != nil {
		log.Error().Err(err).Msg("error deleting scrapy item")
		return err
	}
	return nil
}

func (s *Service) CreateScrapyItem(item dao.ScrapyItem) int64 {
	item.CreateTime = time.Now()
	id, err := s.d.CreateScrapyItem(item)
	if err != nil {
		log.Error().Err(err).Msg("error creating scrapy item")
		return id
	}
	return id
}

func (s *Service) StartTask(taskID int, cookies string) error {
	var cancel context.CancelFunc

	s.mu.Lock()
	if s.nowRunTask.cancel != nil {
		cancel = s.nowRunTask.cancel
	}
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()

	ctx, newCancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.nowRunTask = TaskRequest{taskID: taskID, cookies: cookies, cancel: newCancel}
	s.mu.Unlock()

	s.wg.Add(1)
	go s.scrapyLoop(taskID, ctx)
	return nil
}

func (s *Service) DoneTask(taskID int) error {
	s.mu.Lock()
	task := s.nowRunTask
	s.mu.Unlock()

	if taskID != task.taskID {
		return fmt.Errorf("task not running")
	}
	if task.cancel == nil {
		return fmt.Errorf("task not running")
	}
	task.cancel()
	return nil
}

func (s *Service) GetNowRunTaskId() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nowRunTask.taskID
}

func (s *Service) GetMarketRuntimeConfig(cookieStr string) MarketRuntimeConfig {
	client, err := bilihttp.NewBiliClient()
	if err != nil {
		log.Error().Err(err).Msg("failed to create market client")
		return toRuntimeConfig(bilihttp.DefaultMarketRuntimeConfig())
	}

	config, err := client.GetMarketRuntimeConfig(context.Background(), bilihttp.ParseBiliSession(cookieStr))
	if err != nil {
		log.Warn().Err(err).Msg("failed to load remote market config, using fallback")
		fallback := bilihttp.DefaultMarketRuntimeConfig()
		fallback.Message = err.Error()
		return toRuntimeConfig(fallback)
	}
	return toRuntimeConfig(config)
}

func (s *Service) scrapyLoop(taskID int, ctx context.Context) {
	defer s.wg.Done()

	scrapyItem, err := s.d.ReadScrapyItem(taskID)
	if err != nil {
		s.emitEvent("scrapyItem_get_failed", scrapyItem.Id)
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Info().Any("scrapyItem", scrapyItem).Msg("scrapy task canceled")
			return
		default:
			if err := s.scrapyTask(taskID, &scrapyItem); err != nil {
				log.Error().Err(err).Msg("scrapy task failed")
				s.emitEvent("scrapy_failed", scrapyItem.Id)
				return
			}
			if scrapyItem.NextToken == nil {
				s.emitEvent("scrapy_finished", scrapyItem.Id)
				return
			}
			time.Sleep(3 * time.Second)
		}
	}
}

func (s *Service) scrapyTask(taskID int, item *dao.ScrapyItem) error {
	client, err := bilihttp.NewBiliClient()
	if err != nil {
		return err
	}

	s.mu.Lock()
	cookies := s.nowRunTask.cookies
	s.mu.Unlock()

	session := bilihttp.ParseBiliSession(cookies)
	resp, err := client.ListMarketItems(context.Background(), session, bilihttp.MarketListRequest{
		SortType:        item.Order,
		NextID:          item.NextToken,
		PriceFilters:    []string{item.PriceFilter},
		DiscountFilters: []string{item.DiscountFilter},
		CategoryFilter:  item.Product,
	})
	if err != nil {
		if bilihttp.IsAPIErrorKind(err, bilihttp.ErrKindRateLimited) {
			s.emitEvent("scrapy_wait", 5)
			time.Sleep(5 * time.Second)
			return nil
		}
		return err
	}

	item.NextToken = resp.Data.NextID
	item.Nums++
	changedRows := s.d.SaveMailListToDB(&resp)
	item.IncreaseNumber += int(changedRows)
	if _, err = s.d.UpdateScrapyItem(item); err != nil {
		return err
	}

	s.emitEvent("updateScrapyItem", item)
	if changedRows > 0 {
		s.emitEvent("c2c_items_changed", C2CItemsChangedEvent{
			TaskID:      taskID,
			ChangedRows: changedRows,
			EmittedAt:   time.Now().UnixMilli(),
		})
	}
	return nil
}

func (s *Service) emitEvent(eventName string, payload any) {
	if s.emit == nil {
		return
	}
	s.emit(eventName, payload)
}

func toRuntimeConfig(config domain.MarketRuntimeConfig) MarketRuntimeConfig {
	return MarketRuntimeConfig{
		Categories:      toRuntimeOptions(config.Categories),
		Sorts:           toRuntimeOptions(config.Sorts),
		PriceFilters:    toRuntimeOptions(config.PriceFilters),
		DiscountFilters: toRuntimeOptions(config.DiscountFilters),
		Source:          config.Source,
		Message:         config.Message,
	}
}

func toRuntimeOptions(options []domain.MarketFilterOption) []MarketFilterOption {
	result := make([]MarketFilterOption, 0, len(options))
	for _, option := range options {
		result = append(result, MarketFilterOption{
			Label: option.Label,
			Value: option.Value,
		})
	}
	return result
}
