// Package search composes X advanced-search operators.
package search

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var langPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z-]{1,14}$`)

// Query is one advanced search request.
type Query struct {
	Text        string
	From        string
	To          string
	Lang        string
	Since       string
	Until       string
	Has         []string
	Exclude     []string
	MinLikes    *int
	MinRetweets *int
}

// Build returns the rawQuery string.
func Build(q Query) (string, error) {
	from := handle(q.From)
	to := handle(q.To)
	lang, err := language(q.Lang)
	if err != nil {
		return "", err
	}
	since, err := date("--since", q.Since)
	if err != nil {
		return "", err
	}
	until, err := date("--until", q.Until)
	if err != nil {
		return "", err
	}
	if q.MinLikes != nil && *q.MinLikes < 0 {
		return "", fmt.Errorf("--min-likes must be greater than or equal to 0")
	}
	if q.MinRetweets != nil && *q.MinRetweets < 0 {
		return "", fmt.Errorf("--min-retweets must be greater than or equal to 0")
	}
	if since != "" && until != "" && since > until {
		return "", fmt.Errorf("--since must be on or before --until")
	}
	var parts []string
	if text := strings.TrimSpace(q.Text); text != "" {
		parts = append(parts, text)
	}
	if from != "" {
		parts = append(parts, "from:"+from)
	}
	if to != "" {
		parts = append(parts, "to:"+to)
	}
	if lang != "" {
		parts = append(parts, "lang:"+lang)
	}
	if since != "" {
		parts = append(parts, "since:"+since)
	}
	if until != "" {
		parts = append(parts, "until:"+until)
	}
	for _, item := range q.Has {
		parts = append(parts, "filter:"+strings.ToLower(item))
	}
	for _, item := range q.Exclude {
		item = strings.ToLower(item)
		switch item {
		case "retweets", "replies", "links":
			parts = append(parts, "-filter:"+item)
		default:
			parts = append(parts, "-filter:"+item)
		}
	}
	if q.MinLikes != nil {
		parts = append(parts, fmt.Sprintf("min_faves:%d", *q.MinLikes))
	}
	if q.MinRetweets != nil {
		parts = append(parts, fmt.Sprintf("min_retweets:%d", *q.MinRetweets))
	}
	return strings.Join(parts, " "), nil
}

func handle(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "@")
}

func language(value string) (string, error) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return "", nil
	}
	if !langPattern.MatchString(text) {
		return "", fmt.Errorf("--lang must be an ISO language code like en or zh-cn")
	}
	return text, nil
}

func date(flag, value string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", text); err != nil {
		return "", fmt.Errorf("%s must be in YYYY-MM-DD format", flag)
	}
	return text, nil
}
