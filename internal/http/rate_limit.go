package http

import (
	"context"
	"crypto/sha256"
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

type marketRateLimitState struct {
	mu          sync.Mutex
	nextRequest time.Time
	strikes     int
}

type marketRateLimiter struct {
	mu     sync.Mutex
	states map[string]*marketRateLimitState
}

var defaultMarketRateLimiter = newMarketRateLimiter()

func newMarketRateLimiter() *marketRateLimiter {
	return &marketRateLimiter{states: make(map[string]*marketRateLimitState)}
}

func (l *marketRateLimiter) Wait(ctx context.Context, req *nethttp.Request) (string, error) {
	identity, ok := marketRequestIdentity(req)
	if l == nil || !ok {
		return "", nil
	}
	state := l.state(identity)

	for {
		state.mu.Lock()
		now := time.Now()
		if !now.Before(state.nextRequest) {
			jitter := time.Duration(rand.Int64N(int64(marketRequestJitter) + 1))
			state.nextRequest = now.Add(minimumMarketRequestInterval + jitter)
			state.mu.Unlock()
			return identity, nil
		}
		wait := time.Until(state.nextRequest)
		state.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return identity, ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *marketRateLimiter) Penalize(identity string, requested time.Duration) time.Duration {
	if l == nil || identity == "" {
		return normalizeRateLimitCooldown(requested, 1)
	}
	state := l.state(identity)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.strikes++
	cooldown := normalizeRateLimitCooldown(requested, state.strikes)
	next := time.Now().Add(cooldown)
	if next.After(state.nextRequest) {
		state.nextRequest = next
	}
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
		state = &marketRateLimitState{}
		l.states[identity] = state
	}
	return state
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
