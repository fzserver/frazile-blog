package render

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// rawHTMLRenderer decides what happens to HTML an author types directly into
// markdown. Tags on the allow list (embeds and a few inline elements the
// sanitiser also permits) pass through for bluemonday to check attributes;
// everything else is written as escaped text, so a stray "<title>" or
// "<style>" in prose renders literally instead of being treated as markup.
// Without this, an unclosed skip-content element (title, style, script,
// textarea...) made bluemonday drop the whole remainder of the post.
type rawHTMLRenderer struct{}

var allowedRaw = map[string]bool{
	"video": true, "source": true, "iframe": true, "figure": true, "figcaption": true,
	"img": true, "details": true, "summary": true, "br": true, "kbd": true, "mark": true,
	"sub": true, "sup": true, "div": true, "span": true, "p": true, "a": true, "em": true,
	"strong": true, "code": true, "pre": true, "table": true, "thead": true, "tbody": true,
	"tr": true, "td": true, "th": true, "ul": true, "ol": true, "li": true, "blockquote": true,
	"hr": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "input": true, "del": true, "ins": true, "small": true, "cite": true, "abbr": true,
}

var tagRe2 = regexp.MustCompile(`<\s*/?\s*([a-zA-Z][a-zA-Z0-9-]*)`)

// allowedHTML reports whether every tag in the fragment is on the list.
// HTML comments are fine (bluemonday removes them).
func allowedHTML(b []byte) bool {
	s := string(b)
	if strings.HasPrefix(s, "<!--") {
		return true
	}
	for _, m := range tagRe2.FindAllStringSubmatch(s, -1) {
		if !allowedRaw[strings.ToLower(m[1])] {
			return false
		}
	}
	return true
}

func (r *rawHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
}

func (r *rawHTMLRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	var buf []byte
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		buf = append(buf, seg.Value(source)...)
	}
	if allowedHTML(buf) {
		w.Write(buf)
	} else {
		w.Write(util.EscapeHTML(buf))
	}
	return ast.WalkSkipChildren, nil
}

func (r *rawHTMLRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if !entering {
		if n.HasClosure() {
			line := n.ClosureLine.Value(source)
			if allowedHTML(line) {
				w.Write(line)
			} else {
				w.Write(util.EscapeHTML(line))
			}
		}
		return ast.WalkContinue, nil
	}
	var buf []byte
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		buf = append(buf, line.Value(source)...)
	}
	if allowedHTML(buf) {
		w.Write(buf)
	} else {
		w.Write(util.EscapeHTML(buf))
	}
	return ast.WalkContinue, nil
}
