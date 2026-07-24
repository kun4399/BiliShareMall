package scrapy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/domain"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

const (
	taskRetryDelay                = 10 * time.Second
	taskRequestInterval           = 12 * time.Second
	minimumTaskRequestInterval    = 12 * time.Second
	rateLimitRetryDelay           = 60 * time.Second
	maximumRateLimitRetryDelay    = 15 * time.Minute
	taskRestartRoundDelay         = 1 * time.Second
	taskStopTimeout               = 10 * time.Second
	monitorNotificationTimeout    = 8 * time.Second
	defaultMonitorHitLimitPerRule = 20

	monitorAlertStatusSent   = "sent"
	monitorAlertStatusFailed = "failed"
)

type TaskRuntime struct {
	taskID          int
	runID           uint64
	accountID       int64
	cookies         string
	requestInterval time.Duration
	cancel          context.CancelFunc
	done            chan struct{}
	stopping        bool
}

type ScrapyRuntimeState struct {
	TaskID          int    `json:"taskId"`
	RunID           uint64 `json:"runId"`
	State           string `json:"state"`
	Phase           string `json:"phase"`
	RetryAt         int64  `json:"retryAt"`
	ReasonCode      string `json:"reasonCode"`
	Message         string `json:"message"`
	UpdatedAt       int64  `json:"updatedAt"`
	LastSuccessAt   int64  `json:"lastSuccessAt"`
	CompletedPages  int    `json:"completedPages"`
	CompletedRounds int    `json:"completedRounds"`
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

	runningTasks  map[int]*TaskRuntime
	runtimeStates map[int]ScrapyRuntimeState
	runSequence   atomic.Uint64
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
		runtimeStates:     map[int]ScrapyRuntimeState{},
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
	s.mu.Lock()
	delete(s.runtimeStates, id)
	s.mu.Unlock()
	return nil
}

