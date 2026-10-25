package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/timeutil"
	"gopkg.in/yaml.v3"
)

const schemaVersion = "1"

// Mode is json, yaml, or empty for a human table.
func Mode(asJSON, asYAML bool) (string, error) {
	if asJSON && asYAML {
		return "", fmt.Errorf("use only one of --json or --yaml")
	}
	if asYAML {
		return "yaml", nil
	}
	if asJSON {
		return "json", nil
	}
	switch os.Getenv("OUTPUT") {
	case "yaml":
		return "yaml", nil
	case "json":
		return "json", nil
	case "rich":
		return "", nil
	}
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		return "json", nil
	}
	return "", nil
}

// Success wraps data in the agent schema.
func Success(data any) map[string]any {
	return map[string]any{"ok": true, "schema_version": schemaVersion, "data": data}
}

// Failure wraps an error in the agent schema.
func Failure(err error, details any) map[string]any {
	item := map[string]any{"code": errs.Code(err), "message": err.Error()}
	if details != nil {
		item["details"] = details
	}
	return map[string]any{"ok": false, "schema_version": schemaVersion, "error": item}
}

// Emit writes a payload in the selected mode. It returns false for human mode.
func Emit(w io.Writer, payload any, asJSON, asYAML bool) (bool, error) {
	mode, err := Mode(asJSON, asYAML)
	if err != nil {
		return false, err
	}
	if mode == "" {
		return false, nil
	}
	if mode == "json" {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return true, enc.Encode(payload)
	}
	raw, err := yaml.Marshal(payload)
	if err != nil {
		return true, err
	}
	_, err = w.Write(raw)
	return true, err
}

// Tweets converts tweets to the public JSON shape.
func Tweets(tweets []model.Tweet) []any {
	out := make([]any, 0, len(tweets))
	for _, tweet := range tweets {
		out = append(out, Tweet(tweet))
	}
	return out
}

// Tweet converts one tweet.
func Tweet(tweet model.Tweet) map[string]any {
	data := map[string]any{
		"id":   tweet.ID,
		"text": tweet.Text,
		"author": map[string]any{
			"id": tweet.Author.ID, "name": tweet.Author.Name, "screenName": tweet.Author.ScreenName,
			"profileImageUrl": tweet.Author.ProfileImageURL, "verified": tweet.Author.Verified,
		},
		"metrics": map[string]any{
			"likes": tweet.Metrics.Likes, "retweets": tweet.Metrics.Retweets, "replies": tweet.Metrics.Replies,
			"quotes": tweet.Metrics.Quotes, "views": tweet.Metrics.Views, "bookmarks": tweet.Metrics.Bookmarks,
		},
		"createdAt": tweet.CreatedAt, "createdAtLocal": timeutil.Local(tweet.CreatedAt), "createdAtISO": timeutil.ISO(tweet.CreatedAt),
		"media": media(tweet.Media), "urls": urls(tweet.URLs), "isRetweet": tweet.IsRetweet,
		"retweetedBy": emptyNil(tweet.RetweetedBy), "lang": tweet.Lang, "score": tweet.Score,
		"isSubscriberOnly": tweet.IsSubscriberOnly, "isPromoted": tweet.IsPromoted,
	}
	putText(data, "inReplyToStatusId", tweet.InReplyToStatusID)
	putText(data, "inReplyToUserId", tweet.InReplyToUserID)
	putText(data, "conversationId", tweet.ConversationID)
	putText(data, "quotedStatusId", tweet.QuotedStatusID)
	putText(data, "selfThreadId", tweet.SelfThreadID)
	if tweet.ArticleTitle != nil {
		data["articleTitle"] = *tweet.ArticleTitle
	}
	if tweet.ArticleText != nil {
		data["articleText"] = *tweet.ArticleText
	}
	if tweet.QuotedTweet != nil {
		data["quotedTweet"] = map[string]any{
			"id": tweet.QuotedTweet.ID, "text": tweet.QuotedTweet.Text,
			"author": map[string]any{"screenName": tweet.QuotedTweet.Author.ScreenName, "name": tweet.QuotedTweet.Author.Name},
		}
	}
	return data
}

// Compact is the short tweet shape.
func Compact(tweets []model.Tweet) []any {
	out := make([]any, 0, len(tweets))
	for _, tweet := range tweets {
		text := oneLine(tweet.Text)
		if len(text) > 140 {
			text = text[:137] + "..."
		}
		out = append(out, map[string]any{
			"id": tweet.ID, "author": "@" + tweet.Author.ScreenName, "text": text,
			"likes": tweet.Metrics.Likes, "rts": tweet.Metrics.Retweets, "time": shortTime(tweet.CreatedAt),
		})
	}
	return out
}

// User converts a profile.
func User(user model.UserProfile) map[string]any {
	return map[string]any{
		"id": user.ID, "name": user.Name, "screenName": user.ScreenName, "bio": user.Bio,
		"location": user.Location, "url": user.URL, "followers": user.FollowersCount,
		"following": user.FollowingCount, "tweets": user.TweetsCount, "likes": user.LikesCount,
		"verified": user.Verified, "profileImageUrl": user.ProfileImageURL,
		"createdAt": user.CreatedAt, "createdAtISO": timeutil.ISO(user.CreatedAt),
	}
}

// Users converts profiles.
func Users(users []model.UserProfile) []any {
	out := make([]any, 0, len(users))
	for _, user := range users {
		out = append(out, User(user))
	}
	return out
}

// Folders converts bookmark folders.
func Folders(folders []model.BookmarkFolder) []any {
	out := make([]any, 0, len(folders))
	for _, folder := range folders {
		out = append(out, map[string]any{"id": folder.ID, "name": folder.Name})
	}
	return out
}

func putText(data map[string]any, key, value string) {
	if value != "" {
		data[key] = value
	}
}

func media(items []model.TweetMedia) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"type": item.Type, "url": item.URL, "width": item.Width, "height": item.Height})
	}
	return out
}

func urls(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func emptyNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func oneLine(text string) string {
	out := make([]rune, 0, len(text))
	space := false
	for _, r := range text {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if r == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		out = append(out, r)
	}
	return string(out)
}

func shortTime(createdAt string) string {
	parts := split(createdAt)
	if len(parts) >= 4 && len(parts[3]) >= 5 {
		return parts[1] + " " + parts[2] + " " + parts[3][:5]
	}
	return createdAt
}

func split(text string) []string {
	var parts []string
	start := -1
	for i, r := range text {
		if r == ' ' {
			if start >= 0 {
				parts = append(parts, text[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		parts = append(parts, text[start:])
	}
	return parts
}
