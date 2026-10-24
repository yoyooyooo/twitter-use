// Package auth loads the primary login from the environment or a local browser.
// The account pool never calls this package.
package auth

import (
	"log"
	"os"
	"strings"

	"github.com/yoyooyooo/twitter-use/internal/client"
	"github.com/yoyooyooo/twitter-use/internal/errs"
)

// Session is the primary login.
type Session struct {
	AuthToken string
	CT0       string
	Cookie    string
}

// FromEnv reports the login from TWITTER_AUTH_TOKEN and TWITTER_CT0.
func FromEnv() (Session, bool) { return fromEnv() }

// BrowserSessions returns every same-host auth_token and ct0 pair in local browsers.
func BrowserSessions() []Session {
	sessions, _, _ := browserSessions()
	return sessions
}

// Load returns the primary login. Environment variables win over the browser.
func Load() (Session, error) {
	if session, ok := fromEnv(); ok {
		log.Printf("Loaded cookies from environment variables")
		return session, nil
	}
	session, diagnostics, err := fromBrowser()
	if err != nil {
		return Session{}, err
	}
	if session.AuthToken == "" || session.CT0 == "" {
		return Session{}, errs.Auth(notFound(diagnostics))
	}
	return session, nil
}

// Credentials converts a session for the API client.
func (s Session) Credentials() client.Credentials {
	return client.Credentials{AuthToken: s.AuthToken, CT0: s.CT0, Cookie: s.Cookie}
}

func fromEnv() (Session, bool) {
	token := os.Getenv("TWITTER_AUTH_TOKEN")
	ct0 := os.Getenv("TWITTER_CT0")
	if token != "" && ct0 != "" {
		return Session{AuthToken: token, CT0: ct0}, true
	}
	return Session{}, false
}

func notFound(diagnostics []string) string {
	lines := []string{"No Twitter cookies found."}
	if hint := keychainHint(diagnostics); hint != "" {
		lines = append(lines, "", "Likely cause:", hint, "")
	}
	lines = append(lines,
		"Option 1: Set TWITTER_AUTH_TOKEN and TWITTER_CT0 environment variables",
		"Option 2: Make sure you are logged into x.com in your browser (Arc/Chrome/Edge/Firefox/Brave)",
	)
	return strings.Join(lines, "\n")
}

func keychainHint(diagnostics []string) string {
	text := strings.ToLower(strings.Join(diagnostics, " "))
	if !strings.Contains(text, "keychain") && !strings.Contains(text, "safe storage") && !strings.Contains(text, "secret") {
		return ""
	}
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return "macOS Keychain is locked (SSH session detected).\n  Fix: security unlock-keychain ~/Library/Keychains/login.keychain-db"
	}
	return "The browser cookie key could not be read from the system keychain.\n  Approve the Keychain prompt, or set TWITTER_AUTH_TOKEN and TWITTER_CT0."
}

func browserOrder() []string {
	all := []string{"arc", "chrome", "edge", "firefox", "brave"}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("TWITTER_BROWSER")))
	if env == "" {
		return all
	}
	for _, name := range all {
		if name == env {
			return append([]string{env}, without(all, env)...)
		}
	}
	log.Printf("TWITTER_BROWSER=%q is invalid, using default order", env)
	return all
}

func without(items []string, skip string) []string {
	var out []string
	for _, item := range items {
		if item != skip {
			out = append(out, item)
		}
	}
	return out
}

func joinCookies(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for key, value := range values {
		if key == "" || value == "" || key == "auth_token" || key == "ct0" {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	// auth_token and ct0 stay first so the header matches the explicit pair.
	head := []string{"auth_token=" + values["auth_token"], "ct0=" + values["ct0"]}
	return strings.Join(append(head, parts...), "; ")
}

func sessionFrom(values map[string]string) Session {
	return Session{AuthToken: values["auth_token"], CT0: values["ct0"], Cookie: joinCookies(values)}
}
