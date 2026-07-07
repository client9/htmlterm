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
	rules               []rule
	width               int
	profile             colorprofile.Profile
	ignoreDocumentCSS   bool
	noOSC8Links         bool
	maxBlankLines       int
	stripHiddenInline   bool
	counterMap          map[*html.Node]counterSnapshot // built fresh per Render call
	quoteDepth          int                            // tracks open-quote nesting depth
	nestedTableWidth    int                            // width hint for a table nested inside a cell currently being sized
	nestedTableWidthSet bool                           // whether nestedTableWidth is active
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
form                    { display: block; }
fieldset                { display: block; border-style: normal; padding: 1; margin-bottom: 1; }
legend                  { display: block; font-weight: bold; }
input, button           { display: inline-block; }
textarea                { display: block; border-style: normal; padding-left: 1; padding-right: 1; }
button::before          { content: "[ "; }
button::after           { content: " ]"; }
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
	out, _ := r.renderTree(doc)
	return out, nil
}

// renderTree renders an already-parsed document node, building fresh
// per-document scratch state (resolved CSS rules, counters) from r's
// configuration, the same way Render does after parsing — so it can be
// reused against a tree that didn't come from a fresh html.Parse call (see
// Document.Render). It also returns the fully resolved (absolute,
// document-coordinate) position map — the "propagated incrementally, one
// level at a time" mechanism from RENDERING.md's Position tracking section,
// resolved once the walk reaches this, the document root. Document.Render
// uses this to power Document.Rect; Render just discards it, keeping its
// existing contract exactly. Nothing here can actually fail — parsing (the
// only failure mode) already happened before this is called — so there's no
// error return to thread through.
func (r *Renderer) renderTree(doc *html.Node) (string, map[*html.Node]Rect) {
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
	tokens := rr.renderRootTokens(doc)
	// A trailing brk means the document's last content ended with a
	// structural writeNewline/margin call — the root's own terminating "\n"
	// that box.join()'s "no trailing newline" convention doesn't otherwise
	// produce (that convention exists for boxes embedded into a parent, not
	// the document itself). Bare inline root content (e.g. "<span>hi</span>"
	// with nothing else) ends in no brk and gets no trailing newline,
	// matching that this has always rendered as "hi", not "hi\n".
	trailingNewline := len(tokens) > 0 && tokens[len(tokens)-1].brk
	b, positions := wordWrapTokens(tokens, rr.width, "", 0)
	lines, rowRemap := capBlankRuns(b.lines, b.pre, rr.maxBlankLines)
	if rr.maxBlankLines > 0 && len(positions) > 0 {
		remapped := make(map[*html.Node]Rect, len(positions))
		for n, rect := range positions {
			if rect.Row >= 0 && rect.Row < len(rowRemap) {
				rect.Row = rowRemap[rect.Row]
			}
			remapped[n] = rect
		}
		positions = remapped
	}
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	// A real &nbsp; HTML entity survives rendering as a distinct character
	// (normalizeWhiteSpace/plainInlineText only touch plain ASCII space);
	// normalize it to a plain space in the final string, since terminals
	// don't distinguish breaking from non-breaking spaces.
	return strings.ReplaceAll(out, nbsp, " "), positions
}
