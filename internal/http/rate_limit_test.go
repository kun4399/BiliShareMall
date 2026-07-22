package http

import (
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
