package geoip

import "testing"

// With no databases the resolver is disabled and every lookup is empty --
// the graceful-degradation path the server relies on.
func TestDisabledResolver(t *testing.T) {
	r := Open("", "")
	if r.Enabled() {
		t.Fatal("resolver with no dbs should be disabled")
	}
	loc := r.Lookup("8.8.8.8")
	if loc.City != "" || loc.Region != "" || loc.Country != "" || loc.Provider != "" {
		t.Fatalf("disabled lookup returned data: %+v", loc)
	}
	if l := r.Lookup("not-an-ip"); l.City != "" {
		t.Fatal("bad ip should be empty")
	}
}

func TestPick(t *testing.T) {
	if got := pick(map[string]string{"en": "England", "de": "X"}); got != "England" {
		t.Fatalf("pick en = %q", got)
	}
	if got := pick(map[string]string{"de": "Only"}); got != "Only" {
		t.Fatalf("pick fallback = %q", got)
	}
	if got := pick(nil); got != "" {
		t.Fatalf("pick nil = %q", got)
	}
}
