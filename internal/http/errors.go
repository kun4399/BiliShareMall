package http

import (
	"errors"
	"fmt"
	"time"
)

const (
	ErrKindUnauthorized = "unauthorized"
	ErrKindRateLimited  = "rate_limited"
	ErrKindService      = "service"
	ErrKindExpired      = "expired"
	ErrKindPending      = "pending"
)

type APIError struct {
	Kind    string
	Code    int
	Message string
}

// HTTPStatusError preserves status and retry metadata without exposing an
// upstream HTML error page to callers.
type HTTPStatusError struct {
	StatusCode int
	RetryAfter time.Duration
	Message    string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.StatusCode == 429 {
		if e.RetryAfter > 0 {
			return fmt.Sprintf("B站请求过于频繁（HTTP 429），建议等待 %s后重试", formatRetryDuration(e.RetryAfter))
		}
		return "B站请求过于频繁（HTTP 429），请稍后重试"
	}
	if e.Message != "" {
		return fmt.Sprintf("request returned HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("request returned HTTP %d", e.StatusCode)
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s (%d): %s", e.Kind, e.Code, e.Message)
}

func IsAPIErrorKind(err error, kind string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == kind
}

func IsRateLimitError(err error) bool {
	if IsAPIErrorKind(err, ErrKindRateLimited) {
		return true
	}
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == 429
}

func RetryAfter(err error) time.Duration {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.RetryAfter
	}
	return 0
}

func formatRetryDuration(duration time.Duration) string {
	seconds := int(duration.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d 分钟", seconds/60)
	}
	return fmt.Sprintf("%d 秒", seconds)
}

func classifyMarketError(code int, message string) error {
	switch code {
	case 0:
		return nil
	case 429:
		return &APIError{Kind: ErrKindRateLimited, Code: code, Message: message}
	case 71102072, 83001002:
		return &APIError{Kind: ErrKindUnauthorized, Code: code, Message: message}
	case 83000004:
		return &APIError{Kind: ErrKindService, Code: code, Message: message}
	default:
		return &APIError{Kind: ErrKindService, Code: code, Message: message}
	}
}
