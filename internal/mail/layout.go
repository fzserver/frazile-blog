package mail

import (
	"fmt"
	"html"
	"strings"
)

// The house style for every e-mail Frazile sends, so a code from Drive, a
// receipt from Walls and a reset from the blog are recognisably the same
// hand: the logo's spectrum as a rule across the top, the product's name,
// one thing asked or told, and small print that says what happens if it was
// not you.
//
// Built for mail clients, not browsers. A table does the centring, because
// Outlook ignores max-width on a div; every colour is inline, because most
// clients strip stylesheets; the dark-mode block is a courtesy for the
// clients that honour it and harmless in the ones that do not. No images:
// a logo would be blocked by default in half the clients and the spectrum
// rule survives everywhere.
//
// Page carries the content. Both renderings come from the same value, so
// the plain-text alternative can never drift from the HTML.

type Page struct {
	Product string // "Frazile Drive", "Frazile Walls", "Frazile Blog"
	Title   string // the one line that says what this is
	Intro   string // a sentence or two of prose
	Code    string // a verification code, shown large
	Button  *Button
	Rows    []Row  // details: an order, an invoice, a message's sender
	Body    string // more prose, after the rows
	Note    string // the small print at the end
	Contact string // "hello@frazile.com", shown in the footer
	Site    string // "drive.frazile.com", shown in the footer
}

type Button struct {
	Label string
	URL   string
}

type Row struct {
	Label string
	Value string
}

const (
	inkColour   = "#0d1530"
	bodyColour  = "#3a4560"
	mutedColour = "#646f87"
	lineColour  = "#e2e7f1"
	paperColour = "#f3f5fa"
	blueColour  = "#1f5bff"
	spectrum    = "linear-gradient(90deg,#19c6fd,#1f5bff,#7b3ff2,#e8399f,#ff6a1a,#ffc21a)"
)

