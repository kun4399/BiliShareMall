package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestMarketRequestIdentityUsesAccountUID(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://mall.bilibili.com/mall-magic-c/internet/c2c/v2/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "SESSDATA=secret; DedeUserID=1001; bili_jct=csrf")

	identity, ok := marketRequestIdentity(req)
	if !ok {
		t.Fatal("expected market request to be rate limited")
	}
	if identity != "uid:1001" {
		t.Fatalf("expected uid identity, got %q", identity)
	}
}

func TestMarketRateLimiterUsesExponentialCooldownAndResetsAfterSuccess(t *testing.T) {
	limiter := newMarketRateLimiter()
	identity := "uid:1001"

	if got := limiter.Penalize(identity, 0); got != time.Minute {
		t.Fatalf("expected first cooldown 1m, got %s", got)
	}
	if got := limiter.Penalize(identity, 0); got != 2*time.Minute {
		t.Fatalf("expected second cooldown 2m, got %s", got)
	}

	limiter.MarkSuccess(identity)
	if got := limiter.Penalize(identity, 0); got != time.Minute {
		t.Fatalf("expected cooldown reset to 1m, got %s", got)
	}
}

func TestNormalizeRateLimitCooldownHonorsRetryAfterAndCapsAutomaticDelay(t *testing.T) {
	if got := normalizeRateLimitCooldown(3*time.Minute, 1); got != 3*time.Minute {
		t.Fatalf("expected retry-after 3m, got %s", got)
	}
	if got := normalizeRateLimitCooldown(time.Hour, 1); got != time.Hour {
		t.Fatalf("expected explicit retry-after 1h, got %s", got)
	}
	if got := normalizeRateLimitCooldown(0, 10); got != maximumRateLimitCooldown {
		t.Fatalf("expected automatic cooldown cap %s, got %s", maximumRateLimitCooldown, got)
	}
}

