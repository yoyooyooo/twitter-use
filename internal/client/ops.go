package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/graphql"
	"github.com/yoyooyooo/twitter-use/internal/jsonx"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/parser"
)

var imageTypes = map[string]struct{}{"image/jpeg": {}, "image/png": {}, "image/gif": {}, "image/webp": {}}

const maxImage = 5 << 20

func (c *Client) Home(count int, promoted bool, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "HomeTimeline", count: count, promoted: promoted, cursor: cursor, post: true, bare: true,
		instructions: func(data map[string]any) []any {
			return parser.Instructions(data, "data", "home", "home_timeline_urt", "instructions")
		},
	})
}

func (c *Client) FollowingFeed(count int, promoted bool, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "HomeLatestTimeline", count: count, promoted: promoted, cursor: cursor, post: true, bare: true,
		instructions: func(data map[string]any) []any {
			return parser.Instructions(data, "data", "home", "home_timeline_urt", "instructions")
		},
	})
}

func (c *Client) Bookmarks(count int, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{operation: "Bookmarks", count: count, cursor: cursor, post: true, instructions: bookmarkInstructions})
}

func bookmarkInstructions(data map[string]any) []any {
	items := parser.Instructions(data, "data", "bookmark_timeline", "timeline", "instructions")
	if items == nil {
		items = parser.Instructions(data, "data", "bookmark_timeline_v2", "timeline", "instructions")
	}
	return items
}

func (c *Client) BookmarkFolders() ([]model.BookmarkFolder, error) {
	var folders []model.BookmarkFolder
	cursor := ""
	for page := 0; page < 10; page++ {
		variables := map[string]any{}
		if cursor != "" {
			variables["cursor"] = cursor
		}
		data, err := c.graphqlGet("BookmarkFoldersSlice", variables, graphql.FeatureCopy(), nil)
		if err != nil {
			return folders, err
		}
		slice, _ := jsonx.Get(data, "data", "viewer", "user_results", "result", "bookmark_collections_slice").(map[string]any)
		if slice == nil {
			break
		}
		items, _ := slice["items"].([]any)
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if id := jsonx.String(item["id"]); id != "" {
				folders = append(folders, model.BookmarkFolder{ID: id, Name: jsonx.String(item["name"])})
			}
		}
		next := jsonx.String(jsonx.Get(slice, "slice_info", "next_cursor"))
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	return folders, nil
}

func (c *Client) BookmarkFolder(id string, count int, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "BookmarkFolderTimeline", count: count, cursor: cursor, override: true,
		extra: map[string]any{"bookmark_collection_id": id, "includePromotedContent": false},
		instructions: func(data map[string]any) []any {
			return parser.Instructions(data, "data", "bookmark_collection_timeline", "timeline", "instructions")
		},
	})
}

func (c *Client) User(screenName string) (model.UserProfile, error) {
	variables := map[string]any{"screen_name": screenName, "withSafetyModeUserFields": true}
	features := map[string]any{
		"hidden_profile_subscriptions_enabled": true, "rweb_tipjar_consumption_enabled": true,
		"responsive_web_graphql_exclude_directive_enabled": true, "verified_phone_label_enabled": false,
		"subscriptions_verification_info_is_identity_verified_enabled": true, "subscriptions_verification_info_verified_since_enabled": true,
		"highlights_tweets_tab_ui_enabled": true, "responsive_web_twitter_article_notes_tab_enabled": true,
		"subscriptions_feature_can_gift_premium": true, "creator_subscriptions_tweet_preview_api_enabled": true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false, "responsive_web_graphql_timeline_navigation_enabled": true,
	}
	data, err := c.graphqlGet("UserByScreenName", variables, features, nil)
	if err != nil {
		return model.UserProfile{}, err
	}
	result, _ := jsonx.Get(data, "data", "user", "result").(map[string]any)
	if result == nil {
		return model.UserProfile{}, errs.NotFound("User @" + screenName + " not found")
	}
	profile := parser.User(result)
	if profile == nil {
		return model.UserProfile{}, errs.NotFound("User @" + screenName + " not found")
	}
	return *profile, nil
}

func (c *Client) ResolveUserID(identifier string) (string, error) {
	identifier = trimAt(identifier)
	if containsDigitID(identifier) {
		return identifier, nil
	}
	profile, err := c.User(identifier)
	if err != nil {
		return "", err
	}
	return profile.ID, nil
}

