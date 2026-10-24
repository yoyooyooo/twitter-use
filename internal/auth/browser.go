package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

type chromiumBrowser struct {
	name    string
	roots   []string
	service string
	account string
}

func fromBrowser() (Session, []string, error) {
	sessions, diagnostics, err := browserSessions()
	if err != nil || len(sessions) == 0 {
		return Session{}, diagnostics, err
	}
	return sessions[0], diagnostics, nil
}

func browserSessions() ([]Session, []string, error) {
	var diagnostics []string
	var sessions []Session
	seen := map[string]struct{}{}
	for _, name := range browserOrder() {
		var found []Session
		var notes []string
		var err error
		if name == "firefox" {
			found, notes, err = firefoxSessions()
		} else {
			found, notes, err = chromiumSessions(browserSpec(name))
		}
		diagnostics = append(diagnostics, notes...)
		if err != nil {
			diagnostics = append(diagnostics, name+": "+err.Error())
			continue
		}
		for _, session := range found {
			if session.AuthToken == "" || session.CT0 == "" {
				continue
			}
			label := session.AuthToken + "\n" + session.CT0
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = struct{}{}
			sessions = append(sessions, session)
		}
	}
	return sessions, diagnostics, nil
}

func browserSpec(name string) chromiumBrowser {
	home, _ := os.UserHomeDir()
	switch name {
	case "arc":
		return chromiumBrowser{name, []string{filepath.Join(home, "Library/Application Support/Arc/User Data")}, "Arc Safe Storage", "Arc"}
	case "edge":
		return chromiumBrowser{name, []string{filepath.Join(home, "Library/Application Support/Microsoft Edge")}, "Microsoft Edge Safe Storage", "Microsoft Edge"}
	case "brave":
		return chromiumBrowser{name, []string{filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser")}, "Brave Safe Storage", "Brave"}
	default:
		roots := []string{filepath.Join(home, "Library/Application Support/Google/Chrome")}
		if runtime.GOOS == "linux" {
			roots = []string{
				filepath.Join(home, ".config/google-chrome"),
				filepath.Join(home, ".config/chromium"),
			}
		}
		return chromiumBrowser{"chrome", roots, "Chrome Safe Storage", "Chrome"}
	}
}

func chromiumSessions(spec chromiumBrowser) ([]Session, []string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, nil, fmt.Errorf("browser cookie decryption is implemented for macOS and Linux; set TWITTER_AUTH_TOKEN and TWITTER_CT0")
	}
	var notes []string
	var sessions []Session
	key, err := cookieKey(spec)
	if err != nil {
		return nil, []string{spec.name + " keychain: " + err.Error()}, err
	}
	for _, root := range spec.roots {
		for _, db := range cookieDBs(root) {
			groups, err := readChromiumDB(db, key)
			notes = append(notes, spec.name+" "+db)
			if err != nil {
				notes = append(notes, err.Error())
				continue
			}
			for _, values := range groups {
				if values["auth_token"] != "" && values["ct0"] != "" {
					sessions = append(sessions, sessionFrom(values))
				}
			}
		}
	}
	return sessions, notes, nil
}

func cookieDBs(root string) []string {
	var out []string
	for _, profile := range []string{"Default"} {
		path := filepath.Join(root, profile, "Cookies")
		if _, err := os.Stat(path); err == nil {
			out = append(out, path)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, "Profile *", "Cookies"))
	out = append(out, matches...)
	if env := strings.TrimSpace(os.Getenv("TWITTER_CHROME_PROFILE")); env != "" {
		path := filepath.Join(root, env, "Cookies")
		if _, err := os.Stat(path); err == nil {
			return []string{path}
		}
	}
	return out
}

func cookieKey(spec chromiumBrowser) ([]byte, error) {
	var raw []byte
	var err error
	switch runtime.GOOS {
	case "darwin":
		raw, err = exec.Command("security", "find-generic-password", "-w", "-s", spec.service, "-a", spec.account).Output()
		if err != nil {
			return nil, fmt.Errorf("keychain %s: %w", spec.service, err)
		}
		return pbkdf2.Key([]byte(strings.TrimSpace(string(raw))), []byte("saltysalt"), 1003, 16, sha1.New), nil
	default:
		raw, err = exec.Command("secret-tool", "lookup", "application", strings.ToLower(spec.account)).Output()
		if err != nil {
			return nil, fmt.Errorf("secret-tool %s: %w", spec.account, err)
		}
		return pbkdf2.Key([]byte(strings.TrimRight(string(raw), "\n")), []byte("saltysalt"), 1, 16, sha1.New), nil
	}
}

func readChromiumDB(path string, key []byte) ([]map[string]string, error) {
	copied, err := copyDB(path)
	if err != nil {
		return nil, err
	}
	defer os.Remove(copied)
	db, err := sql.Open("sqlite", "file:"+copied+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT host_key, name, value, encrypted_value FROM cookies WHERE host_key LIKE '%x.com' OR host_key LIKE '%twitter.com'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byHost := map[string]map[string]string{}
	for rows.Next() {
		var host, name, plain string
		var encrypted []byte
		if err := rows.Scan(&host, &name, &plain, &encrypted); err != nil {
			return nil, err
		}
		if !twitterHost(host) || name == "" {
			continue
		}
		if plain == "" && len(encrypted) > 0 {
			plain, err = decryptCookie(encrypted, key)
			if err != nil {
				continue
			}
		}
		values := byHost[host]
		if values == nil {
			values = map[string]string{}
			byHost[host] = values
		}
		values[name] = plain
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var groups []map[string]string
	for _, values := range byHost {
		groups = append(groups, values)
	}
	return groups, nil
}

func firefoxSessions() ([]Session, []string, error) {
	home, _ := os.UserHomeDir()
	roots := []string{filepath.Join(home, "Library/Application Support/Firefox/Profiles")}
	if runtime.GOOS == "linux" {
		roots = []string{filepath.Join(home, ".mozilla/firefox")}
	}
	var notes []string
	var sessions []Session
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "*", "cookies.sqlite"))
		for _, path := range matches {
			notes = append(notes, "firefox "+path)
			groups, err := readFirefox(path)
			if err != nil {
				notes = append(notes, err.Error())
				continue
			}
			for _, values := range groups {
				if values["auth_token"] != "" && values["ct0"] != "" {
					sessions = append(sessions, sessionFrom(values))
				}
			}
		}
	}
	return sessions, notes, nil
}

