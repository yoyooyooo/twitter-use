package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAsRejectsUnknownAccountBeforeRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("TWITTER_AUTH_TOKEN", "")
	t.Setenv("TWITTER_CT0", "")
	t.Setenv("TWITTER_ACCOUNTS_FILE", "")
	if err := os.MkdirAll(filepath.Join(home, ".twitter-use"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version":1,"accounts":[{"alias":"main","primary":true,"auth_token":"alpha","ct0":"ct0a"},{"alias":"spare","auth_token":"beta","ct0":"ct0b"}]}`)
	if err := os.WriteFile(filepath.Join(home, ".twitter-use", "accounts.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root := newRoot()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--as", "missing", "whoami"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), `No account "missing"`) {
		t.Fatalf("error = %v", err)
	}
}