func (c *Client) UserTweets(userID string, count int, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "UserTweets", count: count, cursor: cursor,
		extra: map[string]any{"userId": userID, "includePromotedContent": true, "withQuickPromoteEligibilityTweetFields": true, "withVoice": true, "withV2Timeline": true},
		instructions: func(data map[string]any) []any {
			items := parser.Instructions(data, "data", "user", "result", "timeline", "timeline", "instructions")
			if items == nil {
				items = parser.Instructions(data, "data", "user", "result", "timeline_v2", "timeline", "instructions")
			}
			return items
		},
	})
}

func (c *Client) Likes(userID string, count int, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "Likes", count: count, cursor: cursor, override: true,
		extra: map[string]any{"userId": userID, "includePromotedContent": false, "withClientEventToken": false, "withBirdwatchNotes": false, "withVoice": true},
		instructions: func(data map[string]any) []any {
			items := parser.Instructions(data, "data", "user", "result", "timeline", "timeline", "instructions")
			if items == nil {
				items = parser.Instructions(data, "data", "user", "result", "timeline_v2", "timeline", "instructions")
			}
			return items
		},
	})
}

func (c *Client) Search(query string, count int, product, cursor string) ([]model.Tweet, error) {
	if product == "" {
		product = "Top"
	}
	return c.timeline(timelineSpec{
		operation: "SearchTimeline", count: count, override: true, post: true, cursor: cursor,
		extra: map[string]any{"rawQuery": query, "querySource": "typed_query", "product": product},
		instructions: func(data map[string]any) []any {
			return parser.Instructions(data, "data", "search_by_raw_query", "search_timeline", "timeline", "instructions")
		},
	})
}

func (c *Client) TweetDetail(id string, count int, ranking, cursor string) ([]model.Tweet, error) {
	if ranking != "Relevance" && ranking != "Recency" {
		return nil, fmt.Errorf("ranking must be Relevance or Recency, got %s", ranking)
	}
	return c.timeline(timelineSpec{
		operation: "TweetDetail", count: count, override: true, cursor: cursor,
		extra: map[string]any{
			"focalTweetId": id, "referrer": "tweet", "with_rux_injections": false, "includePromotedContent": true,
			"rankingMode": ranking, "withCommunity": true, "withQuickPromoteEligibilityTweetFields": true,
			"withBirdwatchNotes": true, "withVoice": true,
		},
		toggles: map[string]any{"withArticleRichContentState": true, "withArticlePlainText": false, "withGrokAnalyze": false, "withDisallowedReplyControls": false},
		instructions: func(data map[string]any) []any {
			items := parser.Instructions(data, "data", "tweetResult", "result", "timeline", "instructions")
			if items == nil {
				items = parser.Instructions(data, "data", "threaded_conversation_with_injections_v2", "instructions")
			}
			return items
		},
	})
}

func (c *Client) Article(id string) (model.Tweet, error) {
	data, err := c.graphqlGet("TweetResultByRestId", map[string]any{
		"tweetId": id, "withCommunity": false, "includePromotedContent": false, "withVoice": false,
	}, map[string]any{
		"longform_notetweets_consumption_enabled": true, "responsive_web_twitter_article_tweet_consumption_enabled": true,
		"longform_notetweets_rich_text_read_enabled": true, "longform_notetweets_inline_media_enabled": true,
		"articles_preview_enabled": true, "responsive_web_graphql_exclude_directive_enabled": true, "verified_phone_label_enabled": false,
	}, map[string]any{"withArticleRichContentState": true, "withArticlePlainText": true})
	if err != nil {
		return model.Tweet{}, err
	}
	result, _ := jsonx.Get(data, "data", "tweetResult", "result").(map[string]any)
	if result == nil {
		return model.Tweet{}, errs.NotFound("Article not found: tweet_id=" + id)
	}
	tweet := parser.Tweet(result, 0)
	if tweet == nil || (tweet.ArticleTitle == nil && tweet.ArticleText == nil) {
		return model.Tweet{}, errs.NotFound("Tweet " + id + " has no article content")
	}
	return *tweet, nil
}

