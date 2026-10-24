// Package accounts selects a sticky sub-account and persists cooldowns.
// The state file stores SHA-256 labels only. Tokens never enter it.
package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yoyooyooo/twitter-use/internal/errs"
)

const (
	FileEnv  = "TWITTER_ACCOUNTS_FILE"
	StateEnv = "TWITTER_ACCOUNT_STATE"

	defaultCooldown = 15 * time.Minute
	authBlock       = time.Hour
)

// Account is one auth_token/ct0 pair. Label is safe to log.
type Account struct {
	AuthToken string
	CT0       string
	Label     string
}

// Label returns the first 12 hex chars of SHA-256(auth_token).
func Label(authToken string) string {
	sum := sha256.Sum256([]byte(authToken))
	return hex.EncodeToString(sum[:])[:12]
}

// OperationKey is the GraphQL operation name, which is the quota bucket.
func OperationKey(rawURL string) string {
	path := rawURL
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	parts := strings.Split(path, "/")
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	for i, part := range kept {
		if part == "graphql" && i+2 < len(kept) {
			return kept[i+2]
		}
	}
	return path
}

// ParseAccounts reads "auth_token ct0" lines. Blank lines and # comments are ignored.
func ParseAccounts(text string) ([]Account, error) {
	var out []Account
	seen := map[string]struct{}{}
	for lineno, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, errs.Auth(fmt.Sprintf("Invalid account line %d: expected 'auth_token ct0'", lineno+1))
		}
		if _, ok := seen[parts[0]]; ok {
			return nil, errs.Auth(fmt.Sprintf("Duplicate auth token on line %d", lineno+1))
		}
		seen[parts[0]] = struct{}{}
		out = append(out, Account{AuthToken: parts[0], CT0: parts[1], Label: Label(parts[0])})
	}
	return out, nil
}

// ConfigDir is ~/.twitter-use. Account files stay outside the repository.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".twitter-use")
}

// DefaultAccountsFile is ~/.twitter-use/accounts.txt.
func DefaultAccountsFile() string {
	return filepath.Join(ConfigDir(), "accounts.txt")
}

// DefaultStateFile is ~/.twitter-use/account_pool.json.
func DefaultStateFile() string {
	return filepath.Join(ConfigDir(), "account_pool.json")
}

// ResolveAccountsFile returns the pool file, or empty when public reads use one login.
// An explicit TWITTER_ACCOUNTS_FILE wins even when that path is missing.
func ResolveAccountsFile() string {
	if raw := strings.TrimSpace(os.Getenv(FileEnv)); raw != "" {
		return expand(raw)
	}
	path := DefaultAccountsFile()
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

// Configured reports whether public reads must use the pool.
func Configured() bool { return ResolveAccountsFile() != "" }

// LoadFromEnv loads the configured pool. A configured but unusable file is an error.
func LoadFromEnv() (*Pool, error) {
	path := ResolveAccountsFile()
	if path == "" {
		return nil, errs.Auth(fmt.Sprintf("No account pool file. Add %s or set %s.", DefaultAccountsFile(), FileEnv))
	}
	text, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errs.Auth(fmt.Sprintf("%s does not exist: %s", FileEnv, path))
		}
		return nil, errs.Auth(fmt.Sprintf("Cannot read %s: %s", path, err))
	}
	list, err := ParseAccounts(string(text))
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errs.Auth("Account pool has no accounts: " + path)
	}
	state := DefaultStateFile()
	if raw := strings.TrimSpace(os.Getenv(StateEnv)); raw != "" {
		state = expand(raw)
	}
	log.Printf("Loaded %d pooled accounts from %s", len(list), path)
	return New(list, state)
}

// Pool is sticky selection with a cross-process cooldown file.
type Pool struct {
	Accounts []Account
	State    string
	byLabel  map[string]Account
	sticky   string
}

func New(list []Account, statePath string) (*Pool, error) {
	if len(list) == 0 {
		return nil, errs.Auth("Account pool is empty")
	}
	by := make(map[string]Account, len(list))
	for _, account := range list {
		by[account.Label] = account
	}
	return &Pool{Accounts: list, State: statePath, byLabel: by}, nil
}

