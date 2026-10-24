// Package cli is the twitter command.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yoyooyooo/twitter-use/internal/accounts"
	"github.com/yoyooyooo/twitter-use/internal/auth"
	"github.com/yoyooyooo/twitter-use/internal/client"
	"github.com/yoyooyooo/twitter-use/internal/config"
	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/output"
	"github.com/yoyooyooo/twitter-use/internal/timeutil"
)

// Execute runs the CLI.
func Execute() int {
	root := newRoot()
	if err := root.Execute(); err != nil {
		return 1
	}
	return 0
}

type app struct {
	json       bool
	yaml       bool
	compact    bool
	verbose    bool
	configPath string
	out        io.Writer
	err        io.Writer
}

func newRoot() *cobra.Command {
	a := &app{out: os.Stdout, err: os.Stderr}
	root := &cobra.Command{
		Use:           "twitter",
		Short:         "Read and write X from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := output.Mode(a.json, a.yaml); err != nil {
				return err
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&a.json, "json", false, "Output JSON")
	root.PersistentFlags().BoolVar(&a.yaml, "yaml", false, "Output YAML")
	root.PersistentFlags().BoolVarP(&a.compact, "compact", "c", false, "Compact tweet JSON")
	root.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false, "Log request decisions to stderr")
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "Config YAML path")
	addRead(root, a)
	addWrite(root, a)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root
}

func (a *app) open(cmd *cobra.Command, usePool bool) (*client.Client, config.Config, error) {
	cfg := config.Load(a.configPath)
	rate := client.Rate{
		RequestDelay:   time.Duration(cfg.RateLimit.RequestDelay * float64(time.Second)),
		MaxRetries:     cfg.RateLimit.MaxRetries,
		RetryBaseDelay: time.Duration(cfg.RateLimit.RetryBaseDelay * float64(time.Second)),
	}
	if usePool && accounts.Configured() {
		pool, err := accounts.LoadFromEnv()
		if err != nil {
			return nil, cfg, err
		}
		first := pool.Accounts[0]
		api, err := client.New(client.Credentials{AuthToken: first.AuthToken, CT0: first.CT0}, pool, rate)
		return api, cfg, err
	}
	_ = cmd
	session, err := a.identity(rate)
	if err != nil {
		return nil, cfg, err
	}
	api, err := client.New(session.Credentials(), nil, rate)
	return api, cfg, err
}

func (a *app) identity(rate client.Rate) (auth.Session, error) {
	if session, ok := auth.FromEnv(); ok {
		log.Printf("Loaded cookies from environment variables")
		return session, nil
	}
	sessions := auth.BrowserSessions()
	if label := rememberedIdentity(); label != "" {
		for _, session := range sessions {
			if accounts.Label(session.AuthToken) == label {
				log.Printf("Using remembered identity %s", label)
				return session, nil
			}
		}
	}
	var last error
	for _, session := range sessions {
		api, err := client.New(session.Credentials(), nil, rate, client.WithoutTransaction())
		if err != nil {
			last = err
			continue
		}
		profile, err := api.Me()
		if err != nil || profile.ScreenName == "" {
			if err != nil {
				last = err
			}
			continue
		}
		label := accounts.Label(session.AuthToken)
		_ = rememberIdentity(label)
		log.Printf("Selected browser identity %s @%s", label, profile.ScreenName)
		return session, nil
	}
	if last == nil {
		last = errs.Auth("no working browser login; set TWITTER_AUTH_TOKEN and TWITTER_CT0")
	}
	return auth.Session{}, last
}

func identityPath() string {
	return filepath.Join(accounts.ConfigDir(), "identity")
}

