package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yoyooyooo/twitter-use/internal/jsonx"
	"github.com/yoyooyooo/twitter-use/internal/model"
)

// Timeline returns tweets and the bottom cursor from one GraphQL page.
func Timeline(data any, instructions []any) ([]model.Tweet, string) {
	var tweets []model.Tweet
	var cursor string
	for _, raw := range instructions {
		instruction, _ := raw.(map[string]any)
		entries, _ := instruction["entries"].([]any)
		if entries == nil {
			entries, _ = instruction["moduleItems"].([]any)
		}
		for _, rawEntry := range entries {
			entry, _ := rawEntry.(map[string]any)
			content, _ := entry["content"].(map[string]any)
			if next := cursorValue(content); next != "" {
				cursor = next
			}
			item, _ := content["itemContent"].(map[string]any)
			if result, _ := jsonx.Get(item, "tweet_results", "result").(map[string]any); result != nil {
				if tweet := Tweet(result, 0); tweet != nil {
					tweet.IsPromoted = promoted(entry, item)
					tweets = append(tweets, *tweet)
				}
			}
			items, _ := content["items"].([]any)
			for _, rawNested := range items {
				nested, _ := rawNested.(map[string]any)
				nestedItem, _ := jsonx.Get(nested, "item", "itemContent").(map[string]any)
				result, _ := jsonx.Get(nestedItem, "tweet_results", "result").(map[string]any)
				if result == nil {
					continue
				}
				if tweet := Tweet(result, 0); tweet != nil {
					tweet.IsPromoted = promoted(nested, nestedItem)
					tweets = append(tweets, *tweet)
				}
			}
		}
	}
	return tweets, cursor
}

// Instructions reads a nested instruction list.
func Instructions(data any, keys ...any) []any {
	items, _ := jsonx.Get(data, keys...).([]any)
	return items
}

// User parses a user result. UserUnavailable and missing rest_id return nil.
func User(userData map[string]any) *model.UserProfile {
	if jsonx.String(userData["__typename"]) == "UserUnavailable" || jsonx.String(userData["rest_id"]) == "" {
		return nil
	}
	legacy, _ := userData["legacy"].(map[string]any)
	core, _ := userData["core"].(map[string]any)
	avatar, _ := userData["avatar"].(map[string]any)
	location, _ := userData["location"].(map[string]any)
	if legacy == nil {
		legacy = map[string]any{}
	}
	if core == nil {
		core = map[string]any{}
	}
	profile := model.UserProfile{
		ID:              jsonx.String(userData["rest_id"]),
		Name:            firstString(core["name"], legacy["name"]),
		ScreenName:      firstString(core["screen_name"], legacy["screen_name"]),
		Bio:             jsonx.String(legacy["description"]),
		Location:        firstString(location["location"], legacy["location"]),
		URL:             jsonx.String(jsonx.Get(legacy, "entities", "url", "urls", 0, "expanded_url")),
		FollowersCount:  jsonx.Int(legacy["followers_count"], 0),
		FollowingCount:  jsonx.Int(legacy["friends_count"], 0),
		TweetsCount:     jsonx.Int(legacy["statuses_count"], 0),
		LikesCount:      jsonx.Int(legacy["favourites_count"], 0),
		Verified:        jsonx.Bool(userData["is_blue_verified"]) || jsonx.Bool(legacy["verified"]),
		ProfileImageURL: firstString(avatar["image_url"], legacy["profile_image_url_https"]),
		CreatedAt:       firstString(core["created_at"], legacy["created_at"]),
	}
	return &profile
}

