package nordigen

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

	// Name of the client, useful if you have multiple clients
	name string
}

type Transport struct {
	rt  http.RoundTripper
	cli *Client
}

func (c Client) String() string {
	if c.name == "" {
		return "nordigen-go-lib" + "@" + baseUrl
	}
	return c.name + "@" + baseUrl
}

// StartTokenHandler handles token refreshes in the background
func (c *Client) StartTokenHandler(ctx context.Context) error {
	// Initialize the first token
	token, err := c.newToken(ctx)
	if err != nil {
		return errors.New("getting initial token: " + err.Error())
	}
	c.m.Lock()
	c.token = token
	c.m.Unlock()

	go c.tokenHandler(ctx)
	return nil
}

// tokenHandler gets a new token using the refresh token and a new pair when the
// refresh token expires.
func (c *Client) tokenHandler(ctx context.Context) {
	logger := slog.With("client", c.String(), "method", "tokenHandler")

	newTokenTimer := time.NewTimer(0)     // Start immediately
	refreshTokenTimer := time.NewTimer(0) // Start immediately
	defer func() {
		newTokenTimer.Stop()
		refreshTokenTimer.Stop()
		logger.Debug("stopped")
	}()

	resetTimer := func(timer *time.Timer, expiryTime time.Time) {
		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset(time.Until(expiryTime))
	}

	logger.Debug("starting")
	for {
		c.m.RLock()
		newTokenExpiry := c.token.accessExpires(2)
		refreshTokenExpiry := c.token.refreshExpires(2)
		c.m.RUnlock()

		resetTimer(newTokenTimer, newTokenExpiry)
		logger.Debug("new token timer", "expiry", newTokenExpiry)
		resetTimer(refreshTokenTimer, refreshTokenExpiry)
		logger.Debug("refresh token timer", "expiry", refreshTokenExpiry)

		select {
		case <-ctx.Done():
			logger.Debug("context done")
			return
		case <-newTokenTimer.C:
			logger.Debug("getting new token")
			if token, err := c.newToken(ctx); err != nil {
				panic(fmt.Sprintf("getting new token: %s", err))
			} else {
				c.updateToken(token)
			}
		case <-refreshTokenTimer.C:
			logger.Debug("refreshing token")
			if token, err := c.refreshToken(ctx); err != nil {
				panic(fmt.Sprintf("refreshing token: %s", err))
			} else {
				c.updateToken(token)
			}
		}
	}
}

// updateToken updates the client's token
func (c *Client) updateToken(t *Token) {
	c.m.Lock()
	defer c.m.Unlock()
	c.token = t
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
