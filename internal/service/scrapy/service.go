package scrapy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/domain"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

const (
	taskRetryDelay                = 10 * time.Second
	taskRequestInterval           = 3 * time.Second
	taskRestartRoundDelay         = 1 * time.Second
	defaultMonitorHitLimitPerRule = 20

	monitorAlertStatusSent   = "sent"
	monitorAlertStatusFailed = "failed"
)

type TaskRuntime struct {
	taskID          int
	accountID       int64
	cookies         string
	requestInterval time.Duration
	cancel          context.CancelFunc
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
	SkuName  string `json:"skuName"`
	MinPrice int    `json:"minPrice"`
	MaxPrice int    `json:"maxPrice"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

type MonitorConfig struct {
	Webhook string        `json:"webhook"`
	Rules   []MonitorRule `json:"rules"`
}

type MonitorHitItem struct {
	RuleID       int64  `json:"ruleId"`
	TaskID       int    `json:"taskId"`
	C2CItemsID   int64  `json:"c2cItemsId"`
	SkuID        int64  `json:"skuId"`
	ItemName     string `json:"itemName"`
	Price        int    `json:"price"`
	ShowPrice    string `json:"showPrice"`
	ItemLink     string `json:"itemLink"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
	OccurredAt   int64  `json:"occurredAt"`
}

type MonitorHitGroup struct {
	RuleID int64            `json:"ruleId"`
	Hits   []MonitorHitItem `json:"hits"`
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
	// Creation flow keeps runtime settings at defaults; users can customize later per task.
	item.AccountID = 0
	if item.RequestIntervalSec <= 0 {
		item.RequestIntervalSec = 3
	}
	id, err := s.d.CreateScrapyItem(item)
	if err != nil {
		log.Error().Err(err).Msg("error creating scrapy item")
		return id
	}
	return id
}

func (s *Service) StartTask(taskID int, cookies string) error {
	scrapyItem, err := s.d.ReadScrapyItem(taskID)
	if err != nil {
		return err
	}

	resolvedCookies := strings.TrimSpace(cookies)
	if scrapyItem.AccountID > 0 {
		account, accountErr := s.d.GetAuthAccountByID(scrapyItem.AccountID)
		if accountErr != nil {
			return fmt.Errorf("bound account not found, please re-select account")
		}
		resolvedCookies = strings.TrimSpace(account.Cookies)
	}
	if resolvedCookies == "" {
		return fmt.Errorf("missing login cookies")
	}
	requestInterval := s.resolveTaskRequestInterval(scrapyItem.RequestIntervalSec)

	s.mu.Lock()
	if running := s.runningTasks[taskID]; running != nil && running.cancel != nil {
		s.mu.Unlock()
		return fmt.Errorf("task already running")
	}

	// Every manual start should begin from a clean counter state.
	scrapyItem.Nums = 0
	scrapyItem.IncreaseNumber = 0
	scrapyItem.NextToken = nil
	if _, err := s.d.UpdateScrapyItem(&scrapyItem); err != nil {
		s.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasks[taskID] = &TaskRuntime{
		taskID:          taskID,
		accountID:       scrapyItem.AccountID,
		cookies:         resolvedCookies,
		requestInterval: requestInterval,
		cancel:          cancel,
	}
	s.mu.Unlock()

	s.emitEvent("updateScrapyItem", scrapyItem)

	s.wg.Add(1)
	go s.scrapyLoop(taskID, resolvedCookies, requestInterval, ctx)
	return nil
}

func (s *Service) UpdateScrapyTaskConfig(taskID int, accountID int64, requestIntervalSeconds float64) error {
	if requestIntervalSeconds < 0 {
		return fmt.Errorf("request interval must be >= 0")
	}
	if s.isTaskRunning(taskID) {
		return fmt.Errorf("task is running, stop it first")
	}
	if accountID > 0 {
		if _, err := s.d.GetAuthAccountByID(accountID); err != nil {
			return fmt.Errorf("account not found")
		}
	}
	if err := s.d.UpdateScrapyTaskConfig(taskID, accountID, requestIntervalSeconds); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("task not found")
		}
		return err
	}
	item, err := s.d.ReadScrapyItem(taskID)
	if err == nil {
		s.emitEvent("updateScrapyItem", item)
	}
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

