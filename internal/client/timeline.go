package client

import (
	"log"
	"math"

	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/graphql"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/parser"
	"github.com/yoyooyooo/twitter-use/internal/timeutil"
)

type timelineSpec struct {
	operation    string
	count        int
	instructions func(map[string]any) []any
	extra        map[string]any
	override     bool
	post         bool
	bare         bool
	promoted     bool
	toggles      map[string]any
	cursor       string
}

func (c *Client) timeline(spec timelineSpec) ([]model.Tweet, error) {
	count, err := EnforceCount(spec.count)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		c.finish(0, "complete", "")
		return nil, nil
	}
	c.reset(count)
	var tweets []model.Tweet
	seen := map[string]struct{}{}
	cursor := spec.cursor
	continuation := ""
	stop := ""
	emptyPages := 0
	attempts := 0
	maxAttempts := int(math.Ceil(float64(count)/20)) + 2
	for len(tweets) < count && attempts < maxAttempts {
		attempts++
		variables := map[string]any{"count": minCount(count-len(tweets)+5, 40)}
		if !spec.override {
			variables["includePromotedContent"] = spec.promoted
			variables["latestControlAvailable"] = true
			variables["requestContext"] = "launch"
		}
		for key, value := range spec.extra {
			variables[key] = value
		}
		if cursor != "" {
			variables["cursor"] = cursor
		}
		var data map[string]any
		features := graphql.FeatureCopy()
		if spec.bare {
			features = nil
		}
		if spec.post {
			data, err = c.graphqlPost(spec.operation, variables, features)
		} else {
			data, err = c.graphqlGet(spec.operation, variables, graphql.FeatureCopy(), spec.toggles)
		}
		if err != nil {
			if api, ok := err.(*errs.Error); ok && api.Status == 429 && len(tweets) > 0 {
				stop = "rate_limited"
				continuation = cursor
				log.Printf("Rate limit exhausted the account pool; returning %d tweets already fetched", len(tweets))
				break
			}
			return tweets, err
		}
		page, next := parser.Timeline(data, spec.instructions(data))
		added := 0
		skippedNew := 0
		halt := false
		for _, tweet := range page {
			if tweet.ID == "" {
				continue
			}
			if _, ok := seen[tweet.ID]; ok {
				continue
			}
			keep, bound := timeutil.Admit(tweet.CreatedAt, c.since, c.until)
			if bound != "" {
				stop = bound
				continuation = ""
				halt = true
				break
			}
			if !keep {
				skippedNew++
				continue
			}
			seen[tweet.ID] = struct{}{}
			tweets = append(tweets, tweet)
			added++
		}
		if halt {
			break
		}
		if next == "" {
			continuation = ""
			if len(tweets) >= count {
				stop = "complete"
			} else {
				stop = "source_exhausted"
			}
			break
		}
		if next == cursor {
			continuation = ""
			stop = "cursor_stalled"
			break
		}
		continuation = next
		cursor = next
		if added == 0 && skippedNew == 0 {
			emptyPages++
			if emptyPages >= emptyPageLimit {
				continuation = ""
				stop = "empty_page"
				break
			}
		} else if added > 0 {
			emptyPages = 0
		}
		if len(tweets) < count {
			c.pageDelay()
		}
	}
	if stop == "" {
		switch {
		case len(tweets) >= count && continuation != "":
			stop = "max_count"
		case continuation != "":
			stop = "attempt_limit"
		case len(tweets) >= count:
			stop = "complete"
		default:
			stop = "source_exhausted"
		}
	}
	if len(tweets) > count {
		tweets = tweets[:count]
	}
	c.finish(len(tweets), stop, continuation)
	return tweets, nil
}

func (c *Client) users(operation, userID string, count int, cursor string, instructions func(map[string]any) []any, post bool) ([]model.UserProfile, error) {
	count, err := EnforceCount(count)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		c.finish(0, "complete", "")
		return nil, nil
	}
	c.reset(count)
	var users []model.UserProfile
	seen := map[string]struct{}{}
	continuation, stop := "", ""
	emptyPages, attempts := 0, 0
	maxAttempts := int(math.Ceil(float64(count)/20)) + 2
	for len(users) < count && attempts < maxAttempts {
		attempts++
		variables := map[string]any{"userId": userID, "count": minCount(count-len(users)+5, 40), "includePromotedContent": false}
		if cursor != "" {
			variables["cursor"] = cursor
		}
		var data map[string]any
		if post {
			data, err = c.graphqlPost(operation, variables, graphql.FeatureCopy())
		} else {
			data, err = c.graphqlGet(operation, variables, graphql.FeatureCopy(), nil)
		}
		if err != nil {
			if api, ok := err.(*errs.Error); ok && api.Status == 429 && len(users) > 0 {
				stop = "rate_limited"
				continuation = cursor
				break
			}
			return users, err
		}
		list := instructions(data)
		if len(list) == 0 {
			continuation = ""
			if len(users) >= count {
				stop = "complete"
			} else {
				stop = "source_exhausted"
			}
			break
		}
		page, next := parser.UsersFromInstructions(list)
		added := 0
		for _, user := range page {
			if user.ID == "" {
				continue
			}
			if _, ok := seen[user.ID]; ok {
				continue
			}
			seen[user.ID] = struct{}{}
			users = append(users, user)
			added++
		}
		if next == "" {
			continuation = ""
			if len(users) >= count {
				stop = "complete"
			} else {
				stop = "source_exhausted"
			}
			break
		}
		if next == cursor {
			continuation = ""
			stop = "cursor_stalled"
			break
		}
		continuation = next
		cursor = next
		if added == 0 {
			emptyPages++
			if emptyPages >= emptyPageLimit {
				continuation = ""
				stop = "empty_page"
				break
			}
		} else {
			emptyPages = 0
		}
		if len(users) < count {
			c.pageDelay()
		}
	}
	if stop == "" {
		switch {
		case len(users) >= count && continuation != "":
			stop = "max_count"
		case continuation != "":
			stop = "attempt_limit"
		case len(users) >= count:
			stop = "complete"
		default:
			stop = "source_exhausted"
		}
	}
	if len(users) > count {
		users = users[:count]
	}
	c.finish(len(users), stop, continuation)
	return users, nil
}

func (c *Client) reset(requested int) {
	c.fetch = model.FetchStatus{Requested: requested, Stop: "complete"}
}

func (c *Client) finish(returned int, stop, cursor string) {
	if _, ok := resumeStops[stop]; !ok {
		cursor = ""
	}
	c.fetch = model.FetchStatus{Requested: c.fetch.Requested, Returned: returned, Stop: stop, NextCursor: cursor}
}
