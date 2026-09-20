package mail

import (
	"strings"
	"testing"
)

// The house style, and the rule that the plain-text alternative carries
// the same facts as the HTML: a client that shows no HTML must still be
// able to act on the message.
func TestPageRendersBothWays(t *testing.T) {
	p := Page{
		Product: "Frazile Blog", Title: "Your verification code",
		Intro: "Enter this code to finish verifying your address.", Code: "418207",
		Rows: []Row{{Label: "Asked for from", Value: "1.2.3.4"}},
		Note: "It expires in 10 minutes.", Site: "blog.frazile.com", Contact: "hello@frazile.com",
	}
	html, text := p.HTML(), p.Text()
	for _, want := range []string{"418207", "Frazile Blog", "Your verification code", "1.2.3.4", "blog.frazile.com"} {
		if !strings.Contains(html, want) {
			t.Errorf("the HTML is missing %q", want)
		}
		if !strings.Contains(text, want) {
			t.Errorf("the text is missing %q", want)
		}
	}
	// The spectrum is drawn as cells, so it survives a client that drops
	// gradients, and the layout is a table, for the ones that ignore
	// max-width on a div.
	for _, want := range []string{"#19c6fd", "#ffc21a", "<table", `role="presentation"`} {
		if !strings.Contains(html, want) {
			t.Errorf("the HTML is missing %q", want)
		}
	}
	// Anything a person typed is escaped before it goes out.
	hostile := Page{Title: `<script>alert(1)</script>`, Body: `a & b`}.HTML()
	if strings.Contains(hostile, "<script>") || !strings.Contains(hostile, "&amp;") {
		t.Error("content must be escaped")
	}
}
