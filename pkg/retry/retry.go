package retry

import (
	"net/http"
)

func IsRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return true
	default:
		return statusCode >= 500 && statusCode < 600
	}
}
