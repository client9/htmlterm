package htmlterm

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// Options configures a Renderer.
type Options struct {
	CSS               string               // additional stylesheet layered above built-in UA defaults
	Width             int                  // terminal column count; affects wrapping, tables, percentage widths
	IgnoreDocumentCSS bool                 // if true, <style> elements and style= attributes in HTML are ignored
	Profile           colorprofile.Profile // color profile; zero value (NoTTY) auto-detects from environment
	NoOSC8Links       bool                 // if true, OSC 8 hyperlink sequences are not emitted for <a> elements
	MaxBlankLines     int                  // if > 0, collapses runs of blank lines to at most this many; <pre> content is not affected
	StripHiddenInline bool                 // if true, elements hidden via their own inline style= (display:none, visibility:hidden, opacity:0, zero height/max-height with overflow:hidden) are removed before rendering; independent of IgnoreDocumentCSS
}

// Renderer renders HTML+CSS to terminal strings.
//
// A Renderer can be reused for multiple Render calls, including concurrent
// calls. Per-document state is built fresh for each render.
type Renderer struct {
	rules             []rule
	width             int
	profile           colorprofile.Profile
	ignoreDocumentCSS bool
	noOSC8Links       bool
	maxBlankLines     int
	stripHiddenInline bool
	counterMap        map[*html.Node]counterSnapshot // built fresh per Render call
	quoteDepth        int                            // tracks open-quote nesting depth
}

// uaCSS is the built-in default stylesheet (lowest priority — user CSS overrides it).
const uaCSS = `
table                   { display: table; }
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
img::before             { content: attr(alt); }
abbr[title]::after      { content: " (" attr(title) ")"; }
hr                      { display: block; border-top: "─"; }
`

// New parses opts.CSS and returns a Renderer.
func New(opts Options) (*Renderer, error) {
	rules, err := parseCSS(uaCSS + opts.CSS)
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	profile := opts.Profile
	if profile == 0 {
		profile = colorprofile.Detect(os.Stdout, os.Environ())
	}
	return &Renderer{
		rules:             rules,
		width:             opts.Width,
		profile:           profile,
		ignoreDocumentCSS: opts.IgnoreDocumentCSS,
		noOSC8Links:       opts.NoOSC8Links,
		maxBlankLines:     opts.MaxBlankLines,
		stripHiddenInline: opts.StripHiddenInline,
	}, nil
}

// Render parses htmlStr and returns a styled terminal string.
func (r *Renderer) Render(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("htmlterm: %w", err)
	}
	if r.stripHiddenInline {
		stripHiddenInline(doc)
	}
	rr := &Renderer{
		rules:             r.rules,
		width:             r.width,
		profile:           r.profile,
		ignoreDocumentCSS: r.ignoreDocumentCSS,
		noOSC8Links:       r.noOSC8Links,
		maxBlankLines:     r.maxBlankLines,
		stripHiddenInline: r.stripHiddenInline,
	}
	if !r.ignoreDocumentCSS {
		if extra := extractStyleRules(doc); len(extra) > 0 {
			combined := make([]rule, len(r.rules)+len(extra))
			copy(combined, r.rules)
			copy(combined[len(r.rules):], extra)
			rr.rules = combined
		}
	}
	rr.counterMap = rr.buildCounterMap(doc)
	rr.quoteDepth = 0
	w := cappedWriter{maxBlanks: rr.maxBlankLines}
	rr.renderNode(&w, doc)
	return w.String(), nil
}
