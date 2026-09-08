package render

import (
	"strings"
	"testing"
)

func TestSanitise(t *testing.T) {
	out := HTML("# Hi\n\n<script>alert(1)</script>\n\n<img src=\"/media/a.png\" onerror=\"x()\">\n\n<video controls src=\"/media/v.mp4\"></video>\n\n<iframe src=\"https://evil.example/x\"></iframe>\n\n<iframe src=\"https://www.youtube-nocookie.com/embed/abc_123\"></iframe>\n\n[x](javascript:alert(1))")
	for _, bad := range []string{"<script", "onerror", "evil.example", "javascript:"} {
		if strings.Contains(out, bad) {
			t.Errorf("output contains %q:\n%s", bad, out)
		}
	}
	for _, good := range []string{`<h1 id="hi">`, `<video`, `src="/media/v.mp4"`, `youtube-nocookie.com/embed/abc_123`, `loading="lazy"`} {
		if !strings.Contains(out, good) {
			t.Errorf("output lacks %q:\n%s", good, out)
		}
	}
}

func TestExcerptAndReading(t *testing.T) {
	txt := Text("<p>Hello <b>there</b> &amp; welcome</p>")
	if txt != "Hello there & welcome" {
		t.Fatalf("Text = %q", txt)
	}
	if e := Excerpt(strings.Repeat("word ", 100), 40); len(e) > 44 || !strings.HasSuffix(e, "…") {
		t.Fatalf("Excerpt = %q", e)
	}
	if m := ReadingMinutes(strings.Repeat("w ", 500)); m != 3 {
		t.Fatalf("ReadingMinutes = %d", m)
	}
}
