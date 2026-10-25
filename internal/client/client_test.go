package client

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yoyooyooo/twitter-use/internal/accounts"
)

type scripted struct{ calls []script }

type script struct {
	status int
	body   string
	reset  string
}

func (s *scripted) Do(req *http.Request) (*http.Response, error) {
	if len(s.calls) == 0 {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("no script")), Header: http.Header{}, Request: req}, nil
	}
	next := s.calls[0]
	s.calls = s.calls[1:]
	header := http.Header{}
	if next.reset != "" {
		header.Set("x-rate-limit-reset", next.reset)
	}
	return &http.Response{StatusCode: next.status, Body: io.NopCloser(strings.NewReader(next.body)), Header: header, Request: req}, nil
}

func TestSwitchesAccountOn429AndRetriesSameRequest(t *testing.T) {
	list, err := accounts.ParseAccounts("alpha ct0a\nbeta ct0b\n")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := accounts.New(list, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	transport := &scripted{calls: []script{
		{status: 429, body: "slow", reset: "4102444800"},
		{status: 200, body: `{"data":{"ok":true}}`},
	}}
	api, err := New(Credentials{AuthToken: "alpha", CT0: "ct0a"}, pool, Rate{MaxRetries: 0}, WithHTTP(transport), WithoutTransaction(), WithSleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	data, err := api.api("https://x.com/i/api/graphql/abc/TweetDetail", http.MethodGet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if data["data"] == nil {
		t.Fatalf("%#v", data)
	}
	if api.current == nil || api.current.Label != list[1].Label {
		t.Fatalf("stuck on %#v", api.current)
	}
	if len(transport.calls) != 0 {
		t.Fatal("unused script")
	}
}

func TestTweetDetailFollowsShowMoreThreads(t *testing.T) {
	first := timelineBody("focal", "ShowMoreThreads", "more")
	reply := timelineBody("reply", "", "")
	transport := &scripted{calls: []script{{status: 200, body: first}, {status: 200, body: reply}}}
	api, err := New(Credentials{AuthToken: "alpha", CT0: "ct0a"}, nil, Rate{}, WithHTTP(transport), WithoutTransaction(), WithSleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	tweets, err := api.TweetDetail("focal", 10, "Relevance", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tweets) != 2 || tweets[0].ID != "focal" || tweets[1].ID != "reply" {
		t.Fatalf("%#v", tweets)
	}
	if len(transport.calls) != 0 {
		t.Fatal("unused script")
	}
}

func timelineBody(id, cursorType, cursor string) string {
	cursorEntry := ""
	if cursorType != "" {
		cursorEntry = `,{"entryId":"cursor","content":{"cursorType":"` + cursorType + `","value":"` + cursor + `"}}`
	}
	return `{"data":{"tweetResult":{"result":{"timeline":{"instructions":[{"entries":[` +
		`{"entryId":"tweet-` + id + `","content":{"itemContent":{"tweet_results":{"result":{"rest_id":"` + id + `","core":{"user_results":{"result":{"rest_id":"9","core":{"screen_name":"ada","name":"Ada"}}}},"legacy":{"full_text":"` + id + `","conversation_id_str":"focal"}}}}}}` +
		cursorEntry +
		`]}]}}}}}`
}

func TestCeiling(t *testing.T) {
	if _, err := EnforceCount(501); err == nil {
		t.Fatal("ceiling accepted 501")
	}
	if n, err := EnforceCount(500); err != nil || n != 500 {
		t.Fatal(n, err)
	}
}

func TestEmptyPagesStop(t *testing.T) {
	body := `{"data":{"search_by_raw_query":{"search_timeline":{"timeline":{"instructions":[{"entries":[{"content":{"cursorType":"Bottom","value":"c1"}}]}]}}}}}`
	next := `{"data":{"search_by_raw_query":{"search_timeline":{"timeline":{"instructions":[{"entries":[{"content":{"cursorType":"Bottom","value":"c2"}}]}]}}}}}`
	transport := &scripted{calls: []script{{status: 200, body: body}, {status: 200, body: next}}}
	api, err := New(Credentials{AuthToken: "alpha", CT0: "ct0a"}, nil, Rate{}, WithHTTP(transport), WithoutTransaction(), WithSleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	tweets, err := api.Search("python", 10, "Latest", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tweets) != 0 || api.FetchStatus().Stop != "empty_page" || api.FetchStatus().NextCursor != "" {
		t.Fatalf("%#v %d", api.FetchStatus(), len(tweets))
	}
}
