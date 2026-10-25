// Package graphql resolves Twitter query IDs and builds request URLs.
package graphql

import (
	"encoding/json"
	"log"
	"net/url"
	"regexp"
	"sync"

	"github.com/yoyooyooo/twitter-use/internal/errs"
)

const openAPIURL = "https://raw.githubusercontent.com/fa0311/twitter-openapi/refs/heads/main/src/config/placeholder.json"

// FallbackQueryIDs are the last known query IDs. A 404 retries a live lookup.
var FallbackQueryIDs = map[string]string{
	"HomeTimeline": "0toR0MxC06yY3Fc57GXBpA", "HomeLatestTimeline": "KQPfcqQJJqF6M0D77p3JKw",
	"UserByScreenName": "1VOOyvKkiI3FMmkeDNxM9A", "UserTweets": "q6xj5bs0hapm9309hexA_g",
	"UserTweetsAndReplies": "bnsYxFljT6y98rFwiGr3fw",
	"TweetDetail":          "xd_EMdYvB9hfZsZ6Idri0w", "Likes": "mzOgReEsktOIIXzWkqf0ig",
	"SearchTimeline": "VhUd6vHVmLBcw0uX-6jMLA", "Bookmarks": "2neUNDqrrFzbLui8yallcQ",
	"ListLatestTweetsTimeline": "RlZzktZY_9wJynoepm8ZsA", "Followers": "IOh4aS6UdGWGJUYTqliQ7Q",
	"Following": "zx6e-TLzRkeDO_a7p4b3JQ", "CreateTweet": "IID9x6WsdMnTlXnzXGq8ng",
	"DeleteTweet": "VaenaVgh5q5ih7kvyVjgtg", "FavoriteTweet": "lI07N6Otwv1PhnEgXILM7A",
	"UnfavoriteTweet": "ZYKSe-w7KEslx3JhSIk5LA", "CreateRetweet": "ojPdsZsimiJrUGLR1sjUtA",
	"DeleteRetweet": "iQtK4dl5hBmXewYZuEOKVw", "Viewer": "TpQxQtWMTMnIAMPlMGMqNw", "CreateBookmark": "aoDbu3RHznuiSkQ9aNM67Q",
	"DeleteBookmark": "Wlmlj2-xzyS1GN3a6cj-mQ", "TweetResultByRestId": "a-Xpq8PjX9K6U5cmc1f05A",
	"BookmarkFoldersSlice": "i78YDd0Tza-dV4SYs58kRg", "BookmarkFolderTimeline": "hNY7X2xE2N7HVF6Qb_mu6w",
}

// Features is the flag set sent with timeline requests. Live HTML may update existing keys.
var Features = map[string]bool{
	"responsive_web_graphql_exclude_directive_enabled": true, "verified_phone_label_enabled": false,
	"creator_subscriptions_tweet_preview_api_enabled": true, "responsive_web_graphql_timeline_navigation_enabled": true,
	"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false, "c9s_tweet_anatomy_moderator_badge_enabled": true,
	"tweetypie_unmention_optimization_enabled": true, "responsive_web_edit_tweet_api_enabled": true,
	"graphql_is_translatable_rweb_tweet_is_translatable_enabled": true, "view_counts_everywhere_api_enabled": true,
	"longform_notetweets_consumption_enabled": true, "responsive_web_twitter_article_tweet_consumption_enabled": true,
	"tweet_awards_web_tipping_enabled": false, "longform_notetweets_rich_text_read_enabled": true,
	"longform_notetweets_inline_media_enabled": true, "rweb_video_timestamps_enabled": true,
	"responsive_web_media_download_video_enabled": true, "freedom_of_speech_not_reach_fetch_enabled": true,
	"standardized_nudges_misinfo": true, "responsive_web_enhance_cards_enabled": false,
}

