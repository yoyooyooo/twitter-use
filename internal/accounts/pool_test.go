package accounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirIsOurs(t *testing.T) {
	if filepath.Base(ConfigDir()) != ".twitter-use" {
		t.Fatal(ConfigDir())
	}
	if filepath.Base(DefaultAccountsFile()) != "accounts.txt" || filepath.Base(DefaultStateFile()) != "account_pool.json" {
		t.Fatal(DefaultAccountsFile(), DefaultStateFile())
	}
	if strings.Contains(ConfigDir(), "twitter-cli") {
		t.Fatal(ConfigDir())
	}
}

func TestParseAndStickyPool(t *testing.T) {
	list, err := ParseAccounts("# comment\n\nalpha ct0a\nbeta ct0b\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Label == list[1].Label || len(list[0].Label) != 12 {
		t.Fatalf("labels = %#v", list)
	}
	if _, err := ParseAccounts("alpha ct0a\nalpha ct0b\n"); err == nil {
		t.Fatal("duplicate token was accepted")
	}
	dir := t.TempDir()
	pool, err := New(list, filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := pool.Acquire("TweetDetail")
	if err != nil || first == nil || first.Label != list[0].Label {
		t.Fatalf("first = %#v %v", first, err)
	}
	again, err := pool.Acquire("TweetDetail")
	if err != nil || again.Label != first.Label {
		t.Fatalf("sticky = %#v", again)
	}
	if err := pool.Cool(*first, "TweetDetail", 1e12); err != nil {
		t.Fatal(err)
	}
	next, err := pool.Acquire("TweetDetail")
	if err != nil || next.Label != list[1].Label {
		t.Fatalf("rotated = %#v", next)
	}
	raw, err := os.ReadFile(pool.State)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "alpha") || strings.Contains(string(raw), "ct0") {
		t.Fatalf("state stored a credential: %s", raw)
	}
}

func TestExplicitMissingFileDoesNotFallBack(t *testing.T) {
	t.Setenv(FileEnv, filepath.Join(t.TempDir(), "missing.txt"))
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("missing explicit pool file fell through")
	}
}

func TestPoolPathOverridesDoNotNeedHomeDirectory(t *testing.T) {
	dir := t.TempDir()
	// A file in place of HOME makes accidental default-path access fail.
	home := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(home, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	accountPath := filepath.Join(dir, "private", "accounts.txt")
	statePath := filepath.Join(dir, "state", "account_pool.json")
	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accountPath, []byte("alpha ct0a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(FileEnv, accountPath)
	t.Setenv(StateEnv, statePath)
	pool, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	account, err := pool.Acquire("TweetDetail")
	if err != nil || account == nil || account.Label != Label("alpha") {
		t.Fatalf("acquire = %v, %v", account, err)
	}
	if pool.State != statePath {
		t.Fatalf("state path = %q, want %q", pool.State, statePath)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatal(err)
	}
}

func TestOperationKey(t *testing.T) {
	got := OperationKey("https://x.com/i/api/graphql/abc/TweetDetail?variables=1")
	if got != "TweetDetail" {
		t.Fatal(got)
	}
}
