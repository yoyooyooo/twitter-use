package client

import (
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
)

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TWITTER_PROXY", "HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy", "REQUEST_METHOD"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProxyEnvironment(t *testing.T) {
	for _, test := range []struct {
		name, target, want string
		env                map[string]string
	}{
		{"direct", "https://x.com", "", nil},
		{"HTTPS", "https://x.com", "http://secure.example:8080", map[string]string{"HTTPS_PROXY": "http://secure.example:8080"}},
		{"HTTP", "http://example.test", "http://plain.example:8080", map[string]string{"HTTP_PROXY": "http://plain.example:8080"}},
		{"lowercase", "https://x.com", "http://lower.example:8080", map[string]string{"https_proxy": "http://lower.example:8080"}},
		{"scheme specific", "https://x.com", "", map[string]string{"HTTP_PROXY": "http://plain.example:8080"}},
		{"host port", "https://x.com", "http://proxy.example:8080", map[string]string{"HTTPS_PROXY": "proxy.example:8080"}},
		{"NO_PROXY", "https://x.com", "", map[string]string{"HTTPS_PROXY": "http://proxy.example", "NO_PROXY": "x.com"}},
		{"lowercase bypass", "https://sub.example.test", "", map[string]string{"https_proxy": "http://proxy.example", "no_proxy": ".example.test"}},
		{"CIDR bypass", "https://192.0.2.8", "", map[string]string{"HTTPS_PROXY": "http://proxy.example", "NO_PROXY": "192.0.2.0/24"}},
		{"wildcard", "https://x.com", "", map[string]string{"HTTPS_PROXY": "http://proxy.example", "NO_PROXY": "*"}},
		{"unmatched bypass", "https://x.com", "http://proxy.example", map[string]string{"HTTPS_PROXY": "http://proxy.example", "NO_PROXY": "example.test"}},
		{"explicit wins", "https://x.com", "socks5h://chosen.example:1080", map[string]string{"TWITTER_PROXY": "socks5h://chosen.example:1080", "HTTPS_PROXY": "http://proxy.example", "NO_PROXY": "*"}},
		{"explicit direct", "https://x.com", "", map[string]string{"TWITTER_PROXY": "", "HTTPS_PROXY": "http://proxy.example"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearProxyEnv(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			resolve, err := proxyFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(test.target)
			proxy, err := resolve(u)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if proxy != nil {
				got = proxy.String()
			}
			if got != test.want {
				t.Fatalf("proxy = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInvalidProxyDoesNotExposeCredentials(t *testing.T) {
	for _, key := range []string{"TWITTER_PROXY", "HTTPS_PROXY"} {
		t.Run(key, func(t *testing.T) {
			clearProxyEnv(t)
			t.Setenv(key, "http://synthetic-user:synthetic-password@proxy.example:bad")
			_, err := proxyFromEnvironment()
			if err == nil || !strings.Contains(err.Error(), "invalid proxy") || strings.Contains(err.Error(), "synthetic") {
				t.Fatalf("unsafe or missing configuration error: %v", err)
			}
		})
	}
	proxy, _ := url.Parse("http://synthetic-user:synthetic-password@proxy.example:8080")
	err := safeProxyError(errors.New("proxy "+proxy.String()+" rejected synthetic-password"), proxy)
	if strings.Contains(err.Error(), "synthetic") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("unsafe runtime proxy error: %v", err)
	}
}

type proxyDoer func(*http.Request) (*http.Response, error)

func (f proxyDoer) Do(req *http.Request) (*http.Response, error) { return f(req) }

func proxyResponse(req *http.Request, status int, header http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("ok")), Request: req}
}

func TestRedirectResolvesProxyAndPreservesBody(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://proxy.example:8080")
	t.Setenv("NO_PROXY", "direct.example")
	resolve, err := proxyFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	var routes []string
	transport := &proxyTransport{resolve: resolve, create: func(route string) (Doer, error) {
		return proxyDoer(func(req *http.Request) (*http.Response, error) {
			routes = append(routes, route)
			body, _ := io.ReadAll(req.Body)
			if req.Method != http.MethodPost || string(body) != "readonly-query" {
				t.Errorf("redirect changed method/body: %s %q", req.Method, body)
			}
			if req.URL.Host == "x.com" {
				return proxyResponse(req, 307, http.Header{"Location": {"https://direct.example/next"}}), nil
			}
			for _, key := range []string{"Cookie", "Authorization", "X-Csrf-Token"} {
				if req.Header.Get(key) != "" {
					t.Errorf("redirect retained %s", key)
				}
			}
			return proxyResponse(req, 200, http.Header{}), nil
		}), nil
	}}
	client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
	req, _ := http.NewRequest(http.MethodPost, "https://x.com/start", strings.NewReader("readonly-query"))
	for _, key := range []string{"Cookie", "Authorization", "X-Csrf-Token"} {
		req.Header.Set(key, "synthetic-secret")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(routes) != 2 || routes[0] != "http://proxy.example:8080" || routes[1] != "" {
		t.Fatalf("redirect routes = %v", routes)
	}
}

func TestConcurrentProxyRoutesReuseSeparateClients(t *testing.T) {
	proxy, _ := url.Parse("http://proxy.example:8080")
	created := map[string]int{}
	transport := &proxyTransport{
		resolve: func(u *url.URL) (*url.URL, error) {
			if u.Host == "direct.example" {
				return nil, nil
			}
			return proxy, nil
		},
		create: func(route string) (Doer, error) {
			created[route]++
			return proxyDoer(func(req *http.Request) (*http.Response, error) {
				if (req.URL.Host == "direct.example") != (route == "") {
					t.Errorf("request used wrong proxy route")
				}
				return proxyResponse(req, 200, http.Header{}), nil
			}), nil
		},
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			host := "proxied.example"
			if i%2 == 0 {
				host = "direct.example"
			}
			req, _ := http.NewRequest(http.MethodGet, "https://"+host, nil)
			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
	if len(created) != 2 || created[""] != 1 || created[proxy.String()] != 1 {
		t.Fatalf("created clients = %v", created)
	}
}

func TestProxyTransportTimeoutAndRedirectOwnership(t *testing.T) {
	clearProxyEnv(t)
	transport, err := buildTransport(8)
	if err != nil {
		t.Fatal(err)
	}
	outer := transport.(*http.Client)
	if outer.Timeout != 8*time.Second || outer.CheckRedirect == nil {
		t.Fatal("outer timeout or redirect policy missing")
	}
	inner, err := buildProxyClient(8, "")
	if err != nil {
		t.Fatal(err)
	}
	if inner.(tlsclient.HttpClient).GetFollowRedirect() {
		t.Fatal("inner client must not follow redirects")
	}
	outer.Timeout = 10 * time.Millisecond
	outer.Transport.(*proxyTransport).create = func(string) (Doer, error) {
		return proxyDoer(func(req *http.Request) (*http.Response, error) {
			if _, ok := req.Context().Deadline(); !ok {
				return nil, errors.New("outer deadline missing")
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		}), nil
	}
	req, _ := http.NewRequest(http.MethodGet, "https://x.com", nil)
	if _, err := outer.Do(req); err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("timeout = %v", err)
	}
}
