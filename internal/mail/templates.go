package mail

import (
	"fmt"
	"html"
)

// Code renders the verification e-mail for one purpose. Plain text and a
// minimal HTML twin; the code is the only thing that matters.
func Code(site, purpose, code string, ttlMin int) (subject, text, htmlBody string) {
	var lead string
	switch purpose {
	case "reset":
		subject = fmt.Sprintf("%s: your password reset code", site)
		lead = "Use this code to reset your password."
	case "email":
		subject = fmt.Sprintf("%s: confirm your new e-mail address", site)
		lead = "Use this code to confirm your new e-mail address."
	default:
		subject = fmt.Sprintf("%s: your verification code", site)
		lead = "Use this code to verify your e-mail address and finish signing up."
	}
	text = fmt.Sprintf("%s\n\n    %s\n\nThe code is valid for %d minutes. If you did not request it, ignore this message.\n\n— %s\n", lead, code, ttlMin, site)
	htmlBody = fmt.Sprintf(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;max-width:480px;margin:0 auto;padding:24px;color:#222">
<p style="font-size:16px">%s</p>
<p style="font-size:32px;font-weight:700;letter-spacing:8px;text-align:center;padding:16px;background:#f3f1ff;border-radius:8px;margin:20px 0">%s</p>
<p style="font-size:14px;color:#555">The code is valid for %d minutes. If you did not request it, you can ignore this message.</p>
<p style="font-size:13px;color:#888">— %s</p></div>`, html.EscapeString(lead), html.EscapeString(code), ttlMin, html.EscapeString(site))
	return
}
