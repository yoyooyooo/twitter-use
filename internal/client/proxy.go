package client

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	http "github.com/bogdanfinn/fhttp"
	"golang.org/x/net/http/httpproxy"
)

type proxyResolver func(*url.URL) (*url.URL, error)

// Snapshot the environment once per transport, not in a package-global cache.
// An explicit TWITTER_PROXY, including an empty value for direct access, wins.
func proxyFromEnvironment() (proxyResolver, error) {
	if raw, set := os.LookupEnv("TWITTER_PROXY"); set {
		proxy, err := parseProxy(raw, false)
		if err != nil {
			return nil, errors.New("invalid proxy URL in TWITTER_PROXY")
		}
		return func(*url.URL) (*url.URL, error) { return proxy, nil }, nil
	}
	cfg := httpproxy.FromEnvironment()
	for _, setting := range []struct {
		name  string
		value *string
	}{
		{"HTTP_PROXY/http_proxy", &cfg.HTTPProxy},
		{"HTTPS_PROXY/https_proxy", &cfg.HTTPSProxy},
	} {
		proxy, err := parseProxy(*setting.value, true)
		if err != nil {
			return nil, errors.New("invalid proxy URL in " + setting.name)
		}
		if proxy != nil {
			*setting.value = proxy.String()
		}
	}
	return cfg.ProxyFunc(), nil
}

func parseProxy(raw string, allowHost bool) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if allowHost && !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	proxy, err := url.Parse(raw)
	if err != nil || proxy.Hostname() == "" || proxy.RawQuery != "" || proxy.Fragment != "" || (proxy.Path != "" && proxy.Path != "/") {
		return nil, errors.New("invalid proxy URL")
	}
	switch proxy.Scheme {
	case "http", "https", "socks4", "socks4a", "socks5", "socks5h":
	default:
		return nil, errors.New("unsupported proxy scheme")
	}
	if port := proxy.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid proxy port")
		}
	}
	return proxy, nil
}

// Keep separate clients for direct and proxied routes. Mutating a shared
// tls-client's proxy would race concurrent requests and redirect destinations.
type proxyTransport struct {
	resolve proxyResolver
	create  func(string) (Doer, error)
	mu      sync.Mutex
	clients map[string]Doer
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	proxy, err := t.resolve(req.URL)
	if err != nil {
		return nil, err
	}
	key := ""
	if proxy != nil {
		key = proxy.String()
	}
	t.mu.Lock()
	client := t.clients[key]
	if client == nil {
		client, err = t.create(key)
		if err == nil {
			if t.clients == nil {
				t.clients = map[string]Doer{}
			}
			t.clients[key] = client
		}
	}
	t.mu.Unlock()
	if err != nil {
		return nil, safeProxyError(err, proxy)
	}
	resp, err := client.Do(req)
	return resp, safeProxyError(err, proxy)
}

func safeProxyError(err error, proxy *url.URL) error {
	if err == nil || proxy == nil || proxy.User == nil {
		return err
	}
	password, _ := proxy.User.Password()
	values := []string{proxy.String(), proxy.User.String(), proxy.User.Username(), password, url.QueryEscape(password), url.PathEscape(password)}
	return errors.New(redactValues(err.Error(), values))
}
