// Package render turns post markdown into sanitised HTML. Authors are trusted
// users (admin-promoted), but the sanitiser still runs so a pasted snippet
// cannot smuggle a script in, and so the set of allowed embeds is explicit:
// images, self-hosted video, and YouTube iframes.
package render

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote, extension.Typographer),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe(),
		// Priority below the default HTML renderer (1000) so ours wins for raw HTML nodes.
		renderer.WithNodeRenderers(util.Prioritized(&rawHTMLRenderer{}, 100))),
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[a-zA-Z0-9_\-:.]+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6", "sup", "li", "div", "section")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(language-[a-zA-Z0-9_+-]+|footnotes|footnote-ref|footnote-backref|task-list-item|align-(left|center|right)|embed|wide)$`)).OnElements("code", "pre", "a", "li", "div", "section", "p", "figure", "img")
	p.AllowAttrs("role").Matching(regexp.MustCompile(`^doc-endnotes$`)).OnElements("div", "section")
	p.AllowAttrs("align").Matching(regexp.MustCompile(`^(left|center|right)$`)).OnElements("td", "th", "p")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowElements("figure", "figcaption", "video", "source", "iframe", "details", "summary", "mark", "kbd", "sub", "sup")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^(/media/[^\s"']+|https://[^\s"']+)$`)).OnElements("video", "source", "img")
	p.AllowAttrs("poster").Matching(regexp.MustCompile(`^(/media/[^\s"']+|https://[^\s"']+)$`)).OnElements("video")
	p.AllowAttrs("controls", "loop", "muted", "autoplay", "playsinline", "preload", "width", "height").OnElements("video")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^video/(mp4|webm|ogg|quicktime)$`)).OnElements("source")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^https://(www\.)?(youtube\.com|youtube-nocookie\.com)/embed/[A-Za-z0-9_\-]+(\?[A-Za-z0-9_=&\-]*)?$`)).OnElements("iframe")
	p.AllowAttrs("width", "height", "frameborder", "allow", "allowfullscreen", "title", "loading").OnElements("iframe")
	p.AllowAttrs("loading", "width", "height", "alt", "title").OnElements("img")
	p.AllowAttrs("open").OnElements("details")
	p.RequireNoFollowOnLinks(false)
	p.RequireNoReferrerOnLinks(false)
	p.AllowRelativeURLs(true)
	return p
}()

// HTML renders markdown to sanitised HTML.
func HTML(markdown string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "<p>render error</p>"
	}
	out := policy.SanitizeBytes(buf.Bytes())
	// Lazy-load every image; the sanitiser would strip a loading attr the
	// author did not write, so add it after.
	out = bytes.ReplaceAll(out, []byte("<img "), []byte(`<img loading="lazy" `))
	return string(out)
}

var tagRe = regexp.MustCompile(`<[^>]*>`)
var spaceRe = regexp.MustCompile(`\s+`)

// Text strips tags from rendered HTML, for excerpts and search snippets.
func Text(htmlStr string) string {
	s := tagRe.ReplaceAllString(htmlStr, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&#34;", `"`).Replace(s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// Excerpt cuts plain text to about n characters on a word boundary.
func Excerpt(text string, n int) string {
	text = strings.TrimSpace(text)
	if len(text) <= n {
		return text
	}
	cut := text[:n]
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

// ReadingMinutes estimates reading time at ~220 words a minute, minimum 1.
func ReadingMinutes(text string) int {
	words := len(strings.Fields(text))
	m := (words + 219) / 220
	if m < 1 {
		m = 1
	}
	return m
}