func TestMarketRateLimiterServesSameAccountInFIFOOrder(t *testing.T) {
	limiter := newMarketRateLimiter()
	limiter.minInterval = 15 * time.Millisecond
	limiter.jitter = 0

	req := marketRateLimitTestRequest(t, "1001")
	if _, err := limiter.Wait(context.Background(), req.Clone(context.Background())); err != nil {
		t.Fatalf("initial Wait error: %v", err)
	}

	results := make(chan int, 2)
	for i := 2; i <= 3; i++ {
		index := i
		queued := make(chan struct{}, 1)
		go func() {
			ctx := WithMarketRequestWaitObserver(context.Background(), func(MarketRequestWaitInfo) {
				select {
				case queued <- struct{}{}:
				default:
				}
			})
			if _, err := limiter.Wait(ctx, req.Clone(ctx)); err != nil {
				t.Errorf("Wait(%d) error: %v", index, err)
				return
			}
			results <- index
		}()
		select {
		case <-queued:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for request to queue")
		}
	}

	for expected := 2; expected <= 3; expected++ {
		select {
		case got := <-results:
			if got != expected {
				t.Fatalf("expected FIFO result %d, got %d", expected, got)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for limiter")
		}
	}
}

func TestMarketRateLimiterCanceledWaiterDoesNotConsumeNextSlot(t *testing.T) {
	limiter := newMarketRateLimiter()
	limiter.minInterval = 30 * time.Millisecond
	limiter.jitter = 0
	req := marketRateLimitTestRequest(t, "1001")

	if _, err := limiter.Wait(context.Background(), req.Clone(context.Background())); err != nil {
		t.Fatalf("initial Wait error: %v", err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	canceledResult := make(chan error, 1)
	go func() {
		_, err := limiter.Wait(cancelCtx, req.Clone(cancelCtx))
		canceledResult <- err
	}()
	time.Sleep(3 * time.Millisecond)

	nextResult := make(chan time.Time, 1)
	startedAt := time.Now()
	go func() {
		_, _ = limiter.Wait(context.Background(), req.Clone(context.Background()))
		nextResult <- time.Now()
	}()
	cancel()

	if err := <-canceledResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	select {
	case acquiredAt := <-nextResult:
		if elapsed := acquiredAt.Sub(startedAt); elapsed > 55*time.Millisecond {
			t.Fatalf("canceled waiter consumed a slot, next request waited %s", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for next request")
	}
}

func TestMarketRateLimiterDefersLowPriorityRequestPastDeadline(t *testing.T) {
	limiter := newMarketRateLimiter()
	limiter.minInterval = 50 * time.Millisecond
	limiter.jitter = 0
	req := marketRateLimitTestRequest(t, "1001")

	if _, err := limiter.Wait(context.Background(), req.Clone(context.Background())); err != nil {
		t.Fatalf("initial Wait error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	ctx = WithMarketRequestPriority(ctx, MarketRequestLowPriority)
	if _, err := limiter.Wait(ctx, req.Clone(ctx)); !errors.Is(err, ErrMarketRequestDeferred) {
		t.Fatalf("expected deferred low-priority request, got %v", err)
	}
}

func TestMarketRateLimiterPrioritizesNormalRequestsOverQueuedStatusRefresh(t *testing.T) {
	limiter := newMarketRateLimiter()
	limiter.minInterval = 20 * time.Millisecond
	limiter.jitter = 0
	req := marketRateLimitTestRequest(t, "1001")

	if _, err := limiter.Wait(context.Background(), req.Clone(context.Background())); err != nil {
		t.Fatalf("initial Wait error: %v", err)
	}

	results := make(chan string, 2)
	lowQueued := make(chan struct{}, 1)
	lowCtx := WithMarketRequestPriority(context.Background(), MarketRequestLowPriority)
	lowCtx = WithMarketRequestWaitObserver(lowCtx, func(MarketRequestWaitInfo) {
		select {
		case lowQueued <- struct{}{}:
		default:
		}
	})
	go func() {
		_, _ = limiter.Wait(lowCtx, req.Clone(lowCtx))
		results <- "low"
	}()
	select {
	case <-lowQueued:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for low-priority request to queue")
	}

	normalQueued := make(chan struct{}, 1)
	normalCtx := WithMarketRequestWaitObserver(context.Background(), func(MarketRequestWaitInfo) {
		select {
		case normalQueued <- struct{}{}:
		default:
		}
	})
	go func() {
		_, _ = limiter.Wait(normalCtx, req.Clone(normalCtx))
		results <- "normal"
	}()
	select {
	case <-normalQueued:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for normal request to queue")
	}

	select {
	case first := <-results:
		if first != "normal" {
			t.Fatalf("expected normal request first, got %s", first)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prioritized request")
	}
	select {
	case second := <-results:
		if second != "low" {
			t.Fatalf("expected low-priority request second, got %s", second)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for low-priority request")
	}
}

func TestMarketRateLimiterKeepsDifferentAccountsIndependent(t *testing.T) {
	limiter := newMarketRateLimiter()
	limiter.minInterval = 100 * time.Millisecond
	limiter.jitter = 0
	accountOne := marketRateLimitTestRequest(t, "1001")
	accountTwo := marketRateLimitTestRequest(t, "2002")

	if _, err := limiter.Wait(context.Background(), accountOne.Clone(context.Background())); err != nil {
		t.Fatalf("initial account-one Wait error: %v", err)
	}
	acquired := make(chan struct{}, 1)
	go func() {
		_, _ = limiter.Wait(context.Background(), accountTwo.Clone(context.Background()))
		acquired <- struct{}{}
	}()

	select {
	case <-acquired:
	case <-time.After(40 * time.Millisecond):
		t.Fatal("different account was blocked by another account's interval")
	}
}

func marketRateLimitTestRequest(t *testing.T, uid string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://mall.bilibili.com/mall-magic-c/internet/c2c/v2/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "SESSDATA=secret; DedeUserID="+uid+"; bili_jct=csrf")
	return req
}
