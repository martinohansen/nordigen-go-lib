package nordigen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const baseUrl = "bankaccountdata.gocardless.com"
const apiPath = "/api/v2"

type Client struct {
	c         *http.Client
	secretId  string
	secretKey string

	m     *sync.RWMutex
	token *Token
}

type Transport struct {
	rt  http.RoundTripper
	cli *Client
}

// StartTokenHandler handles token refreshes in the background
func (c *Client) StartTokenHandler(ctx context.Context) error {
	// Initialize the first token
	err := c.newToken(ctx)
	if err != nil {
		return errors.New("getting initial token: " + err.Error())
	}

	go c.tokenHandler(ctx)
	return nil
}

// tokenHandler gets a new token using the refresh token and a new pair when the
// refresh token expires
func (c *Client) tokenHandler(ctx context.Context) {
	refresh := time.NewTicker(time.Hour * 12)  // 12 hours
	new := time.NewTicker(time.Hour * 24 * 14) // 14 days
	defer refresh.Stop()
	defer new.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-new.C:
			if err := c.newToken(ctx); err != nil {
				// TODO(Martin): Improve error handling
				panic(fmt.Sprintf("getting new token: %s", err))
			}

		case <-refresh.C:
			if err := c.refreshToken(ctx); err != nil {
				panic(fmt.Sprintf("refreshing token: %s", err))
			}
		}
	}
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "https"
	req.URL.Host = baseUrl
	req.URL.Path = strings.Join([]string{apiPath, req.URL.Path}, "/")

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Add the access token to the request if it exists
	if t.cli.token != nil {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.cli.token.Access))
	}

	return t.rt.RoundTrip(req)
}

// NewClient creates a new Nordigen client that handles token refreshes and adds
// the necessary headers, host, and path to all requests.
func NewClient(secretId, secretKey string) (*Client, error) {
	c := &Client{
		c:         &http.Client{Timeout: 60 * time.Second},
		secretId:  secretId,
		secretKey: secretKey,

		m: &sync.RWMutex{},
	}

	// Add transport to handle headers, host and path for all requests
	c.c.Transport = Transport{rt: http.DefaultTransport, cli: c}

	// Start token handler
	if err := c.StartTokenHandler(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

// do sends the HTTP request and returns a RateLimitError when the response code
// indicates that the request was throttled.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.c.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		// We have been rate limited, parse the response and return a
		// RateLimitError for the caller to handle.
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		rl := parseRateLimit(resp.Header)
		return nil, &RateLimitError{APIError: &APIError{StatusCode: resp.StatusCode, Body: string(body)}, RateLimit: rl}
	}

	return resp, nil
}

// parseRateLimit extracts rate limit information from the HTTP headers
// https://bankaccountdata.zendesk.com/hc/en-gb/articles/11529584398236-Bank-API-Rate-Limits-and-Rate-Limit-Headers
func parseRateLimit(h http.Header) RateLimit {
	toInt := func(v string) int {
		i, _ := strconv.Atoi(v)
		return i
	}

	rl := RateLimit{}

	if v := h.Get("HTTP_X_RATELIMIT_ACCOUNT_SUCCESS_LIMIT"); v != "" {
		rl.Limit = toInt(v)
	} else {
		rl.Limit = toInt(h.Get("HTTP_X_RATELIMIT_LIMIT"))
	}

	if v := h.Get("HTTP_X_RATELIMIT_ACCOUNT_SUCCESS_REMAINING"); v != "" {
		rl.Remaining = toInt(v)
	} else {
		rl.Remaining = toInt(h.Get("HTTP_X_RATELIMIT_REMAINING"))
	}

	if v := h.Get("HTTP_X_RATELIMIT_ACCOUNT_SUCCESS_RESET"); v != "" {
		rl.Reset = toInt(v)
	} else {
		rl.Reset = toInt(h.Get("HTTP_X_RATELIMIT_RESET"))
	}

	return rl
}
