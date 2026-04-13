package scrapy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	bilihttp "github.com/mikumifa/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

const (
	taskRetryDelay        = 10 * time.Second
	taskRequestInterval   = 3 * time.Second
	taskRestartRoundDelay = 1 * time.Second
)

type TaskRuntime struct {
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

type MonitorRule struct {
	ID       int64  `json:"id"`
	SkuID    int64  `json:"skuId"`
	MinPrice int    `json:"minPrice"`
	MaxPrice int    `json:"maxPrice"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

type MonitorConfig struct {
	Webhook string        `json:"webhook"`
	Rules   []MonitorRule `json:"rules"`
}

type C2CItemsChangedEvent struct {
	TaskID      int   `json:"taskId"`
	ChangedRows int64 `json:"changedRows"`
	EmittedAt   int64 `json:"emittedAt"`
}

type ScrapyRetryEvent struct {
	TaskID  int    `json:"taskId"`
	Seconds int    `json:"seconds"`
	Reason  string `json:"reason"`
}

type ScrapyRoundEvent struct {
	TaskID      int   `json:"taskId"`
	CompletedAt int64 `json:"completedAt"`
}

type EventEmitter func(eventName string, payload any)

type marketClient interface {
	ListMarketItems(ctx context.Context, session *bilihttp.BiliSession, req bilihttp.MarketListRequest) (domain.MailListResponse, error)
}

type Service struct {
	d                 *dao.Database
	emit              EventEmitter
	notifier          DingTalkNotifier
	marketFn          func() (marketClient, error)
	retryDelay        time.Duration
	requestInterval   time.Duration
	restartRoundDelay time.Duration

	wg sync.WaitGroup
	mu sync.Mutex

	runningTasks map[int]*TaskRuntime
}

func NewService(database *dao.Database, emit EventEmitter) *Service {
	return &Service{
		d:        database,
		emit:     emit,
		notifier: NewHTTPDingTalkNotifier(),
		marketFn: func() (marketClient, error) {
			return bilihttp.NewBiliClient()
		},
		retryDelay:        taskRetryDelay,
		requestInterval:   taskRequestInterval,
		restartRoundDelay: taskRestartRoundDelay,
		runningTasks:      map[int]*TaskRuntime{},
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
	if s.isTaskRunning(id) {
		return fmt.Errorf("task is running, stop it first")
	}
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
	if _, err := s.d.ReadScrapyItem(taskID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if running := s.runningTasks[taskID]; running != nil && running.cancel != nil {
		return fmt.Errorf("task already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasks[taskID] = &TaskRuntime{
		taskID:  taskID,
		cookies: cookies,
		cancel:  cancel,
	}

	s.wg.Add(1)
	go s.scrapyLoop(taskID, cookies, ctx)
	return nil
}

func (s *Service) DoneTask(taskID int) error {
	s.mu.Lock()
	runtime := s.runningTasks[taskID]
	s.mu.Unlock()

	if runtime == nil || runtime.cancel == nil {
		return fmt.Errorf("task not running")
	}
	runtime.cancel()
	return nil
}

func (s *Service) GetNowRunTaskId() int {
	ids := s.GetRunningTaskIds()
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func (s *Service) GetRunningTaskIds() []int {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]int, 0, len(s.runningTasks))
	for taskID, runtime := range s.runningTasks {
		if runtime != nil && runtime.cancel != nil {
			ids = append(ids, taskID)
		}
	}
	sort.Ints(ids)
	return ids
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

func (s *Service) GetMonitorConfig() MonitorConfig {
	config, err := s.d.GetMonitorConfig()
	if err != nil {
		log.Error().Err(err).Msg("failed to read monitor config")
		return MonitorConfig{
			Webhook: "",
			Rules:   []MonitorRule{},
		}
	}
	return toMonitorConfig(config)
}

func (s *Service) SaveMonitorConfig(config MonitorConfig) error {
	webhook := strings.TrimSpace(config.Webhook)
	if len(config.Rules) > 0 && webhook == "" {
		return fmt.Errorf("webhook is required when monitor rules are configured")
	}
	for _, rule := range config.Rules {
		rule.Remark = strings.TrimSpace(rule.Remark)
		if rule.SkuID <= 0 {
			return fmt.Errorf("invalid skuId: %d", rule.SkuID)
		}
		if rule.MinPrice < 0 || rule.MaxPrice < 0 {
			return fmt.Errorf("price cannot be negative")
		}
		if rule.MinPrice > rule.MaxPrice {
			return fmt.Errorf("minPrice cannot be greater than maxPrice")
		}
	}
	config.Webhook = webhook
	return s.d.SaveMonitorConfig(toDAOMonitorConfig(config))
}

func (s *Service) scrapyLoop(taskID int, cookies string, ctx context.Context) {
	defer s.wg.Done()
	defer s.unregisterTask(taskID)

	scrapyItem, err := s.d.ReadScrapyItem(taskID)
	if err != nil {
		s.emitEvent("scrapyItem_get_failed", taskID)
		return
	}

	scrapyItem.NextToken = normalizeNextToken(scrapyItem.NextToken)
	for {
		if ctx.Err() != nil {
			log.Info().Int("taskID", taskID).Msg("scrapy task canceled")
			return
		}

		roundFinished, err := s.scrapyTask(taskID, cookies, &scrapyItem)
		if err != nil {
			var retryErr *requestRetryableError
			if errors.As(err, &retryErr) {
				s.emitEvent("scrapy_retry_wait", ScrapyRetryEvent{
					TaskID:  taskID,
					Seconds: int(s.retryDelay.Seconds()),
					Reason:  retryErr.Error(),
				})
				s.emitEvent("scrapy_wait", int(s.retryDelay.Seconds()))
				if !sleepWithContext(ctx, s.retryDelay) {
					return
				}
				continue
			}

			log.Error().Err(err).Int("taskID", taskID).Msg("scrapy task failed")
			s.emitEvent("scrapy_failed", taskID)
			return
		}

		if roundFinished {
			s.emitEvent("scrapy_round_finished", ScrapyRoundEvent{
				TaskID:      taskID,
				CompletedAt: time.Now().UnixMilli(),
			})
			// Keep backward compatibility for existing listeners.
			s.emitEvent("scrapy_finished", taskID)
			scrapyItem.NextToken = nil
			if !sleepWithContext(ctx, s.restartRoundDelay) {
				return
			}
			continue
		}

		if !sleepWithContext(ctx, s.requestInterval) {
			return
		}
	}
}

func (s *Service) scrapyTask(taskID int, cookies string, item *dao.ScrapyItem) (bool, error) {
	client, err := s.marketFn()
	if err != nil {
		return false, err
	}

	session := bilihttp.ParseBiliSession(cookies)
	resp, err := client.ListMarketItems(context.Background(), session, bilihttp.MarketListRequest{
		SortType:        item.Order,
		NextID:          normalizeNextToken(item.NextToken),
		PriceFilters:    []string{item.PriceFilter},
		DiscountFilters: []string{item.DiscountFilter},
		CategoryFilter:  item.Product,
	})
	if err != nil {
		return false, &requestRetryableError{cause: err}
	}

	item.NextToken = normalizeNextToken(resp.Data.NextID)
	item.Nums++
	changedRows, err := s.d.SaveMailListToDBStrict(&resp)
	if err != nil {
		return false, err
	}
	item.IncreaseNumber += int(changedRows)
	if _, err = s.d.UpdateScrapyItem(item); err != nil {
		return false, err
	}

	s.emitEvent("updateScrapyItem", item)
	if changedRows > 0 {
		s.emitEvent("c2c_items_changed", C2CItemsChangedEvent{
			TaskID:      taskID,
			ChangedRows: changedRows,
			EmittedAt:   time.Now().UnixMilli(),
		})
	}

	s.trySendMonitorAlerts(taskID, resp.Data.Data)
	return item.NextToken == nil, nil
}

func (s *Service) trySendMonitorAlerts(taskID int, items []domain.MarketItem) {
	if len(items) == 0 {
		return
	}
	webhook, err := s.d.ReadMonitorWebhook()
	if err != nil {
		log.Error().Err(err).Msg("failed to read monitor webhook")
		return
	}
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return
	}

	rules, err := s.d.ReadEnabledMonitorRules()
	if err != nil {
		log.Error().Err(err).Msg("failed to read monitor rules")
		return
	}
	if len(rules) == 0 {
		return
	}

	rulesBySku := make(map[int64][]dao.MonitorRule)
	for _, rule := range rules {
		rulesBySku[rule.SkuID] = append(rulesBySku[rule.SkuID], rule)
	}

	for _, item := range items {
		name, skuID := pickMonitorNameAndSku(item)
		if skuID == 0 {
			continue
		}
		candidates := rulesBySku[skuID]
		if len(candidates) == 0 {
			continue
		}
		for _, rule := range candidates {
			if item.Price < rule.MinPrice || item.Price > rule.MaxPrice {
				continue
			}

			reserved, reserveErr := s.d.ReserveMonitorAlert(rule.ID, item.C2CItemsID, taskID)
			if reserveErr != nil {
				log.Error().Err(reserveErr).Int64("ruleID", rule.ID).Int64("itemID", item.C2CItemsID).Msg("reserve monitor alert failed")
				continue
			}
			if !reserved {
				continue
			}

			if name == "" {
				name = strings.TrimSpace(item.C2CItemsName)
			}
			displayPrice := strings.TrimSpace(item.ShowPrice)
			if displayPrice == "" {
				displayPrice = fmt.Sprintf("%.2f", float64(item.Price)/100)
			}
			text := buildDingTalkMarkdown(name, displayPrice, buildItemLink(item.C2CItemsID))
			sendErr := s.notifier.SendMarkdown(context.Background(), webhook, "市集助手", text)
			if sendErr != nil {
				_ = s.d.ReleaseMonitorAlertReservation(rule.ID, item.C2CItemsID)
				log.Error().Err(sendErr).Int64("ruleID", rule.ID).Int64("itemID", item.C2CItemsID).Msg("send dingtalk alert failed")
				continue
			}
			if err := s.d.MarkMonitorAlertSent(rule.ID, item.C2CItemsID, time.Now()); err != nil {
				log.Error().Err(err).Int64("ruleID", rule.ID).Int64("itemID", item.C2CItemsID).Msg("mark monitor alert sent failed")
			}
		}
	}
}

func (s *Service) emitEvent(eventName string, payload any) {
	if s.emit == nil {
		return
	}
	s.emit(eventName, payload)
}

func (s *Service) unregisterTask(taskID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runningTasks, taskID)
}

func (s *Service) isTaskRunning(taskID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime := s.runningTasks[taskID]
	return runtime != nil && runtime.cancel != nil
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

func toMonitorConfig(config dao.MonitorConfig) MonitorConfig {
	rules := make([]MonitorRule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		rules = append(rules, MonitorRule{
			ID:       rule.ID,
			SkuID:    rule.SkuID,
			MinPrice: rule.MinPrice,
			MaxPrice: rule.MaxPrice,
			Enabled:  rule.Enabled,
			Remark:   rule.Remark,
		})
	}
	return MonitorConfig{
		Webhook: config.Webhook,
		Rules:   rules,
	}
}

func toDAOMonitorConfig(config MonitorConfig) dao.MonitorConfig {
	rules := make([]dao.MonitorRule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		rules = append(rules, dao.MonitorRule{
			ID:       rule.ID,
			SkuID:    rule.SkuID,
			MinPrice: rule.MinPrice,
			MaxPrice: rule.MaxPrice,
			Enabled:  rule.Enabled,
			Remark:   strings.TrimSpace(rule.Remark),
		})
	}
	return dao.MonitorConfig{
		Webhook: config.Webhook,
		Rules:   rules,
	}
}

func normalizeNextToken(token *string) *string {
	if token == nil {
		return nil
	}
	value := strings.TrimSpace(*token)
	if value == "" {
		return nil
	}
	return &value
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func pickMonitorNameAndSku(item domain.MarketItem) (string, int64) {
	if len(item.DetailDtoList) > 0 {
		detail := item.DetailDtoList[0]
		return strings.TrimSpace(detail.Name), int64(detail.SkuID)
	}
	return strings.TrimSpace(item.C2CItemsName), 0
}

func buildItemLink(c2cItemsID int64) string {
	return fmt.Sprintf("https://mall.bilibili.com/neul-next/index.html?page=magic-market_detail&noTitleBar=1&itemsId=%d", c2cItemsID)
}

type requestRetryableError struct {
	cause error
}

func (e *requestRetryableError) Error() string {
	if e == nil || e.cause == nil {
		return "request failed"
	}
	return e.cause.Error()
}

func (e *requestRetryableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