func (s *Service) CreateScrapyItem(item dao.ScrapyItem) int64 {
	item.CreateTime = time.Now()
	// Creation flow keeps runtime settings at defaults; users can customize later per task.
	item.AccountID = 0
	if item.RequestIntervalSec <= 0 {
		item.RequestIntervalSec = minimumTaskRequestInterval.Seconds()
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
		stopping := running.stopping
		s.mu.Unlock()
		if stopping {
			return fmt.Errorf("task is stopping, please wait")
		}
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
	runID := s.runSequence.Add(1)
	runtime := &TaskRuntime{
		taskID:          taskID,
		runID:           runID,
		accountID:       scrapyItem.AccountID,
		cookies:         resolvedCookies,
		requestInterval: requestInterval,
		cancel:          cancel,
		done:            make(chan struct{}),
	}
	s.runningTasks[taskID] = runtime
	initialState := ScrapyRuntimeState{
		TaskID:          taskID,
		RunID:           runID,
		State:           "starting",
		Phase:           "initializing",
		Message:         "正在启动任务",
		UpdatedAt:       time.Now().UnixMilli(),
		CompletedPages:  scrapyItem.Nums,
		CompletedRounds: scrapyItem.IncreaseNumber,
	}
	s.runtimeStates[taskID] = initialState
	s.mu.Unlock()

	s.emitEvent("updateScrapyItem", scrapyItem)
	s.emitEvent("scrapy_task_status", initialState)

	s.wg.Add(1)
	go s.scrapyLoop(runtime, ctx)
	return nil
}

func (s *Service) UpdateScrapyTaskConfig(taskID int, accountID int64, requestIntervalSeconds float64) error {
	if requestIntervalSeconds < minimumTaskRequestInterval.Seconds() {
		return fmt.Errorf("request interval must be at least %.0f seconds", minimumTaskRequestInterval.Seconds())
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
	if runtime == nil || runtime.cancel == nil {
		s.mu.Unlock()
		return nil
	}
	if !runtime.stopping {
		runtime.stopping = true
		state := s.runtimeStates[taskID]
		state.State = "stopping"
		state.Phase = "stopping"
		state.RetryAt = 0
		state.ReasonCode = ""
		state.Message = "正在停止任务"
		state.UpdatedAt = time.Now().UnixMilli()
		s.runtimeStates[taskID] = state
		s.mu.Unlock()
		s.emitEvent("scrapy_task_status", state)
		runtime.cancel()
	} else {
		s.mu.Unlock()
	}

	timer := time.NewTimer(taskStopTimeout)
	defer timer.Stop()
	select {
	case <-runtime.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("task stop timed out")
	}
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

func (s *Service) GetScrapyRuntimeStates() []ScrapyRuntimeState {
	s.mu.Lock()
	defer s.mu.Unlock()

	states := make([]ScrapyRuntimeState, 0, len(s.runtimeStates))
	for _, state := range s.runtimeStates {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].TaskID < states[j].TaskID
	})
	return states
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config, err := client.GetMarketRuntimeConfig(ctx, bilihttp.ParseBiliSession(cookieStr))
	if err != nil {
		log.Warn().Err(err).Msg("failed to load remote market config, using fallback")
		fallback := bilihttp.DefaultMarketRuntimeConfig()
		if errors.Is(err, bilihttp.ErrMarketRequestDeferred) ||
			errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) {
			fallback.Message = "当前请求通道繁忙，已使用内置筛选配置"
		} else {
			fallback.Message = "远程筛选配置暂时不可用，已使用内置配置"
		}
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

func (s *Service) scrapyLoop(runtime *TaskRuntime, ctx context.Context) {
	defer s.wg.Done()
	defer close(runtime.done)
	defer s.unregisterTask(runtime.taskID, runtime.runID)

	taskID := runtime.taskID
	runID := runtime.runID

	scrapyItem, err := s.d.ReadScrapyItem(taskID)
	if err != nil {
		s.emitEvent("scrapyItem_get_failed", taskID)
		s.failTask(taskID, runID, "task_read_failed", "读取任务配置失败")
		return
	}

	scrapyItem.NextToken = normalizeNextToken(scrapyItem.NextToken)
	client, err := s.marketFn()
	if err != nil {
		log.Error().Err(err).Int("taskID", taskID).Uint64("runID", runID).Msg("failed to create market client")
		s.failTask(taskID, runID, "client_initialization_failed", "初始化网络客户端失败")
		s.emitEvent("scrapy_failed", taskID)
		return
	}
	session := bilihttp.ParseBiliSession(runtime.cookies)
	consecutiveRateLimits := 0
	retryCount := 0
	for {
		if ctx.Err() != nil {
			s.stopTask(taskID, runID)
			return
		}

		s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
			state.State = "running"
			state.Phase = "requesting"
			state.RetryAt = 0
			state.ReasonCode = ""
			state.Message = "正在获取商品数据"
		})
		requestCtx := bilihttp.WithMarketRequestWaitObserver(ctx, func(info bilihttp.MarketRequestWaitInfo) {
			log.Debug().
				Int("taskID", taskID).
				Uint64("runID", runID).
				Int64("accountID", runtime.accountID).
				Str("phase", "rate_limit_queue").
				Int64("waitMs", info.Wait.Milliseconds()).
				Msg("scrapy task waiting for safe request slot")
			s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
				state.State = "queued"
				state.Phase = "rate_limit_queue"
				state.RetryAt = time.Now().Add(info.Wait).UnixMilli()
				state.ReasonCode = ""
				state.Message = fmt.Sprintf("安全排队中，预计等待 %d 秒", durationSecondsCeil(info.Wait))
			})
		})
		roundFinished, err := s.scrapyTaskWithClient(requestCtx, taskID, client, session, &scrapyItem)
		if err != nil {
			if ctx.Err() != nil {
				s.stopTask(taskID, runID)
				return
			}

			classification := classifyScrapyTaskError(err)
			if !classification.retryable {
				log.Error().
					Err(err).
					Int("taskID", taskID).
					Uint64("runID", runID).
					Str("errorKind", classification.code).
					Msg("scrapy task failed")
				s.failTask(taskID, runID, classification.code, classification.message)
				s.emitEvent("scrapy_failed", taskID)
				return
			}

			delay := s.retryDelay
			if classification.code == "rate_limited" {
				consecutiveRateLimits++
				delay = rateLimitBackoff(consecutiveRateLimits, bilihttp.RetryAfter(err))
			} else {
				consecutiveRateLimits = 0
			}
			retryCount++
			retryAt := time.Now().Add(delay)
			s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
				state.State = "retrying"
				state.Phase = "retry_wait"
				state.RetryAt = retryAt.UnixMilli()
				state.ReasonCode = classification.code
				state.Message = classification.message
			})
			log.Warn().
				Err(err).
				Int("taskID", taskID).
				Uint64("runID", runID).
				Int("retryCount", retryCount).
				Str("errorKind", classification.code).
				Dur("retryDelay", delay).
				Msg("scrapy request will retry")
			s.emitEvent("scrapy_retry_wait", ScrapyRetryEvent{
				TaskID:  taskID,
				Seconds: durationSecondsCeil(delay),
				Reason:  classification.message,
			})
			s.emitEvent("scrapy_wait", durationSecondsCeil(delay))
			if !sleepWithContext(ctx, delay) {
				s.stopTask(taskID, runID)
				return
			}
			continue
		}
		consecutiveRateLimits = 0
		retryCount = 0
		s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
			state.State = "running"
			state.Phase = "persisted"
			state.RetryAt = 0
			state.ReasonCode = ""
			state.Message = "本页抓取成功"
			state.LastSuccessAt = time.Now().UnixMilli()
			state.CompletedPages = scrapyItem.Nums
			state.CompletedRounds = scrapyItem.IncreaseNumber
		})

		if roundFinished {
			scrapyItem.IncreaseNumber++
			if _, err := s.d.UpdateScrapyItem(&scrapyItem); err != nil {
				log.Error().Err(err).Int("taskID", taskID).Uint64("runID", runID).Msg("failed to persist scrapy round count")
				s.failTask(taskID, runID, "database_write_failed", "保存任务进度失败")
				s.emitEvent("scrapy_failed", taskID)
				return
			}
			s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
				state.CompletedRounds = scrapyItem.IncreaseNumber
				state.Message = "本轮抓取完成"
			})
			s.emitEvent("updateScrapyItem", scrapyItem)
			s.emitEvent("scrapy_round_finished", ScrapyRoundEvent{
				TaskID:      taskID,
				CompletedAt: time.Now().UnixMilli(),
			})
			// Keep backward compatibility for existing listeners.
			s.emitEvent("scrapy_finished", taskID)
			scrapyItem.NextToken = nil
			if !sleepWithContext(ctx, s.restartRoundDelay) {
				s.stopTask(taskID, runID)
				return
			}
			continue
		}

		if !sleepWithContext(ctx, runtime.requestInterval) {
			s.stopTask(taskID, runID)
			return
		}
	}
}

