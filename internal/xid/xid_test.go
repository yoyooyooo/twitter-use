package xid

import "testing"

func TestOnDemandURLReadsWebpackHash(t *testing.T) {
	page := `59924:"ondemand.s",60041:"i18n/emoji",59924:"4b0d8ef8e73c2f3d"`
	got := OnDemandURL(page)
	want := "https://abs.twimg.com/responsive-web/client-web/ondemand.s.4b0d8ef8e73c2f3da.js"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestOnDemandURLReadsLegacyAssignment(t *testing.T) {
	page := `"ondemand.s":"abcdef1234567890"`
	got := OnDemandURL(page)
	want := "https://abs.twimg.com/responsive-web/client-web/ondemand.s.abcdef1234567890a.js"
	if got != want {
		t.Fatalf("got %s", got)
	}
}
