package model

// SplitThread separates the requested post, the ancestor chain, and the other posts.
// Ancestors are ordered from the root toward the parent. Posts that are not in the
// returned set are omitted rather than invented.
func SplitThread(tweets []Tweet, focalID string) (focal *Tweet, ancestors []Tweet, replies []Tweet) {
	ancestors = []Tweet{}
	replies = []Tweet{}
	byID := make(map[string]Tweet, len(tweets))
	for _, tweet := range tweets {
		if tweet.ID != "" {
			byID[tweet.ID] = tweet
		}
	}
	if tweet, ok := byID[focalID]; ok {
		copy := tweet
		focal = &copy
	}
	seen := map[string]bool{}
	if focalID != "" {
		seen[focalID] = true
	}
	var chain []Tweet
	parent := ""
	if focal != nil {
		parent = focal.InReplyToStatusID
	}
	for parent != "" && !seen[parent] {
		tweet, ok := byID[parent]
		if !ok {
			break
		}
		chain = append(chain, tweet)
		seen[parent] = true
		parent = tweet.InReplyToStatusID
	}
	for i := len(chain) - 1; i >= 0; i-- {
		ancestors = append(ancestors, chain[i])
	}
	for _, tweet := range tweets {
		if tweet.ID == "" || seen[tweet.ID] {
			continue
		}
		replies = append(replies, tweet)
		seen[tweet.ID] = true
	}
	return focal, ancestors, replies
}
