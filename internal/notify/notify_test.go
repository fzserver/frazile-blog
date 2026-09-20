package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The house style is carried in headers, so this checks the ones a phone
// actually uses: the title says which product and what happened, the tag
// draws the icon, the priority decides whether it buzzes, and the click
// opens the page that answers it.
func TestEventCarriesTheHouseStyle(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		got = r
	}))
	defer srv.Close()

	New(srv.URL, nil).SendContext(context.Background(), Event{
		Product: "Frazile Drive", Title: "Plan paid", Tag: TagMoney, Priority: Normal,
		Lines: []string{"Pro · monthly · ₹2,004.82", "asha@example.com"},
		Click: "https://drive.frazile.com/admin/accounts/7",
	})

	if got == nil {
		t.Fatal("nothing was posted")
	}
	if want := "Frazile Drive: Plan paid"; got.Header.Get("Title") != want {
		t.Errorf("Title is %q, want %q", got.Header.Get("Title"), want)
	}
	if got.Header.Get("Tags") != TagMoney || got.Header.Get("Priority") != "3" {
		t.Errorf("Tags %q, Priority %q", got.Header.Get("Tags"), got.Header.Get("Priority"))
	}
	if got.Header.Get("Click") == "" {
		t.Error("an alert should link to the page that answers it")
	}
	// The body may carry anything: it is UTF-8, not a header.
	if body != "Pro · monthly · ₹2,004.82\nasha@example.com" {
		t.Errorf("body is %q", body)
	}
}

// A service with no topic configured must send nothing, and an unreachable
// ntfy must never be the reason something fails.
func TestQuietWithoutATopic(t *testing.T) {
	var n *Notifier
	n.Send(Event{Title: "nothing"})               // a nil notifier
	New("", nil).Send(Event{Title: "nothing"})    // no topic
	New("http://127.0.0.1:1/x", nil).SendContext( // nothing listening
		context.Background(), Event{Title: "unreachable"})
}
