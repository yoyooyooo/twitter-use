package cli

import (
	"slices"
	"testing"
)

func TestCommandSurface(t *testing.T) {
	root := newRoot()
	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
		for _, child := range cmd.Commands() {
			names = append(names, cmd.Name()+" "+child.Name())
		}
	}
	slices.Sort(names)
	want := []string{
		"article", "bookmark", "bookmarks", "bookmarks folders", "follow", "followers", "following",
		"home", "like", "likes", "posts", "retweet", "search", "skill", "tweet", "unbookmark", "unfollow",
		"unlike", "unretweet", "user", "whoami",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("commands = %v", names)
	}
	gone := []string{"feed", "user-posts", "favorites", "show", "status", "list", "post", "reply", "quote", "delete", "favorite", "unfavorite"}
	have := map[string]bool{}
	for _, name := range names {
		have[name] = true
	}
	for _, name := range gone {
		if have[name] {
			t.Fatalf("removed command still registered: %s", name)
		}
	}
}