func (c *Client) List(id string, count int, cursor string) ([]model.Tweet, error) {
	return c.timeline(timelineSpec{
		operation: "ListLatestTweetsTimeline", count: count, override: true, cursor: cursor,
		extra: map[string]any{"listId": id},
		instructions: func(data map[string]any) []any {
			return parser.Instructions(data, "data", "list", "tweets_timeline", "timeline", "instructions")
		},
	})
}

func (c *Client) Followers(userID string, count int, cursor string) ([]model.UserProfile, error) {
	return c.users("Followers", userID, count, cursor, func(data map[string]any) []any {
		return parser.Instructions(data, "data", "user", "result", "timeline", "timeline", "instructions")
	}, true)
}

func (c *Client) Following(userID string, count int, cursor string) ([]model.UserProfile, error) {
	return c.users("Following", userID, count, cursor, func(data map[string]any) []any {
		return parser.Instructions(data, "data", "user", "result", "timeline", "timeline", "instructions")
	}, true)
}

func (c *Client) UploadImage(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", errs.Media("File not found: " + path)
	}
	if info.Size() > maxImage {
		return "", errs.Media(fmt.Sprintf("File too large: %.1f MB (max 5 MB)", float64(info.Size())/1024/1024))
	}
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if i := strings.Index(mediaType, ";"); i >= 0 {
		mediaType = mediaType[:i]
	}
	if _, ok := imageTypes[mediaType]; !ok {
		return "", errs.Media("Unsupported image format: " + mediaType + " (supported: jpeg, png, gif, webp)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", errs.Media(err.Error())
	}
	endpoint := "https://upload.twitter.com/i/media/upload.json"
	initValues := url.Values{"command": {"INIT"}, "total_bytes": {fmt.Sprint(info.Size())}, "media_type": {mediaType}}
	initBody, err := c.form(endpoint, initValues)
	if err != nil {
		return "", errs.Media(err.Error())
	}
	mediaID := jsonx.String(initBody["media_id_string"])
	if mediaID == "" {
		return "", errs.Media("INIT did not return media_id")
	}
	appendValues := url.Values{"command": {"APPEND"}, "media_id": {mediaID}, "segment_index": {"0"}, "media_data": {base64.StdEncoding.EncodeToString(raw)}}
	if _, err := c.form(endpoint, appendValues); err != nil {
		return "", errs.Media(err.Error())
	}
	if _, err := c.form(endpoint, url.Values{"command": {"FINALIZE"}, "media_id": {mediaID}}); err != nil {
		return "", errs.Media(err.Error())
	}
	return mediaID, nil
}

func (c *Client) form(endpoint string, values url.Values) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header = formHeaders(c.headers(endpoint, http.MethodPost))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	payload, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, clip(string(payload), 300))
	}
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON")
	}
	return parsed, nil
}

func (c *Client) CreateTweet(text, replyTo string, mediaIDs []string) (string, error) {
	variables := map[string]any{
		"tweet_text":              text,
		"media":                   map[string]any{"media_entities": mediaEntities(mediaIDs), "possibly_sensitive": false},
		"semantic_annotation_ids": []any{}, "dark_request": false,
	}
	if replyTo != "" {
		variables["reply"] = map[string]any{"in_reply_to_tweet_id": replyTo, "exclude_reply_user_ids": []any{}}
	}
	return c.createdTweet("Failed to create tweet", variables)
}

func (c *Client) Quote(id, text string, mediaIDs []string) (string, error) {
	variables := map[string]any{
		"tweet_text": text, "attachment_url": "https://x.com/i/status/" + id,
		"media":                   map[string]any{"media_entities": mediaEntities(mediaIDs), "possibly_sensitive": false},
		"semantic_annotation_ids": []any{}, "dark_request": false,
	}
	return c.createdTweet("Failed to create quote tweet", variables)
}

func (c *Client) createdTweet(fail string, variables map[string]any) (string, error) {
	data, err := c.graphqlPost("CreateTweet", variables, graphql.FeatureCopy())
	if err != nil {
		return "", err
	}
	c.writeDelay()
	if id := jsonx.String(jsonx.Get(data, "data", "create_tweet", "tweet_results", "result", "rest_id")); id != "" {
		return id, nil
	}
	return "", errs.API(0, fail)
}

func mediaEntities(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"media_id": id, "tagged_users": []any{}})
	}
	return out
}

func (c *Client) DeleteTweet(id string) error {
	_, err := c.graphqlPost("DeleteTweet", map[string]any{"tweet_id": id, "dark_request": false}, nil)
	if err == nil {
		c.writeDelay()
	}
	return err
}

