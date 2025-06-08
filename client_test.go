package nordigen

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

var (
	sharedClient *Client
	initOnce     sync.Once
)

func initTestClient(t *testing.T) *Client {
	id, idExists := os.LookupEnv("NORDIGEN_SECRET_ID")
	key, keyExists := os.LookupEnv("NORDIGEN_SECRET_KEY")
	if !idExists || !keyExists {
		t.Skip("NORDIGEN_SECRET_ID and NORDIGEN_SECRET_KEY not set")
	}

	initOnce.Do(func() {
		c := &Client{
			c:         &http.Client{Timeout: 60 * time.Second},
			secretId:  id,
			secretKey: key,

			m: &sync.RWMutex{},
		}
		c.c.Transport = Transport{rt: http.DefaultTransport, cli: c}

		// Initialize the first token
		err := c.newToken(context.Background())
		if err != nil {
			t.Fatalf("newToken: %s", err)
		}

		sharedClient = c
	})

	return sharedClient
}

func TestAccessRefresh(t *testing.T) {
	c := initTestClient(t)

	// Expire token immediately
	c.token.AccessExpires = 0

	ctx, cancel := context.WithCancel(context.Background())
	go c.tokenHandler(ctx)
	_, err := c.ListRequisitions()
	if err != nil {
		t.Fatalf("ListRequisitions: %s", err)
	}
	cancel() // Stop handler again
}

func TestRefreshRefresh(t *testing.T) {
	c := initTestClient(t)

	// Expire token immediately
	c.token.RefreshExpires = 0

	ctx, cancel := context.WithCancel(context.Background())
	go c.tokenHandler(ctx)
	_, err := c.ListRequisitions()
	if err != nil {
		t.Fatalf("ListRequisitions: %s", err)
	}
	cancel() // Stop handler again
}

func TestRateLimitError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("HTTP_X_RATELIMIT_LIMIT", "10")
		w.Header().Set("HTTP_X_RATELIMIT_REMAINING", "0")
		w.Header().Set("HTTP_X_RATELIMIT_RESET", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer ts.Close()

	c := &Client{c: ts.Client()}
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Errorf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err == nil {
		t.Errorf("expected error")
	}
	if resp != nil {
		t.Errorf("expected nil response")
	}

	rlErr, ok := err.(*RateLimitError)
	if !ok {
		t.Errorf("expected RateLimitError got %T", err)
	}
	if rlErr.RateLimit.Limit != 10 || rlErr.RateLimit.Remaining != 0 || rlErr.RateLimit.Reset != 5 {
		t.Errorf("unexpected rate limit values: %+v", rlErr.RateLimit)
	}

	// Error should unwrap
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T", err)
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("expected %v, got %v", apiErr, err)
	}
}