func (s *Service) scrapyTask(taskID int, cookies string, item *dao.ScrapyItem) (bool, error) {
	client, err := s.marketFn()
	if err != nil {
		return false, err
	}
	return s.scrapyTaskWithClient(context.Background(), taskID, client, bilihttp.ParseBiliSession(cookies), item)
}

func (s *Service) scrapyTaskWithClient(ctx context.Context, taskID int, client marketClient, session *bilihttp.BiliSession, item *dao.ScrapyItem) (bool, error) {
	resp, err := client.ListMarketItems(ctx, session, bilihttp.MarketListRequest{
		SortType:        item.Order,
		NextID:          normalizeNextToken(item.NextToken),
		PriceFilters:    []string{item.PriceFilter},
		DiscountFilters: []string{item.DiscountFilter},
		CategoryFilter:  item.Product,
	})
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
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

	s.trySendMonitorAlerts(ctx, taskID, resp.Data.Data)
	return item.NextToken == nil, nil
}

func (s *Service) trySendMonitorAlerts(ctx context.Context, taskID int, items []domain.MarketItem) {
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
		if ctx.Err() != nil {
			return
		}
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
				if ctx.Err() != nil {
					return
				}
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
				notifyCtx, cancel := context.WithTimeout(ctx, monitorNotificationTimeout)
				sendErr := s.notifier.SendMarkdown(notifyCtx, webhook, "市集助手", text)
				cancel()
				if sendErr != nil {
					_ = s.d.ReleaseMonitorAlertReservation(rule.ID, item.C2CItemsID)
					if ctx.Err() != nil {
						return
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

func (s *Service) unregisterTask(taskID int, runID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime := s.runningTasks[taskID]
	if runtime != nil && runtime.runID == runID {
		delete(s.runningTasks, taskID)
	}
}

func (s *Service) isTaskRunning(taskID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime := s.runningTasks[taskID]
	return runtime != nil && runtime.cancel != nil
}

func (s *Service) updateRuntimeState(taskID int, runID uint64, update func(*ScrapyRuntimeState)) {
	s.mu.Lock()
	state, ok := s.runtimeStates[taskID]
	if !ok || state.RunID != runID {
		s.mu.Unlock()
		return
	}
	update(&state)
	state.UpdatedAt = time.Now().UnixMilli()
	s.runtimeStates[taskID] = state
	s.mu.Unlock()
	s.emitEvent("scrapy_task_status", state)
}

func (s *Service) stopTask(taskID int, runID uint64) {
	log.Info().Int("taskID", taskID).Uint64("runID", runID).Str("phase", "stopped").Msg("scrapy task stopped")
	s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
		state.State = "stopped"
		state.Phase = "stopped"
		state.RetryAt = 0
		state.ReasonCode = ""
		state.Message = "任务已停止"
	})
}

func (s *Service) failTask(taskID int, runID uint64, code string, message string) {
	s.updateRuntimeState(taskID, runID, func(state *ScrapyRuntimeState) {
		state.State = "failed"
		state.Phase = "failed"
		state.RetryAt = 0
		state.ReasonCode = code
		state.Message = message
	})
}

type scrapyTaskErrorClassification struct {
	code      string
	message   string
	retryable bool
}

func classifyScrapyTaskError(err error) scrapyTaskErrorClassification {
	if bilihttp.IsRateLimitError(err) {
		return scrapyTaskErrorClassification{
			code:      "rate_limited",
			message:   "B站请求频率受限，任务会自动等待后重试",
			retryable: true,
		}
	}
	if bilihttp.IsAPIErrorKind(err, bilihttp.ErrKindUnauthorized) {
		return scrapyTaskErrorClassification{
			code:    "unauthorized",
			message: "登录状态已失效，请重新登录或更换账号",
		}
	}

	var statusErr *bilihttp.HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case 401, 403:
			return scrapyTaskErrorClassification{
				code:    "unauthorized",
				message: "登录状态已失效或当前账号无权访问",
			}
		case 400, 404, 405, 422:
			return scrapyTaskErrorClassification{
				code:    "invalid_request",
				message: "爬取配置无效，请检查筛选条件后重试",
			}
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return scrapyTaskErrorClassification{
			code:      "timeout",
			message:   "网络请求超时，任务会自动重试",
			retryable: true,
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return scrapyTaskErrorClassification{
			code:      "network",
			message:   "网络连接暂时不可用，任务会自动重试",
			retryable: true,
		}
	}
	return scrapyTaskErrorClassification{
		code:      "request_failed",
		message:   "请求暂时失败，任务会自动重试",
		retryable: true,
	}
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
	minimum := minimumTaskRequestInterval.Seconds()
	if seconds < minimum {
		return minimum
	}
	return seconds
}

func (s *Service) resolveTaskRequestInterval(seconds float64) time.Duration {
	if seconds >= minimumTaskRequestInterval.Seconds() {
		return time.Duration(seconds * float64(time.Second))
	}
	if s.requestInterval > 0 {
		return s.requestInterval
	}
	return time.Duration(normalizeIntervalSeconds(seconds) * float64(time.Second))
}

func rateLimitBackoff(strikes int, retryAfter time.Duration) time.Duration {
	if strikes < 1 {
		strikes = 1
	}
	delay := rateLimitRetryDelay
	for i := 1; i < strikes && delay < maximumRateLimitRetryDelay; i++ {
		delay *= 2
		if delay > maximumRateLimitRetryDelay {
			delay = maximumRateLimitRetryDelay
		}
	}
	if retryAfter > delay {
		return retryAfter
	}
	return delay
}

func durationSecondsCeil(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	return int((duration + time.Second - 1) / time.Second)
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
