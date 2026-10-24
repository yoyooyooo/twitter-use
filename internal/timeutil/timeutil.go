package timeutil

import (
	"fmt"
	"time"
)

const layout = "Mon Jan 02 15:04:05 -0700 2006"

// ParseWindow reads --since and --until. A date is the whole UTC day.
// An RFC3339 value is that instant. Empty values are unset.
func ParseWindow(since, until string) (time.Time, time.Time, error) {
	lower, err := parseBound("--since", since, false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	upper, err := parseBound("--until", until, true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !lower.IsZero() && !upper.IsZero() && lower.After(upper) {
		return time.Time{}, time.Time{}, fmt.Errorf("--since must be on or before --until")
	}
	return lower, upper, nil
}

func parseBound(flag, value string, end bool) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD or RFC3339", flag)
	}
	parsed = parsed.UTC()
	if end {
		return parsed.Add(24*time.Hour - time.Nanosecond), nil
	}
	return parsed, nil
}

// Admit reports whether a reverse-chronological item is inside the window.
// A zero bound is unset. stop is since_reached when the item is older than since.
func Admit(createdAt string, since, until time.Time) (bool, string) {
	if since.IsZero() && until.IsZero() {
		return true, ""
	}
	parsed, ok := parse(createdAt)
	if !ok {
		return true, ""
	}
	if !until.IsZero() && parsed.After(until) {
		return false, ""
	}
	if !since.IsZero() && parsed.Before(since) {
		return false, "since_reached"
	}
	return true, ""
}

func parse(createdAt string) (time.Time, bool) {
	if createdAt == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(layout, createdAt)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// Local formats a Twitter timestamp as local "2006-01-02 15:04".
func Local(createdAt string) string {
	parsed, ok := parse(createdAt)
	if !ok {
		return createdAt
	}
	return parsed.In(time.Local).Format("2006-01-02 15:04")
}

// ISO formats a Twitter timestamp as RFC3339.
func ISO(createdAt string) string {
	parsed, ok := parse(createdAt)
	if !ok {
		return createdAt
	}
	return parsed.Format(time.RFC3339)
}

// Relative formats a Twitter timestamp as "2h ago".
func Relative(createdAt string) string {
	parsed, ok := parse(createdAt)
	if !ok {
		return createdAt
	}
	seconds := int(time.Since(parsed).Seconds())
	if seconds < 0 {
		return "just now"
	}
	if seconds < 60 {
		return itoa(seconds) + "s ago"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return itoa(minutes) + "m ago"
	}
	hours := minutes / 60
	if hours < 24 {
		return itoa(hours) + "h ago"
	}
	days := hours / 24
	if days < 30 {
		return itoa(days) + "d ago"
	}
	months := days / 30
	if months < 12 {
		return itoa(months) + "mo ago"
	}
	return itoa(days/365) + "y ago"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
