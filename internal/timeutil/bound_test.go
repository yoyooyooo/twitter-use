package timeutil

import "testing"

func TestAdmitStopsAtSince(t *testing.T) {
	since, until, err := ParseWindow("2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	keep, stop := Admit("Wed Oct 15 12:00:00 +0000 2026", since, until)
	if !keep || stop != "" {
		t.Fatalf("inside window: keep %v stop %q", keep, stop)
	}
	keep, stop = Admit("Thu Nov 05 12:00:00 +0000 2026", since, until)
	if keep || stop != "" {
		t.Fatalf("after until should be skipped: keep %v stop %q", keep, stop)
	}
	keep, stop = Admit("Wed Sep 30 12:00:00 +0000 2026", since, until)
	if keep || stop != "since_reached" {
		t.Fatalf("before since: keep %v stop %q", keep, stop)
	}
}

func TestParseWindowRejectsInvertedRange(t *testing.T) {
	if _, _, err := ParseWindow("2026-11-01", "2026-10-01"); err == nil {
		t.Fatal("expected inverted window to fail")
	}
}
