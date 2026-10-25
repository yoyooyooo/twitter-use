package parser

import "testing"

func TestTweetAndTimeline(t *testing.T) {
	result := map[string]any{
		"rest_id": "1",
		"core": map[string]any{"user_results": map[string]any{"result": map[string]any{
			"rest_id": "9", "is_blue_verified": true,
			"core": map[string]any{"name": "Ada", "screen_name": "ada"},
		}}},
		"legacy": map[string]any{
			"full_text": "hello", "created_at": "Sat Mar 08 12:00:00 +0000 2026",
			"favorite_count": "2", "retweet_count": 1, "reply_count": 3, "quote_count": 0, "bookmark_count": 4, "lang": "en",
			"in_reply_to_status_id_str": "7", "in_reply_to_user_id_str": "8", "conversation_id_str": "6",
			"quoted_status_id_str": "5", "self_thread": map[string]any{"id_str": "4"},
			"entities": map[string]any{"urls": []any{map[string]any{"expanded_url": "https://example.com"}}},
		},
		"views": map[string]any{"count": "10"},
	}
	data := map[string]any{
		"data": map[string]any{"home": map[string]any{"home_timeline_urt": map[string]any{
			"instructions": []any{map[string]any{"entries": []any{
				map[string]any{"entryId": "tweet-1", "content": map[string]any{"itemContent": map[string]any{"tweet_results": map[string]any{"result": result}}}},
				map[string]any{"entryId": "cursor", "content": map[string]any{"cursorType": "Bottom", "value": "next"}},
			}}},
		}}},
	}
	tweets, cursor := Timeline(data, Instructions(data, "data", "home", "home_timeline_urt", "instructions"))
	if cursor != "next" || len(tweets) != 1 {
		t.Fatalf("tweets=%d cursor=%s", len(tweets), cursor)
	}
	tweet := tweets[0]
	if tweet.Author.ScreenName != "ada" || tweet.Text != "hello" || tweet.Metrics.Likes != 2 || tweet.Metrics.Views != 10 || tweet.URLs[0] != "https://example.com" || !tweet.Author.Verified {
		t.Fatalf("%#v", tweet)
	}
	if tweet.InReplyToStatusID != "7" || tweet.InReplyToUserID != "8" || tweet.ConversationID != "6" || tweet.QuotedStatusID != "5" || tweet.SelfThreadID != "4" {
		t.Fatalf("relations=%s %s %s %s %s", tweet.InReplyToStatusID, tweet.InReplyToUserID, tweet.ConversationID, tweet.QuotedStatusID, tweet.SelfThreadID)
	}
	cursors := Cursors(Instructions(data, "data", "home", "home_timeline_urt", "instructions"))
	if len(cursors["Bottom"]) != 1 || cursors["Bottom"][0] != "next" {
		t.Fatalf("cursors=%v", cursors)
	}
}

func TestLongNoteAndRetweet(t *testing.T) {
	raw := map[string]any{
		"rest_id": "2",
		"core":    map[string]any{"user_results": map[string]any{"result": map[string]any{"rest_id": "1", "legacy": map[string]any{"screen_name": "bob", "name": "Bob"}}}},
		"legacy": map[string]any{
			"full_text": "short", "created_at": "x",
			"retweeted_status_result": map[string]any{"result": map[string]any{
				"rest_id":    "3",
				"core":       map[string]any{"user_results": map[string]any{"result": map[string]any{"rest_id": "4", "core": map[string]any{"screen_name": "ada", "name": "Ada"}}}},
				"legacy":     map[string]any{"full_text": "original", "favorite_count": 1},
				"note_tweet": map[string]any{"note_tweet_results": map[string]any{"result": map[string]any{"text": "full note"}}},
			}},
		},
	}
	tweet := Tweet(raw, 0)
	if tweet == nil || !tweet.IsRetweet || tweet.Text != "full note" || tweet.RetweetedBy != "bob" || tweet.Author.ScreenName != "ada" {
		t.Fatalf("%#v", tweet)
	}
}
