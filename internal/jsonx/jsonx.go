package jsonx

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Get walks nested objects and arrays. Keys are strings or ints.
func Get(v any, keys ...any) any {
	cur := v
	for _, key := range keys {
		switch k := key.(type) {
		case int:
			arr, ok := cur.([]any)
			if !ok || k < 0 || k >= len(arr) {
				return nil
			}
			cur = arr[k]
		case string:
			obj, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = obj[k]
		default:
			return nil
		}
	}
	return cur
}

// Int converts Twitter's mixed number encodings.
func Int(v any, fallback int) int {
	switch n := v.(type) {
	case nil:
		return fallback
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			f, ferr := n.Float64()
			if ferr != nil {
				return fallback
			}
			return int(f)
		}
		return int(i)
	case string:
		text := strings.ReplaceAll(strings.TrimSpace(n), ",", "")
		if text == "" {
			return fallback
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fallback
		}
		return int(f)
	default:
		return fallback
	}
}

// String returns a string value, or empty.
func String(v any) string {
	s, _ := v.(string)
	return s
}

// Bool returns a bool value.
func Bool(v any) bool {
	b, _ := v.(bool)
	return b
}
