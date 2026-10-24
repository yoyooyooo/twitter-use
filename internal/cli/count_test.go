package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yoyooyooo/twitter-use/internal/accounts"
)

func TestInvalidCountBeforeAccountLoading(t *testing.T) {
	// A missing explicit pool fails locally if account loading runs first.
	// No test may discover a real browser session or contact X.
	t.Setenv(accounts.FileEnv, filepath.Join(t.TempDir(), "missing-accounts.txt"))
	commands := []struct {
		name string
		make func(*app) *cobra.Command
		args []string
	}{
		{"search", searchCmd, []string{"golang"}},
		{"posts", postsCmd, []string{"example"}},
		{"likes", likesCmd, []string{"example"}},
		{"tweet", tweetCmd, []string{"123"}},
		{"followers", followersCmd, []string{"example"}},
		{"following", followingCmd, []string{"example"}},
	}
	for _, command := range commands {
		for _, count := range []int{-1, 501} {
			t.Run(fmt.Sprintf("%s/%d", command.name, count), func(t *testing.T) {
				var out bytes.Buffer
				a := &app{json: true, out: &out, err: &out}
				cmd := command.make(a)
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				cmd.SetArgs(append(append([]string{}, command.args...), "-n", fmt.Sprint(count)))
				err := cmd.Execute()
				want := "exceeds the fetch ceiling"
				if count < 0 {
					want = "--max must be greater than 0"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want %q", err, want)
				}
				if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), `"ok": false`) {
					t.Fatalf("missing structured input error: %s", out.String())
				}
			})
		}
	}
}

func TestConfiguredCountBeforeAccountLoading(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(accounts.FileEnv, filepath.Join(dir, "missing-accounts.txt"))
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("fetch:\n  count: 501\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := &app{json: true, out: &out, err: &out, configPath: configPath}
	cmd := searchCmd(a)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"golang"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "exceeds the fetch ceiling") {
		t.Fatalf("error = %v, want configured count validation", err)
	}
}
