package http

import "fmt"

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

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s (%d): %s", e.Kind, e.Code, e.Message)
}

func IsAPIErrorKind(err error, kind string) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Kind == kind
}

func classifyMarketError(code int, message string) error {
	switch code {
	case 0:
		return nil
	case 429:
		return &APIError{Kind: ErrKindRateLimited, Code: code, Message: message}
	case 83001002:
		return &APIError{Kind: ErrKindUnauthorized, Code: code, Message: message}
	case 83000004:
		return &APIError{Kind: ErrKindService, Code: code, Message: message}
	default:
		return &APIError{Kind: ErrKindService, Code: code, Message: message}
	}
}
