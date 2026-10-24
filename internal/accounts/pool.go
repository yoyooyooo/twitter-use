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
// Alias, Primary, and Pool come from accounts.json. A legacy text line is pool-only.
type Account struct {
	AuthToken string
	CT0       string
	Label     string
	Alias     string
	Primary   bool
	Pool      bool
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
		out = append(out, Account{AuthToken: parts[0], CT0: parts[1], Label: Label(parts[0]), Pool: true})
	}
	return out, nil
}

// ConfigDir is ~/.twitter-use. Account files stay outside the repository.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".twitter-use")
}

// DefaultAccountsFile is ~/.twitter-use/accounts.json.
func DefaultAccountsFile() string {
	return filepath.Join(ConfigDir(), "accounts.json")
}

func legacyAccountsFile() string {
	return filepath.Join(ConfigDir(), "accounts.txt")
}

// DefaultStateFile is ~/.twitter-use/account_pool.json.
func DefaultStateFile() string {
	return filepath.Join(ConfigDir(), "account_pool.json")
}

// ResolveAccountsFile returns the account file, or empty when public reads use one login.
// An explicit TWITTER_ACCOUNTS_FILE wins even when that path is missing.
// Without it, accounts.json wins over a legacy accounts.txt.
func ResolveAccountsFile() string {
	if raw := strings.TrimSpace(os.Getenv(FileEnv)); raw != "" {
		return expand(raw)
	}
	if fileExists(DefaultAccountsFile()) {
		return DefaultAccountsFile()
	}
	if fileExists(legacyAccountsFile()) {
		return legacyAccountsFile()
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
	roster, err := parseRoster(path, text)
	if err != nil {
		return nil, err
	}
	pool, err := roster.Pool()
	if err != nil {
		return nil, err
	}
	log.Printf("Loaded %d pooled accounts from %s", len(pool.Accounts), path)
	return pool, nil
}

// LoadRoster reads every account. The pool, when present, contains only pool members.
func LoadRoster() (*Roster, error) {
	path := ResolveAccountsFile()
	if path == "" {
		return nil, errs.Auth(fmt.Sprintf("No account file. Add %s or set %s.", DefaultAccountsFile(), FileEnv))
	}
	text, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errs.Auth(fmt.Sprintf("%s does not exist: %s", FileEnv, path))
		}
		return nil, errs.Auth(fmt.Sprintf("Cannot read %s: %s", path, err))
	}
	roster, err := parseRoster(path, text)
	if err != nil {
		return nil, err
	}
	pooled := 0
	for _, account := range roster.Accounts {
		if account.Pool {
			pooled++
		}
	}
	log.Printf("Loaded %d accounts (%d pooled) from %s", len(roster.Accounts), pooled, path)
	return roster, nil
}

func parseRoster(path string, text []byte) (*Roster, error) {
	list, err := ParseConfig(string(text))
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errs.Auth("Account file has no accounts: " + path)
	}
	state := DefaultStateFile()
	if raw := strings.TrimSpace(os.Getenv(StateEnv)); raw != "" {
		state = expand(raw)
	}
	return &Roster{Accounts: list, State: state}, nil
}

// ParseConfig reads accounts.json, or legacy "auth_token ct0" lines when the text is not JSON.
func ParseConfig(text string) ([]Account, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, errs.Auth("Account file is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		return parseJSONAccounts(trimmed)
	}
	return ParseAccounts(trimmed)
}

type accountFile struct {
	Version  int             `json:"version"`
	Accounts []accountRecord `json:"accounts"`
}

type accountRecord struct {
	Alias     string `json:"alias"`
	Primary   *bool  `json:"primary"`
	Pool      *bool  `json:"pool"`
	AuthToken string `json:"auth_token"`
	CT0       string `json:"ct0"`
}

func parseJSONAccounts(text string) ([]Account, error) {
	var file accountFile
	if err := json.Unmarshal([]byte(text), &file); err != nil {
		return nil, errs.Auth("Invalid account file: " + err.Error())
	}
	if file.Version != 1 {
		return nil, errs.Auth(fmt.Sprintf("Unsupported account file version %d", file.Version))
	}
	if len(file.Accounts) == 0 {
		return nil, errs.Auth("Account file has no accounts")
	}
	out := make([]Account, 0, len(file.Accounts))
	seenToken := map[string]struct{}{}
	seenAlias := map[string]struct{}{}
	primaries := 0
	for i, item := range file.Accounts {
		token := strings.TrimSpace(item.AuthToken)
		ct0 := strings.TrimSpace(item.CT0)
		if token == "" || ct0 == "" {
			return nil, errs.Auth(fmt.Sprintf("Account %d is missing auth_token or ct0", i+1))
		}
		if _, ok := seenToken[token]; ok {
			return nil, errs.Auth(fmt.Sprintf("Duplicate auth token on account %d", i+1))
		}
		seenToken[token] = struct{}{}
		alias := strings.TrimSpace(item.Alias)
		if alias != "" {
			if _, ok := seenAlias[alias]; ok {
				return nil, errs.Auth(fmt.Sprintf("Duplicate alias %q", alias))
			}
			seenAlias[alias] = struct{}{}
		}
		primary := item.Primary != nil && *item.Primary
		if primary {
			primaries++
		}
		pool := !primary
		if item.Pool != nil {
			pool = *item.Pool
		}
		out = append(out, Account{
			AuthToken: token,
			CT0:       ct0,
			Label:     Label(token),
			Alias:     alias,
			Primary:   primary,
			Pool:      pool,
		})
	}
	if primaries > 1 {
		return nil, errs.Auth("Account file has more than one primary")
	}
	return out, nil
}

// Roster is the account file. Pool membership is separate from the primary identity.
type Roster struct {
	Accounts []Account
	State    string
}

// Primary returns the marked identity, or nil.
func (r *Roster) Primary() *Account {
	if r == nil {
		return nil
	}
	for i := range r.Accounts {
		if r.Accounts[i].Primary {
			account := r.Accounts[i]
			return &account
		}
	}
	return nil
}

// Pool returns sticky selection among accounts marked for public reads.
func (r *Roster) Pool() (*Pool, error) {
	var pooled []Account
	for _, account := range r.Accounts {
		if account.Pool {
			pooled = append(pooled, account)
		}
	}
	if len(pooled) == 0 {
		return nil, errs.Auth("Account file has no pooled accounts")
	}
	return New(pooled, r.State)
}

// Resolve finds an alias or a label. An alias wins when both names exist on different accounts.
func (r *Roster) Resolve(ref string) (Account, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Account{}, errs.Auth("Account selector is empty")
	}
	var aliasHit, labelHit *Account
	for i := range r.Accounts {
		account := &r.Accounts[i]
		if account.Alias != "" && account.Alias == ref {
			aliasHit = account
		}
		if account.Label == ref {
			labelHit = account
		}
	}
	if aliasHit != nil && labelHit != nil && aliasHit.Label != labelHit.Label {
		return Account{}, errs.Auth(fmt.Sprintf("Account %q is ambiguous", ref))
	}
	if aliasHit != nil {
		return *aliasHit, nil
	}
	if labelHit != nil {
		return *labelHit, nil
	}
	return Account{}, errs.Auth(fmt.Sprintf("No account %q", ref))
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
