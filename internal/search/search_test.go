package search

import "testing"

func TestBuild(t *testing.T) {
	likes := 10
	got, err := Build(Query{Text: "golang", From: "@ada", Lang: "en", Since: "2026-01-01", Has: []string{"links"}, Exclude: []string{"retweets"}, MinLikes: &likes})
	if err != nil {
		t.Fatal(err)
	}
	want := "golang from:ada lang:en since:2026-01-01 filter:links -filter:retweets min_faves:10"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if _, err := Build(Query{Lang: "1"}); err == nil {
		t.Fatal("bad lang accepted")
	}
}
