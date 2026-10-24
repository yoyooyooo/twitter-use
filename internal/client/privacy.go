package client

import (
	"errors"
	"net/url"
	"sort"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/errs"
)

// Credentials belong only on the first-party HTTPS endpoints, not on script
// hosts or URLs that merely mention x.com in their path, query, or userinfo.
func credentialURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "x.com", "twitter.com":
		return true
	}
	return false
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if !credentialURL(req.URL) {
		req.Header.Del("Cookie")
		req.Header.Del("Authorization")
		req.Header.Del("X-Csrf-Token")
	}
	return nil
}

func (c *Client) redact(text string) string {
	values := []string{c.creds.AuthToken, c.creds.CT0}
	for _, part := range strings.Split(c.creds.Cookie, ";") {
		if _, value, ok := strings.Cut(part, "="); ok {
			value = strings.TrimSpace(value)
			values = append(values, value)
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				values = append(values, value[1:len(value)-1])
			}
		}
	}
	if c.pool != nil {
		for _, account := range c.pool.Accounts {
			values = append(values, account.AuthToken, account.CT0)
		}
	}
	return redactValues(text, values)
}

func redactValues(text string, values []string) string {
	// Replace longer values first so overlapping values cannot expose a suffix.
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	var pairs []string
	for _, value := range values {
		if value != "" {
			pairs = append(pairs, value, "[REDACTED]")
		}
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

func (c *Client) redactError(err error) error {
	if err == nil {
		return nil
	}
	if typed, ok := err.(*errs.Error); ok {
		return &errs.Error{Code: typed.Code, Status: typed.Status, Msg: c.redact(typed.Msg)}
	}
	if safe := c.redact(err.Error()); safe != err.Error() {
		return errors.New(safe)
	}
	return err
}
