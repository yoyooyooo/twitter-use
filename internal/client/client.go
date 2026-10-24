package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/accounts"
	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/graphql"
	"github.com/yoyooyooo/twitter-use/internal/jsonx"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/xid"
)

const (
	absoluteMax    = 500
	emptyPageLimit = 2
)

var resumeStops = map[string]struct{}{"max_count": {}, "rate_limited": {}, "attempt_limit": {}}

// Credentials is one login. Cookie is the full browser cookie header, if any.
type Credentials struct {
	AuthToken string
	CT0       string
	Cookie    string
}

// Rate is the retry policy. It does not cap how many items a command may request.
type Rate struct {
	RequestDelay   time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
}

// Client talks to the X GraphQL API through a Chrome-impersonating transport.
type Client struct {
	creds       Credentials
	pool        *accounts.Pool
	current     *accounts.Account
	rate        Rate
	http        Doer
	fetch       model.FetchStatus
	since       time.Time
	until       time.Time
	transaction *xid.Generator
	triedTx     bool
	sleep       func(time.Duration)
	now         func() time.Time
	tx          bool
}

// Option configures a client.
type Option func(*Client)

// WithHTTP replaces the transport. Tests use this.
func WithHTTP(doer Doer) Option { return func(c *Client) { c.http = doer } }

// WithSleep replaces delays. Tests use a no-op.
func WithSleep(fn func(time.Duration)) Option { return func(c *Client) { c.sleep = fn } }

// WithoutTransaction skips homepage transaction-id setup.
func WithoutTransaction() Option { return func(c *Client) { c.tx = false } }

// New builds a client. A nil pool means the single login in creds.
func New(creds Credentials, pool *accounts.Pool, rate Rate, opts ...Option) (*Client, error) {
	c := &Client{creds: creds, pool: pool, rate: rate, sleep: time.Sleep, now: time.Now, tx: true}
	for _, opt := range opts {
		opt(c)
	}
	if c.http == nil {
		transport, err := newTransport()
		if err != nil {
			return nil, err
		}
		c.http = transport
	}
	if c.tx {
		c.ensureTransaction()
	}
	return c, nil
}

// FetchStatus is the latest pagination record.
func (c *Client) FetchStatus() model.FetchStatus { return c.fetch }

// SetWindow limits a reverse-chronological timeline. Zero means unset.
func (c *Client) SetWindow(since, until time.Time) { c.since, c.until = since, until }

// EnforceCount rejects a request above the single ceiling.
func EnforceCount(count int) (int, error) {
	if count > absoluteMax {
		return 0, fmt.Errorf("Requested %d items, which exceeds the fetch ceiling of %d", count, absoluteMax)
	}
	return count, nil
}

func (c *Client) graphqlGet(operation string, variables map[string]any, features map[string]any, toggles map[string]any) (map[string]any, error) {
	queryID, err := graphql.Resolve(operation, true, c.fetchText)
	if err != nil {
		return nil, err
	}
	usingFallback := queryID == graphql.FallbackQueryIDs[operation]
	data, err := c.api(graphql.BuildURL(queryID, operation, variables, features, toggles), http.MethodGet, nil)
	if err == nil {
		return data, nil
	}
	apiErr, ok := err.(*errs.Error)
	if ok && (apiErr.Status == 404 || apiErr.Status == 422) && usingFallback {
		graphql.Invalidate(operation)
		queryID, resolveErr := graphql.Resolve(operation, false, c.fetchText)
		if resolveErr != nil {
			return nil, resolveErr
		}
		return c.api(graphql.BuildURL(queryID, operation, variables, features, toggles), http.MethodGet, nil)
	}
	return nil, err
}

func (c *Client) graphqlPost(operation string, variables map[string]any, features map[string]any) (map[string]any, error) {
	queryID, err := graphql.Resolve(operation, true, c.fetchText)
	if err != nil {
		return nil, err
	}
	usingFallback := queryID == graphql.FallbackQueryIDs[operation]
	do := func(id string) (map[string]any, error) {
		body := map[string]any{"variables": variables, "queryId": id}
		if features != nil {
			body["features"] = features
		}
		return c.api("https://x.com/i/api/graphql/"+id+"/"+operation, http.MethodPost, body)
	}
	data, err := do(queryID)
	if err == nil {
		return data, nil
	}
	apiErr, ok := err.(*errs.Error)
	if ok && (apiErr.Status == 404 || apiErr.Status == 422) && usingFallback {
		graphql.Invalidate(operation)
		queryID, resolveErr := graphql.Resolve(operation, false, c.fetchText)
		if resolveErr != nil {
			return nil, resolveErr
		}
		return do(queryID)
	}
	return nil, err
}