// Tweet parses one tweet result, unwrapping visibility wrappers and retweets.
func Tweet(result map[string]any, depth int) *model.Tweet {
	if result == nil || depth > 2 {
		return nil
	}
	data, subscriber := unwrap(result)
	if jsonx.String(data["__typename"]) == "TweetTombstone" {
		return nil
	}
	legacy, ok := data["legacy"].(map[string]any)
	core, okCore := data["core"].(map[string]any)
	if !ok || !okCore {
		return nil
	}
	user, _ := jsonx.Get(core, "user_results", "result").(map[string]any)
	if user == nil {
		user = map[string]any{}
	}
	userLegacy, _ := user["legacy"].(map[string]any)
	userCore, _ := user["core"].(map[string]any)
	isRetweet := jsonx.Get(legacy, "retweeted_status_result", "result") != nil
	actual, actualLegacy, actualUser := data, legacy, user
	actualUserLegacy := userLegacy
	retweetSubscriber := false
	if isRetweet {
		retweet, _ := jsonx.Get(legacy, "retweeted_status_result", "result").(map[string]any)
		retweet, retweetSubscriber = unwrap(retweet)
		rtLegacy, okL := retweet["legacy"].(map[string]any)
		rtCore, okC := retweet["core"].(map[string]any)
		if okL && okC {
			actual = retweet
			actualLegacy = rtLegacy
			actualUser, _ = jsonx.Get(rtCore, "user_results", "result").(map[string]any)
			if actualUser == nil {
				actualUser = map[string]any{}
			}
			actualUserLegacy, _ = actualUser["legacy"].(map[string]any)
		}
	}
	urls := []string{}
	for _, item := range asList(jsonx.Get(actualLegacy, "entities", "urls")) {
		obj, _ := item.(map[string]any)
		urls = append(urls, jsonx.String(obj["expanded_url"]))
	}
	var quoted *model.Tweet
	if quotedRaw, ok := jsonx.Get(actual, "quoted_status_result", "result").(map[string]any); ok {
		quoted = Tweet(quotedRaw, depth+1)
	}
	retweetedBy := ""
	if isRetweet {
		retweetedBy = firstString(userCore["screen_name"], userLegacy["screen_name"])
		if retweetedBy == "" {
			retweetedBy = "unknown"
		}
	}
	note := jsonx.String(jsonx.Get(actual, "note_tweet", "note_tweet_results", "result", "text"))
	text := note
	if text == "" {
		text = jsonx.String(actualLegacy["full_text"])
	}
	title, article := articleOf(actual)
	tweet := &model.Tweet{
		ID: textID(actual["rest_id"]), Text: text, Author: author(actualUser, actualUserLegacy),
		Metrics: model.Metrics{
			Likes: jsonx.Int(actualLegacy["favorite_count"], 0), Retweets: jsonx.Int(actualLegacy["retweet_count"], 0),
			Replies: jsonx.Int(actualLegacy["reply_count"], 0), Quotes: jsonx.Int(actualLegacy["quote_count"], 0),
			Views: jsonx.Int(jsonx.Get(actual, "views", "count"), 0), Bookmarks: jsonx.Int(actualLegacy["bookmark_count"], 0),
		},
		CreatedAt: jsonx.String(actualLegacy["created_at"]), Media: mediaOf(actualLegacy), URLs: urls,
		IsRetweet: isRetweet, Lang: jsonx.String(actualLegacy["lang"]), RetweetedBy: retweetedBy, QuotedTweet: quoted,
		InReplyToStatusID: firstID(actualLegacy["in_reply_to_status_id_str"], actualLegacy["in_reply_to_status_id"]),
		InReplyToUserID:   firstID(actualLegacy["in_reply_to_user_id_str"], actualLegacy["in_reply_to_user_id"]),
		ConversationID:    firstID(actualLegacy["conversation_id_str"], actualLegacy["conversation_id"]),
		QuotedStatusID:    firstID(actualLegacy["quoted_status_id_str"], actualLegacy["quoted_status_id"]),
		SelfThreadID:      firstID(jsonx.Get(actualLegacy, "self_thread", "id_str"), jsonx.Get(actualLegacy, "self_thread", "id")),
		ArticleTitle:      title, ArticleText: article,
	}
	if isRetweet {
		tweet.IsSubscriberOnly = subscriber || retweetSubscriber
	} else {
		tweet.IsSubscriberOnly = subscriber
	}
	return tweet
}