var (
	mu       sync.Mutex
	cached   = map[string]string{}
	scanned  bool
	scriptRE = regexp.MustCompile(`(?:src|href)=["'](https://abs\.twimg\.com/responsive-web/client-web[^"']+\.js)["']`)
	opRE     = regexp.MustCompile(`queryId:\s*"([A-Za-z0-9_-]+)"[^}]{0,200}operationName:\s*"([^"]+)"`)
	flagRE   = regexp.MustCompile(`"([a-z][a-z0-9_]+)":\s*\{\s*"value"\s*:\s*(true|false)`)
)

// FetchFunc downloads a URL body.
type FetchFunc func(rawURL string) (string, error)

// BuildURL builds a GraphQL GET URL. False feature flags are omitted.
func BuildURL(queryID, operation string, variables, features, toggles map[string]any) string {
	compact := map[string]any{}
	for key, value := range features {
		if value != false {
			compact[key] = value
		}
	}
	u := "https://x.com/i/api/graphql/" + queryID + "/" + operation +
		"?variables=" + quote(variables) + "&features=" + quote(compact)
	if len(toggles) > 0 {
		u += "&fieldToggles=" + quote(toggles)
	}
	return u
}

func quote(v any) string {
	raw, _ := json.Marshal(v)
	return url.QueryEscape(string(raw))
}

// Resolve returns a query ID, preferring the fallback until it 404s.
func Resolve(operation string, preferFallback bool, fetch FetchFunc) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if id := cached[operation]; id != "" {
		return id, nil
	}
	if preferFallback {
		if id := FallbackQueryIDs[operation]; id != "" {
			cached[operation] = id
			return id, nil
		}
	}
	if fetch != nil {
		if id := fromGitHub(fetch, operation); id != "" {
			cached[operation] = id
			return id, nil
		}
		scan(fetch)
		if id := cached[operation]; id != "" {
			return id, nil
		}
	}
	if id := FallbackQueryIDs[operation]; id != "" {
		cached[operation] = id
		return id, nil
	}
	return "", errs.QueryID(`Cannot resolve queryId for "` + operation + `"`)
}

// Invalidate drops one cached query ID so the next call does a live lookup.
func Invalidate(operation string) {
	mu.Lock()
	delete(cached, operation)
	mu.Unlock()
}

// UpdateFeatures copies changed flags that already exist. It never adds keys.
func UpdateFeatures(page string) {
	mu.Lock()
	defer mu.Unlock()
	found := 0
	for _, match := range flagRE.FindAllStringSubmatch(page, -1) {
		current, ok := Features[match[1]]
		value := match[2] == "true"
		if ok && current != value {
			Features[match[1]] = value
			found++
		}
	}
	if found > 0 {
		log.Printf("Updated %d feature flags from x.com", found)
	}
}

// FeatureCopy returns a copy safe to mutate per request.
func FeatureCopy() map[string]any {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]any, len(Features))
	for key, value := range Features {
		out[key] = value
	}
	return out
}

func fromGitHub(fetch FetchFunc, operation string) string {
	body, err := fetch(openAPIURL)
	if err != nil {
		return ""
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return ""
	}
	item, _ := parsed[operation].(map[string]any)
	id, _ := item["queryId"].(string)
	return id
}

func scan(fetch FetchFunc) {
	if scanned {
		return
	}
	scanned = true
	page, err := fetch("https://x.com")
	if err != nil {
		log.Printf("Failed to scan JS bundles: %s", err)
		return
	}
	scripts := scriptRE.FindAllStringSubmatch(page, -1)
	for _, script := range scripts {
		bundle, err := fetch(script[1])
		if err != nil {
			continue
		}
		for _, match := range opRE.FindAllStringSubmatch(bundle, -1) {
			if _, ok := cached[match[2]]; !ok {
				cached[match[2]] = match[1]
			}
		}
	}
	log.Printf("Scanned %d JS bundles, cached %d query IDs", len(scripts), len(cached))
}