func (c *Client) api(rawURL, method string, body map[string]any) (map[string]any, error) {
	operation := accounts.OperationKey(rawURL)
	switches := 0
	maxSwitches := 0
	if c.pool != nil && len(c.pool.Accounts) > 1 {
		maxSwitches = len(c.pool.Accounts) - 1
		account, err := c.pool.Acquire(operation)
		if err != nil {
			return nil, err
		}
		if account == nil {
			return nil, errs.API(429, "All pooled accounts are cooling down for "+operation)
		}
		c.use(account)
	}
	attempt := 0
	for {
		headers := c.headers(rawURL, method)
		var reader *bytes.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reader = bytes.NewReader(raw)
		}
		var req *http.Request
		var err error
		if reader != nil {
			req, err = http.NewRequest(method, rawURL, reader)
		} else {
			req, err = http.NewRequest(method, rawURL, nil)
		}
		if err != nil {
			return nil, errs.Network(err.Error())
		}
		req.Header = headers
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt == 0 {
				if transport, nerr := newTransport(); nerr == nil {
					c.http = transport
				}
				attempt++
				continue
			}
			return nil, errs.Network(err.Error())
		}
		payload, err := readBody(resp)
		if err != nil {
			return nil, errs.Network(err.Error())
		}
		status := resp.StatusCode
		emptyForbidden := status == 403 && len(bytes.TrimSpace(payload)) == 0
		if emptyForbidden && attempt < 2 {
			if transport, nerr := newTransport(); nerr == nil {
				c.http = transport
			}
			attempt++
			continue
		}
		if (status == 401 || (status == 403 && !emptyForbidden)) && c.switchAccount(operation, "auth failure", accounts.AuthBlockUntil(c.now()), true, &switches, maxSwitches) {
			continue
		}
		if status == 429 && c.switchAccount(operation, "429", c.cooldown(resp.Header), false, &switches, maxSwitches) {
			continue
		}
		if status == 429 && attempt < c.rate.MaxRetries {
			c.backoff(attempt)
			attempt++
			continue
		}
		if status >= 400 {
			return nil, errs.API(status, clip(string(payload), 500))
		}
		if len(bytes.TrimSpace(payload)) == 0 {
			return map[string]any{}, nil
		}
		var parsed any
		if err := json.Unmarshal(payload, &parsed); err != nil {
			return nil, errs.API(status, "Twitter API returned non-JSON: "+clip(string(payload), 200))
		}
		obj, _ := parsed.(map[string]any)
		if obj == nil {
			if list, ok := parsed.([]any); ok {
				return map[string]any{"_list": list}, nil
			}
			return map[string]any{}, nil
		}
		if errors, ok := obj["errors"].([]any); ok && len(errors) > 0 {
			first, _ := errors[0].(map[string]any)
			message := jsonx.String(first["message"])
			if message == "" {
				message = "Unknown error"
			}
			code := jsonx.Int(first["code"], 0)
			if code == 88 && c.switchAccount(operation, "code 88", c.cooldown(resp.Header), false, &switches, maxSwitches) {
				continue
			}
			if code == 88 && attempt < c.rate.MaxRetries {
				c.backoff(attempt)
				attempt++
				continue
			}
			if code == 348 || code == 349 {
				return nil, errs.API(429, "Rate limited: "+message+" (try again later, recommended wait: 15+ minutes)")
			}
			return nil, errs.API(0, "Twitter API returned errors: "+message)
		}
		return obj, nil
	}
}

func (c *Client) switchAccount(operation, kind string, until float64, block bool, switches *int, maxSwitches int) bool {
	if c.pool == nil || c.current == nil || *switches >= maxSwitches {
		return false
	}
	current := *c.current
	var err error
	if block {
		err = c.pool.BlockAuth(current, until)
	} else {
		err = c.pool.Cool(current, operation, until)
	}
	if err != nil {
		log.Printf("account state update failed: %s", err)
		return false
	}
	next, err := c.pool.Acquire(operation)
	if err != nil || next == nil || next.Label == current.Label {
		return false
	}
	c.use(next)
	*switches++
	log.Printf("Switched to account %s after %s on %s", next.Label, kind, operation)
	return true
}

func (c *Client) use(account *accounts.Account) {
	c.current = account
	c.creds.AuthToken = account.AuthToken
	c.creds.CT0 = account.CT0
	c.creds.Cookie = ""
}

func (c *Client) cooldown(h http.Header) float64 {
	if reset, ok := resetUnix(h); ok {
		now := float64(c.now().Unix())
		if reset > now {
			return reset
		}
		return now + 1
	}
	return accounts.DefaultCooldownUntil(c.now())
}

func (c *Client) backoff(attempt int) {
	wait := c.rate.RetryBaseDelay * time.Duration(math.Pow(2, float64(attempt)))
	wait += time.Duration(rand.Float64() * float64(2*time.Second))
	log.Printf("Rate limited, retrying in %s (attempt %d/%d)", wait, attempt+1, c.rate.MaxRetries)
	c.sleep(wait)
}

func (c *Client) fetchText(rawURL string) (string, error) {
	// Homepage and script fetches stay off the API client. A failed
	// bootstrap otherwise poisons the connection used for GraphQL.
	transport, err := newBootstrap()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent())
	if strings.Contains(rawURL, "://x.com") || strings.Contains(rawURL, "://twitter.com") {
		if cookie := c.cookieHeader(); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	resp, err := transport.Do(req)
	if err != nil {
		return "", err
	}
	body, err := readBody(resp)
	return string(body), err
}

func clip(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}

func (c *Client) pageDelay() {
	if c.rate.RequestDelay <= 0 {
		return
	}
	factor := 0.7 + rand.Float64()*0.8
	c.sleep(time.Duration(float64(c.rate.RequestDelay) * factor))
}

func (c *Client) writeDelay() {
	c.sleep(time.Duration(1500+rand.Intn(2500)) * time.Millisecond)
}

func minCount(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsDigitID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func trimAt(value string) string { return strings.TrimPrefix(strings.TrimSpace(value), "@") }
