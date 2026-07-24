package http

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	nethttp "net/http"
	"strings"
	"sync"
	"time"
)

const (
	minimumMarketRequestInterval = 12 * time.Second
	marketRequestJitter          = 3 * time.Second
	initialRateLimitCooldown     = 60 * time.Second
	maximumRateLimitCooldown     = 15 * time.Minute
)

type MarketRequestPriority int

const (
	MarketRequestLowPriority MarketRequestPriority = iota
	MarketRequestNormalPriority
)

type MarketRequestWaitInfo struct {
	Wait time.Duration
}

type marketRequestContextKey struct{}
type marketRequestWaitObserverKey struct{}

// ErrMarketRequestDeferred means a low-priority request could not safely run
// before its deadline. Callers should use cached or bundled data instead.
var ErrMarketRequestDeferred = errors.New("safe request slot unavailable before deadline")

func WithMarketRequestPriority(ctx context.Context, priority MarketRequestPriority) context.Context {
	return context.WithValue(ctx, marketRequestContextKey{}, priority)
}

func WithMarketRequestWaitObserver(ctx context.Context, observer func(MarketRequestWaitInfo)) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, marketRequestWaitObserverKey{}, observer)
}

type marketRateLimitWaiter struct {
	ctx      context.Context
	priority MarketRequestPriority
	ready    chan struct{}
}

type marketRateLimitState struct {
	mu          sync.Mutex
	nextRequest time.Time
	strikes     int
	waiters     []*marketRateLimitWaiter
	wake        chan struct{}
	minInterval time.Duration
	jitter      time.Duration
}

type marketRateLimiter struct {
	mu          sync.Mutex
	states      map[string]*marketRateLimitState
	minInterval time.Duration
	jitter      time.Duration
}

var defaultMarketRateLimiter = newMarketRateLimiter()

func newMarketRateLimiter() *marketRateLimiter {
	return &marketRateLimiter{
		states:      make(map[string]*marketRateLimitState),
		minInterval: minimumMarketRequestInterval,
		jitter:      marketRequestJitter,
	}
}

func (l *marketRateLimiter) Wait(ctx context.Context, req *nethttp.Request) (string, error) {
	identity, ok := marketRequestIdentity(req)
	if l == nil || !ok {
		return "", nil
	}

	state := l.state(identity)
	priority := marketPriorityFromContext(ctx)
	waiter := &marketRateLimitWaiter{
		ctx:      ctx,
		priority: priority,
		ready:    make(chan struct{}),
	}

	state.mu.Lock()
	estimatedWait := state.estimatedWaitLocked(priority)
	if priority == MarketRequestLowPriority {
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline && time.Now().Add(estimatedWait).After(deadline) {
			state.mu.Unlock()
			return identity, ErrMarketRequestDeferred
		}
	}
	state.waiters = append(state.waiters, waiter)
	state.mu.Unlock()

	notifyMarketWait(ctx, estimatedWait)
	state.signal()

	select {
	case <-waiter.ready:
		return identity, nil
	case <-ctx.Done():
		state.mu.Lock()
		state.removeWaiterLocked(waiter)
		state.mu.Unlock()
		state.signal()
		return identity, ctx.Err()
	}
}

func (l *marketRateLimiter) Penalize(identity string, requested time.Duration) time.Duration {
	if l == nil || identity == "" {
		return normalizeRateLimitCooldown(requested, 1)
	}
	state := l.state(identity)
	state.mu.Lock()
	state.strikes++
	cooldown := normalizeRateLimitCooldown(requested, state.strikes)
	next := time.Now().Add(cooldown)
	if next.After(state.nextRequest) {
		state.nextRequest = next
	}
	state.mu.Unlock()
	state.signal()
	return cooldown
}

func (l *marketRateLimiter) MarkSuccess(identity string) {
	if l == nil || identity == "" {
		return
	}
	state := l.state(identity)
	state.mu.Lock()
	state.strikes = 0
	state.mu.Unlock()
}

func (l *marketRateLimiter) state(identity string) *marketRateLimitState {
	l.mu.Lock()
	defer l.mu.Unlock()
	state := l.states[identity]
	if state == nil {
		state = &marketRateLimitState{
			wake:        make(chan struct{}, 1),
			minInterval: l.minInterval,
			jitter:      l.jitter,
		}
		l.states[identity] = state
		go state.dispatch()
	}
	return state
}

