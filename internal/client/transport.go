package client

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	bearer = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
	chrome = "133"
)

// Doer is the HTTP client used for X. Tests substitute a fake.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func newTransport() (Doer, error) { return buildTransport(30) }

func newBootstrap() (Doer, error) { return buildTransport(8) }

func buildTransport(timeout int) (Doer, error) {
	opts := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(timeout),
		tls_client.WithClientProfile(profiles.Chrome_133),
		tls_client.WithDisableHttp3(),
		tls_client.WithCookieJar(nil),
	}
	if proxy := strings.TrimSpace(os.Getenv("TWITTER_PROXY")); proxy != "" {
		opts = append(opts, tls_client.WithProxyUrl(proxy))
	}
	httpClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return nil, err
	}
	log.Printf("tls-client impersonating chrome%s", chrome)
	return httpClient, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}

func userAgent() string {
	platform := "X11; Linux x86_64"
	switch runtime.GOOS {
	case "darwin":
		platform = "Macintosh; Intel Mac OS X 10_15_7"
	case "windows":
		platform = "Windows NT 10.0; Win64; x64"
	}
	return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", platform, chrome)
}

func secCHUA() string {
	return fmt.Sprintf(`"Chromium";v="%s", "Not(A:Brand";v="99", "Google Chrome";v="%s"`, chrome, chrome)
}

func acceptLanguage() string {
	raw := os.Getenv("LC_ALL")
	if raw == "" {
		raw = os.Getenv("LC_MESSAGES")
	}
	if raw == "" {
		raw = os.Getenv("LANG")
	}
	if raw == "" {
		raw = "en_US.UTF-8"
	}
	tag := strings.ReplaceAll(strings.SplitN(raw, ".", 2)[0], "_", "-")
	if tag == "" {
		tag = "en-US"
	}
	language := strings.SplitN(tag, "-", 2)[0]
	return tag + "," + language + ";q=0.9,en;q=0.8"
}

func clientLanguage() string {
	tag := strings.SplitN(acceptLanguage(), ",", 2)[0]
	language := strings.SplitN(tag, "-", 2)[0]
	if language == "" {
		return "en"
	}
	return language
}

func platformName() string {
	switch runtime.GOOS {
	case "darwin":
		return `"macOS"`
	case "windows":
		return `"Windows"`
	default:
		return `"Linux"`
	}
}

func archName() string {
	switch runtime.GOARCH {
	case "arm", "arm64":
		return `"arm"`
	case "amd64", "386":
		return `"x86"`
	default:
		return `""`
	}
}

func platformVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return `"15.0.0"`
	case "windows":
		return `"10.0.0"`
	default:
		return `""`
	}
}

func headerValue(h http.Header, key string) string {
	if h == nil {
		return ""
	}
	if v := h.Get(key); v != "" {
		return v
	}
	return h.Get(strings.ToLower(key))
}

func resetUnix(h http.Header) (float64, bool) {
	raw := headerValue(h, "x-rate-limit-reset")
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return parsed
}