// Acquire returns the sticky account, or the least recently used usable one.
func (p *Pool) Acquire(operation string) (*Account, error) {
	now := float64(time.Now().UnixNano()) / 1e9
	var chosen *Account
	err := p.withState(func(state map[string]any) error {
		if sticky, ok := p.byLabel[p.sticky]; ok && usable(state, sticky, operation, now) {
			touch(state, sticky, now)
			copy := sticky
			chosen = &copy
			return nil
		}
		p.sticky = ""
		var best *Account
		var bestUsed float64
		for i := range p.Accounts {
			account := p.Accounts[i]
			if !usable(state, account, operation, now) {
				continue
			}
			used := lastUsed(state, account)
			if best == nil || used < bestUsed {
				copy := account
				best = &copy
				bestUsed = used
			}
		}
		if best == nil {
			return nil
		}
		p.sticky = best.Label
		touch(state, *best, now)
		chosen = best
		return nil
	})
	return chosen, err
}

// Cool keeps this account off one operation until until (unix seconds).
func (p *Pool) Cool(account Account, operation string, until float64) error {
	err := p.withState(func(state map[string]any) error {
		entry := entry(state, account)
		cd, _ := entry["cooldown"].(map[string]any)
		if cd == nil {
			cd = map[string]any{}
			entry["cooldown"] = cd
		}
		cd[operation] = until
		if p.sticky == account.Label {
			p.sticky = ""
		}
		return nil
	})
	if err == nil {
		log.Printf("Account %s cooling down for %s until %d", account.Label, operation, int64(until))
	}
	return err
}

// BlockAuth sidelines an account after 401/403 without storing its token.
func (p *Pool) BlockAuth(account Account, until float64) error {
	err := p.withState(func(state map[string]any) error {
		entry(state, account)["auth_blocked_until"] = until
		if p.sticky == account.Label {
			p.sticky = ""
		}
		return nil
	})
	if err == nil {
		log.Printf("Account %s blocked after auth failure until %d", account.Label, int64(until))
	}
	return err
}

func (p *Pool) withState(fn func(map[string]any) error) error {
	if err := os.MkdirAll(filepath.Dir(p.State), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p.State, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = os.Chmod(p.State, 0o600)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	raw, err := os.ReadFile(p.State)
	if err != nil {
		return err
	}
	state := map[string]any{"accounts": map[string]any{}}
	if strings.TrimSpace(string(raw)) != "" {
		var parsed map[string]any
		if json.Unmarshal(raw, &parsed) == nil {
			if accounts, ok := parsed["accounts"].(map[string]any); ok {
				parsed["accounts"] = accounts
				state = parsed
			}
		} else {
			log.Printf("Ignoring unreadable account pool state %s", p.State)
		}
	}
	if err := fn(state); err != nil {
		return err
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := f.Write(encoded); err != nil {
		return err
	}
	return f.Sync()
}

func entry(state map[string]any, account Account) map[string]any {
	accounts, _ := state["accounts"].(map[string]any)
	if accounts == nil {
		accounts = map[string]any{}
		state["accounts"] = accounts
	}
	item, _ := accounts[account.Label].(map[string]any)
	if item == nil {
		item = map[string]any{}
		accounts[account.Label] = item
	}
	return item
}

func touch(state map[string]any, account Account, now float64) {
	entry(state, account)["last_used"] = now
}

func lastUsed(state map[string]any, account Account) float64 {
	accounts, _ := state["accounts"].(map[string]any)
	item, _ := accounts[account.Label].(map[string]any)
	return asFloat(item["last_used"])
}

func usable(state map[string]any, account Account, operation string, now float64) bool {
	accounts, _ := state["accounts"].(map[string]any)
	item, ok := accounts[account.Label].(map[string]any)
	if !ok {
		return true
	}
	if asFloat(item["auth_blocked_until"]) > now {
		return false
	}
	cd, ok := item["cooldown"].(map[string]any)
	if !ok {
		return true
	}
	return asFloat(cd[operation]) <= now
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

// DefaultCooldownUntil is the 15-minute window used when Twitter omits the reset header.
func DefaultCooldownUntil(now time.Time) float64 {
	if now.IsZero() {
		now = time.Now()
	}
	return float64(now.Add(defaultCooldown).Unix())
}

// AuthBlockUntil is how long a rejected sub-account stays out of the pool.
func AuthBlockUntil(now time.Time) float64 {
	if now.IsZero() {
		now = time.Now()
	}
	return float64(now.Add(authBlock).Unix())
}

func expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}
