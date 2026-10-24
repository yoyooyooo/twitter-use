package client

import (
	"log"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/graphql"
	"github.com/yoyooyooo/twitter-use/internal/xid"
)

func (c *Client) cookieHeader() string {
	if c.creds.Cookie != "" {
		return c.creds.Cookie
	}
	if c.creds.AuthToken == "" || c.creds.CT0 == "" {
		return ""
	}
	return "auth_token=" + c.creds.AuthToken + "; ct0=" + c.creds.CT0
}

func (c *Client) headers(rawURL, method string) http.Header {
	cookie := c.cookieHeader()
	h := http.Header{
		"Authorization":             {"Bearer " + bearer},
		"Cookie":                    {cookie},
		"X-Csrf-Token":              {c.creds.CT0},
		"X-Twitter-Active-User":     {"yes"},
		"X-Twitter-Auth-Type":       {"OAuth2Session"},
		"X-Twitter-Client-Language": {clientLanguage()},
		"User-Agent":                {userAgent()},
		"Origin":                    {"https://x.com"},
		"Referer":                   {"https://x.com/"},
		"Accept":                    {"*/*"},
	}
	order := []string{
		"authorization", "cookie", "x-csrf-token", "x-twitter-active-user", "x-twitter-auth-type",
		"x-twitter-client-language", "user-agent", "origin", "referer", "accept",
	}
	if method == http.MethodPost {
		h.Set("Content-Type", "application/json")
		order = append([]string{"authorization", "cookie", "x-csrf-token", "content-type"}, order[3:]...)
	}
	if c.transaction != nil && rawURL != "" {
		path := parseURL(rawURL).Path
		if id := c.transaction.ID(method, path); id != "" {
			h.Set("X-Client-Transaction-Id", id)
			order = append(order, "x-client-transaction-id")
		}
	}
	h[http.HeaderOrderKey] = order
	h[http.PHeaderOrderKey] = []string{":method", ":authority", ":scheme", ":path"}
	return h
}

func (c *Client) ensureTransaction() {
	if c.triedTx {
		return
	}
	c.triedTx = true
	// https://x.com without a session is the logged-out shell and no longer
	// embeds the webpack map. /explore with the account cookie still does.
	// The first connection to x.com is often reset, so try twice.
	page, err := c.fetchText("https://x.com/explore")
	if err != nil {
		page, err = c.fetchText("https://x.com/explore")
	}
	if err != nil {
		log.Printf("Failed to init ClientTransaction: %s", err)
		return
	}
	if migrated := xid.MigrationURL(page); migrated != "" {
		if next, err := c.fetchText(migrated); err == nil && next != "" {
			page = next
		}
	}
	graphql.UpdateFeatures(page)
	ondemandURL := xid.OnDemandURL(page)
	if ondemandURL == "" {
		log.Printf("Failed to init ClientTransaction: ondemand file URL missing")
		return
	}
	script, err := c.fetchText(ondemandURL)
	if err != nil {
		log.Printf("Failed to init ClientTransaction: %s", err)
		return
	}
	gen, err := xid.New(page, script)
	if err != nil {
		log.Printf("Failed to init ClientTransaction: %s", err)
		return
	}
	c.transaction = gen
	log.Printf("ClientTransaction initialized")
}

func formHeaders(base http.Header) http.Header {
	base.Set("Content-Type", "application/x-www-form-urlencoded")
	order := append([]string{}, base[http.HeaderOrderKey]...)
	order = append(order, "content-type")
	base[http.HeaderOrderKey] = order
	return base
}
