package filter

import (
	"math"
	"sort"

	"github.com/yoyooyooo/twitter-use/internal/config"
	"github.com/yoyooyooo/twitter-use/internal/model"
)

// Apply scores tweets and applies the configured mode.
func Apply(tweets []model.Tweet, cfg config.Config) []model.Tweet {
	out := append([]model.Tweet(nil), tweets...)
	if len(cfg.Filter.Lang) > 0 {
		allow := map[string]struct{}{}
		for _, lang := range cfg.Filter.Lang {
			if lang != "" {
				allow[lang] = struct{}{}
			}
		}
		kept := out[:0]
		for _, tweet := range out {
			if _, ok := allow[tweet.Lang]; ok {
				kept = append(kept, tweet)
			}
		}
		out = kept
	}
	if cfg.Filter.ExcludeRetweets {
		kept := out[:0]
		for _, tweet := range out {
			if !tweet.IsRetweet {
				kept = append(kept, tweet)
			}
		}
		out = kept
	}
	for i := range out {
		score := score(out[i], cfg.Filter.Weights)
		rounded := math.Round(score*10) / 10
		out[i].Score = &rounded
	}
	sort.SliceStable(out, func(i, j int) bool {
		return *out[i].Score > *out[j].Score
	})
	switch cfg.Filter.Mode {
	case "score":
		kept := out[:0]
		for _, tweet := range out {
			if tweet.Score != nil && *tweet.Score >= cfg.Filter.MinScore {
				kept = append(kept, tweet)
			}
		}
		return kept
	case "all":
		return out
	default:
		n := cfg.Filter.TopN
		if n < 1 {
			n = 20
		}
		if n > len(out) {
			n = len(out)
		}
		return out[:n]
	}
}

func score(tweet model.Tweet, weights map[string]float64) float64 {
	w := func(key string, fallback float64) float64 {
		if v, ok := weights[key]; ok {
			return v
		}
		return fallback
	}
	views := tweet.Metrics.Views
	if views < 1 {
		views = 1
	}
	return w("likes", 1)*float64(tweet.Metrics.Likes) +
		w("retweets", 3)*float64(tweet.Metrics.Retweets) +
		w("replies", 2)*float64(tweet.Metrics.Replies) +
		w("bookmarks", 5)*float64(tweet.Metrics.Bookmarks) +
		w("views_log", 0.5)*math.Log10(float64(views))
}
