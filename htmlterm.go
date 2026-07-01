// Package htmlterm renders a restricted HTML+CSS subset to terminal strings.
// Supported selectors: element, .class, element.class, and space-separated
// descendant chains. See CSS.md for supported properties.
package htmlterm

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// Renderer renders HTML+CSS to terminal strings.
type Renderer struct {
	rules      []rule
	width      int
	profile    colorprofile.Profile
	counterMap map[*html.Node]counterSnapshot // built fresh per Render call
	quoteDepth int                            // tracks open-quote nesting depth
}

// uaCSS is the built-in default stylesheet (lowest priority — user CSS overrides it).
const uaCSS = `
p, blockquote, pre, h1, h2, h3, h4, h5, h6, div, section, article, header, footer, main, nav, aside { display: block; }
dl, dt, dd, figure, figcaption  { display: block; }
address, details, summary, caption, noscript { display: block; }
address  { font-style: italic; }
summary  { font-weight: bold; }
caption  { text-align: center; }
p                       { margin-bottom: 1; }
h1, h2, h3, h4, h5, h6 { font-weight: bold; }
th                      { font-weight: bold; }
dt                      { font-weight: bold; }
strong, b               { font-weight: bold; }
em, i, dfn              { font-style: italic; }
samp, var, cite, figcaption { font-style: italic; }
a                       { text-decoration: underline; }
u, ins                  { text-decoration: underline; }
pre                     { white-space: pre; }
ul, ol, menu            { padding-left: 4; }
dd                      { padding-left: 4; }
dl                      { margin-bottom: 1; }
td, th                  { white-space: nowrap; text-overflow: ellipsis; }
blockquote              { border-left: "│"; border-left-color: #555555; padding-left: 1; padding-right: 2; }
s, del                  { text-decoration: line-through; }
kbd                     { font-weight: bold; }
mark                    { background-color: #cc9900; color: #000000; }
small                   { color: #888888; }
sup                     { text-transform: superscript; }
sub                     { text-transform: subscript; }
q::before               { content: open-quote; }
q::after                { content: close-quote; }
img[alt]::before        { content: "[" attr(alt) "]"; }
`

// New parses css and returns a Renderer. width is the terminal column count.
func New(css string, width int) (*Renderer, error) {
	rules, err := parseCSS(uaCSS + css)
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	return &Renderer{rules: rules, width: width, profile: colorprofile.Detect(os.Stdout, os.Environ())}, nil
}

// Render parses htmlStr and returns a styled terminal string.
func (r *Renderer) Render(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("htmlterm: %w", err)
	}
	rr := r
	if extra := extractStyleRules(doc); len(extra) > 0 {
		combined := make([]rule, len(r.rules)+len(extra))
		copy(combined, r.rules)
		copy(combined[len(r.rules):], extra)
		rr = &Renderer{rules: combined, width: r.width, profile: r.profile}
	}
	rr.counterMap = rr.buildCounterMap(doc)
	rr.quoteDepth = 0
	var sb strings.Builder
	rr.renderNode(&sb, doc)
	return sb.String(), nil
}
