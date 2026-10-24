package client

import (
	"fmt"
	"io"
	"strings"
	"testing"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/accounts"
	"github.com/yoyooyooo/twitter-use/internal/errs"
)

func TestBootstrapCookiesStayOnHTTPSFirstPartyHosts(t *testing.T) {
	c := &Client{creds: Credentials{AuthToken: "synthetic-auth", CT0: "synthetic-csrf"}}
	for _, test := range []struct {
		url  string
		want bool
	}{
		{"https://x.com/explore", true},
		{"https://twitter.com/x/migrate?tok=example", true},
		{"https://X.COM:443/explore", true},
		{"http://x.com/x/migrate?tok=example", false},
		{"https://x.com.example.test/explore", false},
		{"https://x.com@example.test/explore", false},
		{"https://abs.twimg.com/responsive-web/client-web/https://x.com.js", false},
		{"https://example.test/?url=https://twitter.com", false},
		{"https://x.com:8443/explore", false},
		{"https://abs.twimg.com/responsive-web/client-web/main.js", false},
	} {
		t.Run(test.url, func(t *testing.T) {
			req, err := c.bootstrapRequest(test.url)
			if err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get("Cookie") != ""; got != test.want {
				t.Fatalf("cookie attached = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAPIErrorsRedactCredentials(t *testing.T) {
	const token, ct0, extra = "synthetic-auth-secret", "synthetic-csrf-secret", "synthetic-browser-secret"
	cookie := fmt.Sprintf("auth_token=%s; ct0=%s; personalization_id=%q", token, ct0, extra)
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{"http error", 500, cookie},
		{"JSON http error", 500, fmt.Sprintf(`{"message":%q}`, cookie)},
		{"auth error", 403, cookie},
		{"non-JSON", 200, cookie},
		{"GraphQL error", 200, fmt.Sprintf(`{"errors":[{"message":%q}]}`, cookie)},
		{"clipped error", 500, strings.Repeat("x", 490) + token},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := &scripted{calls: []script{{status: test.status, body: test.body}}}
			api, err := New(Credentials{AuthToken: token, CT0: ct0, Cookie: cookie}, nil, Rate{}, WithHTTP(transport), WithoutTransaction())
			if err != nil {
				t.Fatal(err)
			}
			_, err = api.api("https://x.com/i/api/graphql/example/TweetDetail", http.MethodGet, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if typed, ok := err.(*errs.Error); !ok || (test.status == 403 && typed.Code != "not_authenticated") {
				t.Fatalf("error classification changed: %T %v", err, err)
			}
			if strings.Contains(err.Error(), "synthetic-") {
				t.Fatalf("credential or prefix exposed: %s", err)
			}
			if !strings.Contains(err.Error(), "[REDACTED]") {
				t.Fatalf("missing redaction: %s", err)
			}
		})
	}
}

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRedactionCoversPoolAndOverlappingValues(t *testing.T) {
	c := &Client{
		creds: Credentials{AuthToken: "synthetic", CT0: "synthetic-long"},
		pool:  &accounts.Pool{Accounts: []accounts.Account{{AuthToken: "pooled-token", CT0: "pooled-csrf"}}},
	}
	err := c.redactError(errs.API(429, "synthetic-long pooled-token pooled-csrf"))
	if got, want := err.Error(), "Twitter API error (HTTP 429): [REDACTED] [REDACTED] [REDACTED]"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if errs.Code(err) != "rate_limited" {
		t.Fatalf("error classification = %q", errs.Code(err))
	}
}

func TestRedirectCredentials(t *testing.T) {
	for _, test := range []struct {
		url  string
		want bool
	}{
		{"https://x.com/next", true},
		{"http://x.com/next", false},
		{"https://example.test/next", false},
		{"https://subdomain.x.com/next", false},
	} {
		t.Run(test.url, func(t *testing.T) {
			calls := 0
			transport := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": {test.url}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
				}
				for _, name := range []string{"Cookie", "Authorization", "X-Csrf-Token"} {
					if got := req.Header.Get(name) != ""; got != test.want {
						t.Errorf("%s attached = %v, want %v", name, got, test.want)
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
			})
			client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
			req, err := http.NewRequest(http.MethodGet, "https://x.com/start", nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"Cookie", "Authorization", "X-Csrf-Token"} {
				req.Header.Set(name, "synthetic-secret")
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if calls != 2 {
				t.Fatalf("requests = %d, want 2", calls)
			}
		})
	}
}
