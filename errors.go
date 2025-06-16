package nordigen

import (
	"fmt"
	"time"
)

type APIError struct {
	StatusCode int
	Body       string
	Err        error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("APIError %v %v", e.StatusCode, e.Body)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// RateLimit contains information from rate limit headers.
type RateLimit struct {
	Limit     int // Maximum number of requests allowed
	Remaining int // Requests remaining before hitting the limit
	Reset     int // Seconds until the limit resets
}

// RateLimitError is returned when the API responds with HTTP 429. It embeds the
// APIError and exposes the parsed rate limit headers so callers can decide how
// to handle the throttling.
type RateLimitError struct {
	*APIError
	RateLimit RateLimit
}

// Resets returns the duration until the rate limit resets
func (rl RateLimitError) Resets() time.Duration {
	return time.Duration(rl.RateLimit.Reset) * time.Second
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("RateLimitError %v limit=%d remaining=%d reset=%d", e.StatusCode, e.RateLimit.Limit, e.RateLimit.Remaining, e.RateLimit.Reset)
}

func (e *RateLimitError) Unwrap() error {
	return e.APIError
}