func unwrap(result map[string]any) (map[string]any, bool) {
	if result == nil {
		return map[string]any{}, false
	}
	if jsonx.String(result["__typename"]) == "TweetWithVisibilityResults" {
		if inner, ok := result["tweet"].(map[string]any); ok {
			return inner, jsonx.Bool(result["tweetInterstitial"])
		}
	}
	return result, false
}

func author(user, legacy map[string]any) model.Author {
	if user == nil {
		user = map[string]any{}
	}
	if legacy == nil {
		legacy = map[string]any{}
	}
	core, _ := user["core"].(map[string]any)
	avatar, _ := user["avatar"].(map[string]any)
	name := firstString(core["name"], legacy["name"], user["name"])
	if name == "" {
		name = "Unknown"
	}
	screen := firstString(core["screen_name"], legacy["screen_name"], user["screen_name"])
	if screen == "" {
		screen = "unknown"
	}
	return model.Author{
		ID: textID(user["rest_id"]), Name: name, ScreenName: screen,
		ProfileImageURL: firstString(avatar["image_url"], legacy["profile_image_url_https"]),
		Verified:        jsonx.Bool(user["is_blue_verified"]) || jsonx.Bool(legacy["verified"]),
	}
}

func mediaOf(legacy map[string]any) []model.TweetMedia {
	var out []model.TweetMedia
	for _, raw := range asList(jsonx.Get(legacy, "extended_entities", "media")) {
		item, _ := raw.(map[string]any)
		kind := jsonx.String(item["type"])
		width, height := size(item)
		switch kind {
		case "photo":
			out = append(out, model.TweetMedia{Type: "photo", URL: jsonx.String(item["media_url_https"]), Width: width, Height: height})
		case "video", "animated_gif":
			best := jsonx.String(item["media_url_https"])
			bitrate := -1
			variants, _ := jsonx.Get(item, "video_info", "variants").([]any)
			for _, rawVariant := range variants {
				variant, _ := rawVariant.(map[string]any)
				if jsonx.String(variant["content_type"]) != "video/mp4" {
					continue
				}
				rate := jsonx.Int(variant["bitrate"], 0)
				if rate > bitrate {
					bitrate = rate
					best = jsonx.String(variant["url"])
				}
			}
			out = append(out, model.TweetMedia{Type: kind, URL: best, Width: width, Height: height})
		}
	}
	return out
}

func size(item map[string]any) (*int, *int) {
	w := jsonx.Get(item, "original_info", "width")
	h := jsonx.Get(item, "original_info", "height")
	if w == nil && h == nil {
		return nil, nil
	}
	var wp, hp *int
	if w != nil {
		n := jsonx.Int(w, 0)
		wp = &n
	}
	if h != nil {
		n := jsonx.Int(h, 0)
		hp = &n
	}
	return wp, hp
}