// HTML renders the message.
func (p Page) HTML() string {
	esc := html.EscapeString
	var b strings.Builder

	b.WriteString(`<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<meta name="color-scheme" content="light dark">` +
		`<style>@media (prefers-color-scheme:dark){` +
		`.fz-bg{background:#050a18!important}.fz-card{background:#0e1528!important;border-color:#1e2842!important}` +
		`.fz-ink{color:#f5f8fd!important}.fz-body{color:#c3cad8!important}.fz-muted{color:#8a95ab!important}` +
		`.fz-panel{background:#0b1226!important;border-color:#1e2842!important}}</style></head>`)

	b.WriteString(`<body class="fz-bg" style="margin:0;padding:0;background:` + paperColour +
		`;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" class="fz-bg" style="background:` +
		paperColour + `;padding:28px 12px"><tr><td align="center">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" class="fz-card" style="max-width:480px;background:#ffffff;border:1px solid ` +
		lineColour + `;border-radius:16px;overflow:hidden">`)

	// The spectrum rule, as a row of coloured cells so it survives the
	// clients that drop gradients.
	b.WriteString(`<tr><td style="padding:0"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:` +
		spectrum + `"><tr>`)
	for _, c := range []string{"#19c6fd", "#1f5bff", "#7b3ff2", "#e8399f", "#ff6a1a", "#ffc21a"} {
		b.WriteString(`<td height="4" style="height:4px;line-height:4px;font-size:0;background:` + c + `">&nbsp;</td>`)
	}
	b.WriteString(`</tr></table></td></tr>`)

	b.WriteString(`<tr><td style="padding:28px 28px 30px">`)

	if p.Product != "" {
		b.WriteString(`<p class="fz-muted" style="margin:0 0 18px;font-size:13px;font-weight:600;letter-spacing:.04em;color:` +
			mutedColour + `">` + esc(p.Product) + `</p>`)
	}
	if p.Title != "" {
		b.WriteString(`<h1 class="fz-ink" style="margin:0 0 12px;font-size:21px;line-height:1.25;font-weight:650;color:` +
			inkColour + `">` + esc(p.Title) + `</h1>`)
	}
	if p.Intro != "" {
		b.WriteString(`<p class="fz-body" style="margin:0 0 20px;font-size:15px;line-height:1.55;color:` +
			bodyColour + `">` + esc(p.Intro) + `</p>`)
	}
	if p.Code != "" {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" class="fz-panel" style="background:` +
			paperColour + `;border:1px solid ` + lineColour + `;border-radius:12px;margin:0 0 20px"><tr><td align="center" style="padding:18px 12px">` +
			`<span style="font-size:32px;font-weight:700;letter-spacing:.28em;color:` + blueColour +
			`;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace">` + esc(p.Code) + `</span></td></tr></table>`)
	}
	if p.Button != nil {
		b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 20px"><tr><td align="center" style="background:` +
			blueColour + `;border-radius:12px"><a href="` + esc(p.Button.URL) +
			`" style="display:inline-block;padding:13px 26px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none">` +
			esc(p.Button.Label) + `</a></td></tr></table>`)
	}
	if len(p.Rows) > 0 {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 20px;font-size:14px">`)
		for _, r := range p.Rows {
			b.WriteString(`<tr><td class="fz-muted" style="padding:5px 14px 5px 0;color:` + mutedColour +
				`;white-space:nowrap;vertical-align:top">` + esc(r.Label) + `</td>` +
				`<td class="fz-body" style="padding:5px 0;color:` + bodyColour + `">` + esc(r.Value) + `</td></tr>`)
		}
		b.WriteString(`</table>`)
	}
	if p.Body != "" {
		b.WriteString(`<p class="fz-body" style="margin:0 0 20px;font-size:15px;line-height:1.55;color:` +
			bodyColour + `">` + esc(p.Body) + `</p>`)
	}
	if p.Note != "" {
		b.WriteString(`<p class="fz-muted" style="margin:0;font-size:13px;line-height:1.5;color:` +
			mutedColour + `">` + esc(p.Note) + `</p>`)
	}

	b.WriteString(`</td></tr>`)

	if p.Site != "" || p.Contact != "" {
		b.WriteString(`<tr><td class="fz-panel" style="padding:14px 28px;border-top:1px solid ` + lineColour +
			`;background:` + paperColour + `"><p class="fz-muted" style="margin:0;font-size:12px;color:` + mutedColour + `">`)
		if p.Site != "" {
			b.WriteString(esc(p.Site))
		}
		if p.Site != "" && p.Contact != "" {
			b.WriteString(` &middot; `)
		}
		if p.Contact != "" {
			b.WriteString(`<a href="mailto:` + esc(p.Contact) + `" style="color:` + mutedColour + `">` + esc(p.Contact) + `</a>`)
		}
		b.WriteString(`</p></td></tr>`)
	}

	b.WriteString(`</table></td></tr></table></body></html>`)
	return b.String()
}

// Text is the same message for a client that shows no HTML, and for the
// spam filters that mistrust a message without one.
func (p Page) Text() string {
	var b strings.Builder
	if p.Product != "" {
		b.WriteString(p.Product + "\n\n")
	}
	if p.Title != "" {
		b.WriteString(p.Title + "\n\n")
	}
	if p.Intro != "" {
		b.WriteString(p.Intro + "\n\n")
	}
	if p.Code != "" {
		b.WriteString("    " + p.Code + "\n\n")
	}
	if p.Button != nil {
		b.WriteString(p.Button.Label + ": " + p.Button.URL + "\n\n")
	}
	for _, r := range p.Rows {
		b.WriteString(fmt.Sprintf("%-12s %s\n", r.Label, r.Value))
	}
	if len(p.Rows) > 0 {
		b.WriteString("\n")
	}
	if p.Body != "" {
		b.WriteString(p.Body + "\n\n")
	}
	if p.Note != "" {
		b.WriteString(p.Note + "\n")
	}
	if p.Site != "" || p.Contact != "" {
		b.WriteString("\n— " + strings.TrimSpace(p.Site+" "+p.Contact) + "\n")
	}
	return b.String()
}

// Message turns a page into something the sender takes.
func (p Page) Message(to, subject, replyTo string) Message {
	return Message{To: to, Subject: subject, ReplyTo: replyTo, Text: p.Text(), HTML: p.HTML()}
}