func (s *marketRateLimitState) dispatch() {
	for {
		s.mu.Lock()
		s.removeCanceledWaitersLocked()
		waiterIndex := s.nextWaiterIndexLocked()
		if waiterIndex < 0 {
			s.mu.Unlock()
			<-s.wake
			continue
		}

		wait := time.Until(s.nextRequest)
		if wait > 0 {
			waiterContext := s.waiters[waiterIndex].ctx
			s.mu.Unlock()
			notifyMarketWait(waiterContext, wait)
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-s.wake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			continue
		}

		waiter := s.waiters[waiterIndex]
		if waiter.ctx.Err() != nil {
			s.waiters = append(s.waiters[:waiterIndex], s.waiters[waiterIndex+1:]...)
			s.mu.Unlock()
			continue
		}
		s.waiters = append(s.waiters[:waiterIndex], s.waiters[waiterIndex+1:]...)
		jitter := time.Duration(0)
		if s.jitter > 0 {
			jitter = time.Duration(rand.Int64N(int64(s.jitter) + 1))
		}
		s.nextRequest = time.Now().Add(s.minInterval + jitter)
		s.mu.Unlock()
		close(waiter.ready)
	}
}

func (s *marketRateLimitState) nextWaiterIndexLocked() int {
	for i, waiter := range s.waiters {
		if waiter.priority == MarketRequestNormalPriority {
			return i
		}
	}
	if len(s.waiters) > 0 {
		return 0
	}
	return -1
}

func (s *marketRateLimitState) estimatedWaitLocked(priority MarketRequestPriority) time.Duration {
	wait := time.Until(s.nextRequest)
	if wait < 0 {
		wait = 0
	}
	ahead := 0
	for _, waiter := range s.waiters {
		if waiter.ctx.Err() != nil {
			continue
		}
		if priority == MarketRequestNormalPriority && waiter.priority == MarketRequestLowPriority {
			continue
		}
		ahead++
	}
	if ahead > 0 {
		wait += time.Duration(ahead) * s.minInterval
	}
	return wait
}

func (s *marketRateLimitState) removeCanceledWaitersLocked() {
	filtered := s.waiters[:0]
	for _, waiter := range s.waiters {
		if waiter.ctx.Err() == nil {
			filtered = append(filtered, waiter)
		}
	}
	s.waiters = filtered
}

func (s *marketRateLimitState) removeWaiterLocked(target *marketRateLimitWaiter) {
	for i, waiter := range s.waiters {
		if waiter == target {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return
		}
	}
}

func (s *marketRateLimitState) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func marketPriorityFromContext(ctx context.Context) MarketRequestPriority {
	if priority, ok := ctx.Value(marketRequestContextKey{}).(MarketRequestPriority); ok {
		return priority
	}
	return MarketRequestNormalPriority
}

func notifyMarketWait(ctx context.Context, wait time.Duration) {
	if wait <= 0 {
		return
	}
	if observer, ok := ctx.Value(marketRequestWaitObserverKey{}).(func(MarketRequestWaitInfo)); ok {
		observer(MarketRequestWaitInfo{Wait: wait})
	}
}

func marketRequestIdentity(req *nethttp.Request) (string, bool) {
	if req == nil || req.URL == nil || !strings.EqualFold(req.URL.Hostname(), "mall.bilibili.com") {
		return "", false
	}
	if !strings.HasPrefix(req.URL.Path, "/mall-magic-c/") {
		return "", false
	}

	cookieHeader := strings.TrimSpace(req.Header.Get("Cookie"))
	session := ParseBiliSession(cookieHeader)
	if uid := strings.TrimSpace(session.Cookies["DedeUserID"]); uid != "" {
		return "uid:" + uid, true
	}
	if cookieHeader == "" {
		return "anonymous", true
	}
	sum := sha256.Sum256([]byte(cookieHeader))
	return fmt.Sprintf("cookie:%x", sum[:8]), true
}

func normalizeRateLimitCooldown(requested time.Duration, strikes int) time.Duration {
	if strikes < 1 {
		strikes = 1
	}
	cooldown := initialRateLimitCooldown
	for i := 1; i < strikes && cooldown < maximumRateLimitCooldown; i++ {
		cooldown *= 2
		if cooldown > maximumRateLimitCooldown {
			cooldown = maximumRateLimitCooldown
		}
	}
	if requested > cooldown {
		cooldown = requested
	}
	return cooldown
}
