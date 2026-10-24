package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yoyooyooo/twitter-use/internal/model"
)

const ttl = time.Hour

type file struct {
	CreatedAt float64 `json:"created_at"`
	Tweets    []entry `json:"tweets"`
}

type entry struct {
	Index  int    `json:"index"`
	ID     string `json:"id"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

func path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".twitter-use", "last_results.json")
}

// Save persists the last displayed list.
func Save(tweets []model.Tweet) {
	dir := filepath.Dir(path())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	payload := file{CreatedAt: float64(time.Now().UnixNano()) / 1e9}
	for i, tweet := range tweets {
		if tweet.ID == "" {
			continue
		}
		text := tweet.Text
		if len(text) > 80 {
			text = text[:80]
		}
		payload.Tweets = append(payload.Tweets, entry{Index: i + 1, ID: tweet.ID, Author: tweet.Author.ScreenName, Text: text})
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path(), raw, 0o600)
}

// Resolve maps a 1-based index to a tweet id. The second result is the cache size.
func Resolve(index int) (string, int) {
	raw, err := os.ReadFile(path())
	if err != nil {
		return "", 0
	}
	var payload file
	if json.Unmarshal(raw, &payload) != nil {
		return "", 0
	}
	if time.Since(time.Unix(0, int64(payload.CreatedAt*1e9))) > ttl {
		return "", 0
	}
	for _, item := range payload.Tweets {
		if item.Index == index {
			return item.ID, len(payload.Tweets)
		}
	}
	return "", len(payload.Tweets)
}