func rememberedIdentity() string {
	raw, err := os.ReadFile(identityPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func rememberIdentity(label string) error {
	path := identityPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(label+"\n"), 0o600)
}

func (a *app) fail(err error) error {
	if err == nil {
		return nil
	}
	payload := output.Failure(err, nil)
	wrote, emitErr := output.Emit(a.out, payload, a.json, a.yaml)
	if emitErr != nil {
		fmt.Fprintln(a.err, emitErr)
	}
	if !wrote {
		fmt.Fprintf(a.err, "error: %s\n", err)
	}
	return err
}

func (a *app) emit(payload any) error {
	wrote, err := output.Emit(a.out, payload, a.json, a.yaml)
	if err != nil {
		return err
	}
	if wrote {
		return nil
	}
	return errHuman
}

var errHuman = fmt.Errorf("human")

func (a *app) tweets(cmd *cobra.Command, usePool bool, label string, max int, full bool, outputFile string, fetch func(*client.Client, int) ([]model.Tweet, error)) error {
	api, cfg, err := a.open(cmd, usePool)
	if err != nil {
		return a.fail(err)
	}
	count, err := fetchCount(max, cfg)
	if err != nil {
		return a.fail(err)
	}
	tweets, err := fetch(api, count)
	if err != nil && len(tweets) == 0 {
		return a.fail(err)
	}
	if outputFile != "" {
		if err := writeJSON(outputFile, output.Tweets(tweets)); err != nil {
			return a.fail(err)
		}
	}
	if a.compact {
		raw, _ := json.MarshalIndent(output.Compact(tweets), "", "  ")
		fmt.Fprintln(a.out, string(raw))
		return nil
	}
	payload := output.Success(output.Tweets(tweets))
	payload["pagination"] = pagination(api, false)
	if err := a.emit(payload); err == nil {
		return nil
	}
	printTweets(a.out, label, tweets, full)
	if notice := pagination(api, true); notice != nil {
		if msg, ok := notice["message"].(string); ok && msg != "" {
			fmt.Fprintln(a.err, msg)
		}
	}
	if err != nil && !isHuman(err) {
		return a.fail(err)
	}
	return nil
}

func isHuman(err error) bool { return err == errHuman }

func fetchCount(max int, cfg config.Config) (int, error) {
	count := cfg.Fetch.Count
	if max != 0 {
		if max < 0 {
			return 0, errs.Input("--max must be greater than 0")
		}
		count = max
	}
	if count < 1 {
		count = 1
	}
	return client.EnforceCount(count)
}

func pagination(api *client.Client, withMessage bool) map[string]any {
	status := api.FetchStatus()
	if status.Stop == "" {
		status.Stop = "complete"
	}
	notice := map[string]any{
		"requested": status.Requested, "returned": status.Returned,
		"stop": status.Stop, "nextCursor": status.NextCursor,
	}
	if status.Stop != "complete" && status.Stop != "max_count" && status.Stop != "since_reached" {
		notice["truncated"] = true
		notice["reason"] = status.Stop
	}
	if withMessage {
		if msg := stopMessage(status.Stop); msg != "" && status.Stop != "max_count" && status.Stop != "complete" {
			notice["message"] = msg
		}
	}
	return notice
}

func stopMessage(stop string) string {
	switch stop {
	case "rate_limited":
		return "Partial result: pooled accounts are rate-limited. The primary login was not used."
	case "empty_page":
		return "Stopped: later pages kept a cursor but contained no new items."
	case "cursor_stalled":
		return "Stopped: the pagination cursor did not advance."
	case "source_exhausted":
		return "Stopped: the endpoint returned no further page."
	case "attempt_limit":
		return "Stopped: reached the pagination attempt limit before the requested count."
	case "max_count":
		return "Stopped at the requested count. Pass --cursor to continue."
	case "since_reached":
		return "Stopped: reached --since."
	default:
		return ""
	}
}

func printTweets(w io.Writer, label string, tweets []model.Tweet, full bool) {
	fmt.Fprintf(w, "%s — %d\n", label, len(tweets))
	for i, tweet := range tweets {
		text := tweet.Text
		if !full {
			text = strings.ReplaceAll(text, "\n", " ")
			if len([]rune(text)) > 100 {
				text = string([]rune(text)[:97]) + "..."
			}
		}
		fmt.Fprintf(w, "%d\t@%s\t%s\t❤%d\t%s\n", i+1, tweet.Author.ScreenName, timeutil.Relative(tweet.CreatedAt), tweet.Metrics.Likes, text)
	}
}

func writeJSON(path string, data any) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func tweetID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "/status/") {
		parts := strings.Split(value, "/status/")
		value = parts[len(parts)-1]
	}
	value = strings.SplitN(value, "?", 2)[0]
	value = strings.Trim(value, "/")
	if value == "" || !allDigits(value) {
		return "", errs.Input("tweet id must be numeric or a status URL")
	}
	return value, nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