func articleOf(tweet map[string]any) (*string, *string) {
	article, _ := jsonx.Get(tweet, "article", "article_results", "result").(map[string]any)
	if article == nil {
		return nil, nil
	}
	var title *string
	if raw, ok := article["title"].(string); ok {
		title = &raw
	}
	content, _ := article["content_state"].(map[string]any)
	blocks, _ := content["blocks"].([]any)
	if len(blocks) == 0 {
		return title, nil
	}
	entities := entityMap(content["entityMap"])
	mediaURLs := articleMedia(article)
	var parts []string
	ordered := 0
	for _, raw := range blocks {
		block, _ := raw.(map[string]any)
		kind := jsonx.String(block["type"])
		if kind == "" {
			kind = "unstyled"
		}
		if kind == "atomic" {
			parts = append(parts, atomicMarkdown(block, entities)...)
			parts = append(parts, articleImages(block, entities, mediaURLs)...)
			ordered = 0
			continue
		}
		text := renderText(block, entities)
		if text == "" {
			continue
		}
		if kind != "ordered-list-item" {
			ordered = 0
		}
		switch kind {
		case "header-one":
			parts = append(parts, "# "+text)
		case "header-two":
			parts = append(parts, "## "+text)
		case "header-three":
			parts = append(parts, "### "+text)
		case "blockquote":
			parts = append(parts, "> "+text)
		case "unordered-list-item":
			parts = append(parts, "- "+text)
		case "ordered-list-item":
			ordered++
			parts = append(parts, fmt.Sprintf("%d. %s", ordered, text))
		case "code-block":
			parts = append(parts, "```\n"+text+"\n```")
		default:
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return title, nil
	}
	body := strings.Join(parts, "\n\n")
	return title, &body
}

func entityMap(raw any) map[string]any {
	if obj, ok := raw.(map[string]any); ok {
		out := map[string]any{}
		for key, value := range obj {
			out[key] = value
		}
		return out
	}
	out := map[string]any{}
	for _, item := range asList(raw) {
		obj, _ := item.(map[string]any)
		if obj == nil || obj["key"] == nil || obj["value"] == nil {
			continue
		}
		out[fmt.Sprint(obj["key"])] = obj["value"]
	}
	return out
}

func articleMedia(article map[string]any) map[string]string {
	out := map[string]string{}
	var candidates []any
	if cover := article["cover_media"]; cover != nil {
		candidates = append(candidates, cover)
	}
	candidates = append(candidates, asList(article["media_entities"])...)
	for _, raw := range candidates {
		media, _ := raw.(map[string]any)
		if media == nil {
			continue
		}
		info, _ := media["media_info"].(map[string]any)
		image := findImage(info)
		if image == "" {
			image = findImage(media)
		}
		if image == "" {
			continue
		}
		for _, key := range []string{"media_id", "media_key", "id"} {
			if id := jsonx.String(media[key]); id != "" {
				out[id] = image
			}
		}
	}
	return out
}

func atomicMarkdown(block map[string]any, entities map[string]any) []string {
	var parts []string
	for _, raw := range asList(block["entityRanges"]) {
		rng, _ := raw.(map[string]any)
		entity, _ := entities[fmt.Sprint(rng["key"])].(map[string]any)
		if entity == nil || strings.ToUpper(jsonx.String(entity["type"])) != "MARKDOWN" {
			continue
		}
		if text := strings.TrimSpace(jsonx.String(jsonx.Get(entity, "data", "markdown"))); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

func renderText(block map[string]any, entities map[string]any) string {
	text := jsonx.String(block["text"])
	if text == "" {
		return ""
	}
	type link struct {
		offset, length int
		url            string
	}
	var links []link
	for _, raw := range asList(block["entityRanges"]) {
		rng, _ := raw.(map[string]any)
		entity, _ := entities[fmt.Sprint(rng["key"])].(map[string]any)
		if entity == nil || strings.ToUpper(jsonx.String(entity["type"])) != "LINK" {
			continue
		}
		offset, length := jsonx.Int(rng["offset"], -1), jsonx.Int(rng["length"], 0)
		url := strings.TrimSpace(jsonx.String(jsonx.Get(entity, "data", "url")))
		if offset < 0 || length <= 0 || url == "" {
			continue
		}
		links = append(links, link{offset, length, url})
	}
	sort.Slice(links, func(i, j int) bool { return links[i].offset > links[j].offset })
	runes := []rune(text)
	for _, item := range links {
		if item.offset+item.length > len(runes) {
			continue
		}
		label := string(runes[item.offset : item.offset+item.length])
		if label == "" {
			continue
		}
		safeLabel := strings.NewReplacer("[", "\\[", "]", "\\]").Replace(label)
		safeURL := strings.ReplaceAll(item.url, ")", "%29")
		replacement := []rune(fmt.Sprintf("[%s](%s)", safeLabel, safeURL))
		runes = append(runes[:item.offset], append(replacement, runes[item.offset+item.length:]...)...)
	}
	return string(runes)
}

func articleImages(block map[string]any, entities map[string]any, mediaURLs map[string]string) []string {
	var parts []string
	for _, raw := range asList(block["entityRanges"]) {
		rng, _ := raw.(map[string]any)
		entity, _ := entities[fmt.Sprint(rng["key"])].(map[string]any)
		if entity == nil {
			continue
		}
		image := findImage(entity)
		if image == "" {
			for _, item := range asList(jsonx.Get(entity, "data", "mediaItems")) {
				obj, _ := item.(map[string]any)
				if url := mediaURLs[jsonx.String(obj["mediaId"])]; url != "" {
					image = url
					break
				}
			}
		}
		if image == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("![%s](%s)", findCaption(entity), image))
	}
	return parts
}

func findImage(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"original_img_url", "originalImgUrl", "original_url", "originalUrl", "media_url_https", "mediaUrlHttps", "media_url", "mediaUrl", "url", "src", "uri"} {
			candidate := strings.TrimSpace(jsonx.String(typed[key]))
			lower := strings.ToLower(candidate)
			if strings.HasPrefix(lower, "https://pbs.twimg.com/") || strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") || strings.Contains(lower, ".png") || strings.Contains(lower, ".gif") || strings.Contains(lower, ".webp") {
				return candidate
			}
		}
		for _, nested := range typed {
			if found := findImage(nested); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findImage(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func findCaption(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"caption", "alt", "alt_text", "altText", "title", "name"} {
			if text := strings.TrimSpace(jsonx.String(typed[key])); text != "" {
				return text
			}
		}
		for _, nested := range typed {
			if found := findCaption(nested); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findCaption(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func cursorValue(content map[string]any) string {
	if jsonx.String(content["cursorType"]) == "Bottom" {
		return jsonx.String(content["value"])
	}
	return ""
}

// Cursors lists cursor values grouped by cursorType. Order follows the instruction tree.
func Cursors(instructions []any) map[string][]string {
	found := map[string][]string{}
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			kind := jsonx.String(typed["cursorType"])
			item := jsonx.String(typed["value"])
			if kind != "" && item != "" {
				key := kind + "\n" + item
				if !seen[key] {
					seen[key] = true
					found[kind] = append(found[kind], item)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(instructions)
	return found
}

func promoted(entry, item map[string]any) bool {
	id := jsonx.String(entry["entryId"])
	return strings.HasPrefix(id, "promoted-") || item["promotedMetadata"] != nil
}

func asList(v any) []any {
	items, _ := v.([]any)
	return items
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := jsonx.String(value); text != "" {
			return text
		}
	}
	return ""
}

func firstID(values ...any) string {
	for _, value := range values {
		if id := textID(value); id != "" {
			return id
		}
	}
	return ""
}

func textID(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case float64:
		return fmt.Sprintf("%.0f", n)
	case nil:
		return ""
	default:
		return fmt.Sprint(n)
	}
}

// UsersFromInstructions parses a followers or following page.
func UsersFromInstructions(instructions []any) ([]model.UserProfile, string) {
	var users []model.UserProfile
	var cursor string
	for _, raw := range instructions {
		instruction, _ := raw.(map[string]any)
		for _, rawEntry := range asList(instruction["entries"]) {
			entry, _ := rawEntry.(map[string]any)
			content, _ := entry["content"].(map[string]any)
			switch jsonx.String(content["entryType"]) {
			case "TimelineTimelineItem":
				item, _ := content["itemContent"].(map[string]any)
				if result, ok := jsonx.Get(item, "user_results", "result").(map[string]any); ok {
					if user := User(result); user != nil {
						users = append(users, *user)
					}
				}
			case "TimelineTimelineCursor":
				if jsonx.String(content["cursorType"]) == "Bottom" {
					cursor = jsonx.String(content["value"])
				}
			}
		}
	}
	return users, cursor
}