func (s *Service) HasRunningTasks() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, runtime := range s.runningTasks {
		if runtime != nil && runtime.cancel != nil {
			return true
		}
	}
	return false
}

func (s *Service) IsAnyTaskRunningWithAccount(accountID int64) bool {
	if accountID <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, runtime := range s.runningTasks {
		if runtime != nil && runtime.cancel != nil && runtime.accountID == accountID {
			return true
		}
	}
	return false
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

func (s *Service) ListMonitorRuleHits(limitPerRule int) []MonitorHitGroup {
	if limitPerRule <= 0 {
		limitPerRule = defaultMonitorHitLimitPerRule
	}

	config, err := s.d.GetMonitorConfig()
	if err != nil {
		log.Error().Err(err).Msg("failed to read monitor config for hits")
		return []MonitorHitGroup{}
	}

	groups := make([]MonitorHitGroup, 0, len(config.Rules))
	for _, rule := range config.Rules {
		events, readErr := s.d.ReadMonitorAlertEventsByRule(rule.ID, limitPerRule)
		if readErr != nil {
			log.Error().Err(readErr).Int64("ruleID", rule.ID).Msg("failed to read monitor hits")
			groups = append(groups, MonitorHitGroup{
				RuleID: rule.ID,
				Hits:   []MonitorHitItem{},
			})
			continue
		}
		hits := make([]MonitorHitItem, 0, len(events))
		for _, event := range events {
			hits = append(hits, toMonitorHitItem(event))
		}
		groups = append(groups, MonitorHitGroup{
			RuleID: rule.ID,
			Hits:   hits,
		})
	}
	return groups
}

func (s *Service) scrapyLoop(taskID int, cookies string, requestInterval time.Duration, ctx context.Context) {
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
			scrapyItem.IncreaseNumber++
			if _, err := s.d.UpdateScrapyItem(&scrapyItem); err != nil {
				log.Error().Err(err).Int("taskID", taskID).Msg("failed to persist scrapy round count")
				s.emitEvent("scrapy_failed", taskID)
				return
			}
			s.emitEvent("updateScrapyItem", scrapyItem)
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

		if !sleepWithContext(ctx, requestInterval) {
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
		candidateDetails := pickMonitorCandidates(item)
		if len(candidateDetails) == 0 {
			continue
		}
		seenRules := make(map[int64]struct{})
		for _, candidate := range candidateDetails {
			candidates := rulesBySku[candidate.SkuID]
			if len(candidates) == 0 {
				continue
			}
			for _, rule := range candidates {
				if _, duplicated := seenRules[rule.ID]; duplicated {
					continue
				}
				seenRules[rule.ID] = struct{}{}

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

				name := pickItemName(candidate.Name, item.C2CItemsName)
				displayPrice := normalizeDisplayPrice(item.ShowPrice, item.Price)
				itemLink := buildItemLink(item.C2CItemsID)
				text := buildDingTalkMarkdown(name, displayPrice, itemLink)
				sendErr := s.notifier.SendMarkdown(context.Background(), webhook, "市集助手", text)
				if sendErr != nil {
					_ = s.d.ReleaseMonitorAlertReservation(rule.ID, item.C2CItemsID)
					s.recordAndEmitMonitorHit(dao.MonitorAlertEvent{
						RuleID:       rule.ID,
						C2CItemsID:   item.C2CItemsID,
						TaskID:       taskID,
						SkuID:        candidate.SkuID,
						ItemName:     name,
						Price:        item.Price,
						ShowPrice:    displayPrice,
						ItemLink:     itemLink,
						Status:       monitorAlertStatusFailed,
						ErrorMessage: sendErr.Error(),
					})
					log.Error().Err(sendErr).Int64("ruleID", rule.ID).Int64("itemID", item.C2CItemsID).Msg("send dingtalk alert failed")
					continue
				}
				if err := s.d.MarkMonitorAlertSent(rule.ID, item.C2CItemsID, time.Now()); err != nil {
					log.Error().Err(err).Int64("ruleID", rule.ID).Int64("itemID", item.C2CItemsID).Msg("mark monitor alert sent failed")
				}
				s.recordAndEmitMonitorHit(dao.MonitorAlertEvent{
					RuleID:       rule.ID,
					C2CItemsID:   item.C2CItemsID,
					TaskID:       taskID,
					SkuID:        candidate.SkuID,
					ItemName:     name,
					Price:        item.Price,
					ShowPrice:    displayPrice,
					ItemLink:     itemLink,
					Status:       monitorAlertStatusSent,
					ErrorMessage: "",
				})
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
			SkuName:  rule.SkuName,
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

func normalizeIntervalSeconds(seconds float64) float64 {
	if seconds < 0 {
		return 3
	}
	return seconds
}

func (s *Service) resolveTaskRequestInterval(seconds float64) time.Duration {
	if seconds == 0 {
		return 0
	}
	if seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	if s.requestInterval > 0 {
		return s.requestInterval
	}
	return time.Duration(normalizeIntervalSeconds(seconds) * float64(time.Second))
}

func (s *Service) recordAndEmitMonitorHit(event dao.MonitorAlertEvent) {
	if event.Status == "" {
		event.Status = monitorAlertStatusFailed
	}
	if err := s.d.CreateMonitorAlertEvent(event); err != nil {
		log.Error().Err(err).
			Int64("ruleID", event.RuleID).
			Int64("itemID", event.C2CItemsID).
			Str("status", event.Status).
			Msg("create monitor alert event failed")
	}
	s.emitEvent("monitor_alert_result", toMonitorHitItem(event))
}

func toMonitorHitItem(event dao.MonitorAlertEvent) MonitorHitItem {
	occurredAt := event.OccurredAt
	if occurredAt == 0 {
		occurredAt = time.Now().UnixMilli()
	}
	return MonitorHitItem{
		RuleID:       event.RuleID,
		TaskID:       event.TaskID,
		C2CItemsID:   event.C2CItemsID,
		SkuID:        event.SkuID,
		ItemName:     strings.TrimSpace(event.ItemName),
		Price:        event.Price,
		ShowPrice:    strings.TrimSpace(event.ShowPrice),
		ItemLink:     strings.TrimSpace(event.ItemLink),
		Status:       strings.TrimSpace(event.Status),
		ErrorMessage: strings.TrimSpace(event.ErrorMessage),
		OccurredAt:   occurredAt,
	}
}

type monitorCandidate struct {
	SkuID int64
	Name  string
}

func pickMonitorCandidates(item domain.MarketItem) []monitorCandidate {
	if len(item.DetailDtoList) == 0 {
		return []monitorCandidate{}
	}
	candidates := make([]monitorCandidate, 0, len(item.DetailDtoList))
	seenSkuIDs := map[int64]struct{}{}
	for _, detail := range item.DetailDtoList {
		skuID := int64(detail.SkuID)
		if skuID <= 0 {
			continue
		}
		if _, exists := seenSkuIDs[skuID]; exists {
			continue
		}
		seenSkuIDs[skuID] = struct{}{}
		candidates = append(candidates, monitorCandidate{
			SkuID: skuID,
			Name:  strings.TrimSpace(detail.Name),
		})
	}
	return candidates
}

func pickItemName(detailName, itemName string) string {
	name := strings.TrimSpace(detailName)
	if name != "" {
		return name
	}
	name = strings.TrimSpace(itemName)
	if name == "" {
		return "未知商品"
	}
	return name
}

func normalizeDisplayPrice(showPrice string, price int) string {
	displayPrice := strings.TrimSpace(showPrice)
	if displayPrice != "" {
		return displayPrice
	}
	return fmt.Sprintf("%.2f", float64(price)/100)
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