func (c *Client) Like(id string) error {
	return c.simplePost("FavoriteTweet", map[string]any{"tweet_id": id})
}
func (c *Client) Unlike(id string) error {
	return c.simplePost("UnfavoriteTweet", map[string]any{"tweet_id": id, "dark_request": false})
}
func (c *Client) Retweet(id string) error {
	return c.simplePost("CreateRetweet", map[string]any{"tweet_id": id, "dark_request": false})
}
func (c *Client) Unretweet(id string) error {
	return c.simplePost("DeleteRetweet", map[string]any{"source_tweet_id": id, "dark_request": false})
}
func (c *Client) Bookmark(id string) error {
	return c.simplePost("CreateBookmark", map[string]any{"tweet_id": id})
}
func (c *Client) Unbookmark(id string) error {
	return c.simplePost("DeleteBookmark", map[string]any{"tweet_id": id})
}

func (c *Client) simplePost(operation string, variables map[string]any) error {
	_, err := c.graphqlPost(operation, variables, nil)
	if err == nil {
		c.writeDelay()
	}
	return err
}

func (c *Client) viewer() (model.UserProfile, error) {
	data, err := c.graphqlPost("Viewer", map[string]any{"withCommunitiesMemberships": false}, nil)
	if err != nil {
		return model.UserProfile{}, err
	}
	result, _ := jsonx.Get(data, "data", "viewer", "user_results", "result").(map[string]any)
	profile := parser.User(result)
	if profile == nil || profile.ScreenName == "" {
		return model.UserProfile{}, errs.API(0, "Failed to fetch current user info")
	}
	return *profile, nil
}

func (c *Client) Me() (model.UserProfile, error) {
	if profile, err := c.viewer(); err == nil && profile.ScreenName != "" {
		return profile, nil
	}
	data, err := c.api("https://x.com/i/api/1.1/account/multi/list.json", http.MethodGet, nil)
	if err != nil {
		return model.UserProfile{}, err
	}
	screen := ""
	if users, ok := data["users"].([]any); ok && len(users) > 0 {
		first, _ := users[0].(map[string]any)
		screen = jsonx.String(first["screen_name"])
	}
	if list, ok := data["_list"].([]any); ok && len(list) > 0 {
		first, _ := list[0].(map[string]any)
		user, _ := first["user"].(map[string]any)
		if user != nil && user["followers_count"] != nil {
			return model.UserProfile{
				ID: jsonx.String(user["id_str"]), Name: jsonx.String(user["name"]), ScreenName: jsonx.String(user["screen_name"]),
				Bio: jsonx.String(user["description"]), Location: jsonx.String(user["location"]),
				URL:            jsonx.String(jsonx.Get(user, "entities", "url", "urls", 0, "expanded_url")),
				FollowersCount: jsonx.Int(user["followers_count"], 0), FollowingCount: jsonx.Int(user["friends_count"], 0),
				TweetsCount: jsonx.Int(user["statuses_count"], 0), LikesCount: jsonx.Int(user["favourites_count"], 0),
				Verified: jsonx.Bool(user["verified"]), ProfileImageURL: jsonx.String(user["profile_image_url_https"]),
				CreatedAt: jsonx.String(user["created_at"]),
			}, nil
		}
		screen = jsonx.String(user["screen_name"])
	}
	if screen != "" {
		return c.User(screen)
	}
	return model.UserProfile{}, errs.API(0, "Failed to fetch current user info")
}

func (c *Client) Follow(userID string) error {
	return c.friendship("https://x.com/i/api/1.1/friendships/create.json", userID, "Failed to follow user")
}

func (c *Client) Unfollow(userID string) error {
	return c.friendship("https://x.com/i/api/1.1/friendships/destroy.json", userID, "Failed to unfollow user")
}

func (c *Client) friendship(endpoint, userID, fail string) error {
	_, err := c.form(endpoint, url.Values{"user_id": {userID}, "include_profile_interstitial_type": {"1"}})
	if err != nil {
		return errs.API(0, fail+": "+err.Error())
	}
	c.writeDelay()
	return nil
}

// Verify does not consult the legacy account endpoints. Both return 401 for a
// session that can still call GraphQL, so the command request is the check.
func (c *Client) Verify() error {
	return nil
}