func readFirefox(path string) ([]map[string]string, error) {
	copied, err := copyDB(path)
	if err != nil {
		return nil, err
	}
	defer os.Remove(copied)
	db, err := sql.Open("sqlite", "file:"+copied+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT host, name, value FROM moz_cookies WHERE host LIKE '%x.com' OR host LIKE '%twitter.com'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byHost := map[string]map[string]string{}
	for rows.Next() {
		var host, name, value string
		if err := rows.Scan(&host, &name, &value); err != nil {
			return nil, err
		}
		if !twitterHost(host) || name == "" {
			continue
		}
		values := byHost[host]
		if values == nil {
			values = map[string]string{}
			byHost[host] = values
		}
		values[name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var groups []map[string]string
	for _, values := range byHost {
		groups = append(groups, values)
	}
	return groups, nil
}

func copyDB(path string) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dst, err := os.CreateTemp("", "twitter-cookies-*.sqlite")
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(dst.Name())
		return "", err
	}
	return dst.Name(), nil
}

func decryptCookie(encrypted, key []byte) (string, error) {
	if len(encrypted) < 3 || (string(encrypted[:3]) != "v10" && string(encrypted[:3]) != "v11") {
		return string(encrypted), nil
	}
	data := encrypted[3:]
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("bad cookie ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := bytesRepeat(' ', aes.BlockSize)
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)
	if len(plain) == 0 {
		return "", fmt.Errorf("empty cookie")
	}
	pad := int(plain[len(plain)-1])
	if pad <= 0 || pad > len(plain) {
		return "", fmt.Errorf("bad cookie padding")
	}
	for _, b := range plain[len(plain)-pad:] {
		if int(b) != pad {
			return "", fmt.Errorf("bad cookie padding")
		}
	}
	plain = plain[:len(plain)-pad]
	if len(plain) > 32 && !printable(plain[:32]) {
		plain = plain[32:]
	}
	return string(plain), nil
}

func printable(b []byte) bool {
	for _, c := range b {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func twitterHost(host string) bool {
	host = strings.TrimPrefix(host, ".")
	return host == "x.com" || host == "twitter.com" || strings.HasSuffix(host, ".x.com") || strings.HasSuffix(host, ".twitter.com")
}
