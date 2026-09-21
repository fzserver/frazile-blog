package mail

import (
	"strings"
	"testing"
)

// A code belongs in the body only. Subjects are shown on lock screens, in
// notification banners and in mail logs, to people who are not the reader.
func TestNoCodeInAnySubject(t *testing.T) {
	const code = "481516"
	for _, purpose := range []string{"verify", "reset", "email", "login"} {
		subject, text, html := Code("Frazile Blog", purpose, code, 10)
		if strings.Contains(subject, code) {
			t.Errorf("%s: the code is in the subject %q", purpose, subject)
		}
		if subject == "" || !strings.Contains(text, code) || !strings.Contains(html, code) {
			t.Errorf("%s: the code must still be in the body (subject %q)", purpose, subject)
		}
	}
}
