package htmlterm

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// SizeAutomatic is the zero value for Options.Width/Options.Height: track the
// terminal's current size for that dimension. Plain Renderer.Render/Document
// have no terminal to query, so it's inert there (behaves like SizeNatural
// for Height; Width has no natural fallback and is simply left at 0, same as
// before this constant existed). Loop is what actually resolves it — see
// Loop.Run and Document.SetSize — querying the terminal once at startup and
// again on every SIGWINCH, keeping whichever of Width/Height is
// SizeAutomatic live for the life of the Loop. It is the zero value
// deliberately, matching the rest of Options: a caller who never mentions
// Width/Height and drives rendering through Loop gets automatic sizing
// without writing anything.
const SizeAutomatic = 0

// SizeNatural, valid for Options.Height only, means "don't constrain height
// at all" — the document renders at whatever line count its content
// produces, with no clipping or padding (today's behavior, before Height
// existed). There is no equivalent for Width: wrapping always needs a
// concrete column count, so a "natural width" isn't a meaningful concept
// here the way it is for height.
const SizeNatural = -1

// Options configures a Renderer.
type Options struct {
	CSS               string               // additional stylesheet layered above built-in UA defaults
	Width             int                  // terminal column count; affects wrapping, tables, percentage widths
	Height            int                  // content-box line count the whole document is clipped/padded to; zero value is SizeAutomatic (see there); SizeNatural (-1) leaves it unconstrained
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
	height              int
	profile             colorprofile.Profile
	ignoreDocumentCSS   bool
	noOSC8Links         bool
	maxBlankLines       int
	stripHiddenInline   bool
	counterMap          map[*html.Node]counterSnapshot // built fresh per Render call
	quoteDepth          int                            // tracks open-quote nesting depth
	nestedTableWidth    int                            // width hint for a table nested inside a cell currently being sized
	nestedTableWidthSet bool                           // whether nestedTableWidth is active

	// scrollOffsets holds the previous frame's per-element vertical scroll
	// offsets (Document.scrollOffsets, read-only from this Renderer's
	// perspective) — nil for a plain Renderer.Render call, which has no
	// persistent Document to remember offsets across calls. liveScrollOffsets
	// accumulates this frame's offsets (clamped, one entry per element that
	// actually took the scroll/auto+resolved-height branch in
	// renderBlockContentBox) — rebuilt fresh every render rather than mutating
	// scrollOffsets in place, so an element that stops being a scroll
	// container doesn't leave a stale entry behind. See SCROLLING.md.
	scrollOffsets      map[*html.Node]int
	liveScrollOffsets  map[*html.Node]int
	liveScrollViewport map[*html.Node]scrollViewport
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
		height:            opts.Height,
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
	out, _, _, _ := r.renderTree(doc)
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
// error return to thread through. The third and fourth return values are
// this frame's freshly built scroll-offset and scroll-viewport maps (see the
// Renderer.scrollOffsets/liveScrollOffsets/liveScrollViewport doc comment) —
// Document.Render installs them as the new Document.scrollOffsets/
// scrollViewport; Render discards both, same as the position map.
func (r *Renderer) renderTree(doc *html.Node) (string, map[*html.Node]Rect, map[*html.Node]int, map[*html.Node]scrollViewport) {
	rr := &Renderer{
		rules:             r.rules,
		width:             r.width,
		height:            r.height,
		profile:           r.profile,
		ignoreDocumentCSS: r.ignoreDocumentCSS,
		noOSC8Links:       r.noOSC8Links,
		maxBlankLines:     r.maxBlankLines,
		stripHiddenInline: r.stripHiddenInline,
		scrollOffsets:     r.scrollOffsets,
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
	// rr.height <= 0 covers both SizeNatural (-1, explicitly unconstrained)
	// and an unresolved SizeAutomatic (0) — outside of Loop there's no
	// terminal to resolve automatic sizing against, so it's inert here,
	// same as natural. Unlike a per-element "height" (block.go), the root
	// has no paired "overflow" declaration to gate clipping on: a fixed root
	// height is a viewport constraint from the host, not CSS, so it always
	// both pads short content and truncates tall content — what a
	// non-scrolling terminal viewport needs (see forceHeight, box.go).
	if rr.height > 0 {
		lines = forceHeight(lines, rr.height)
	}
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	// A real &nbsp; HTML entity survives rendering as a distinct character
	// (normalizeWhiteSpace/plainInlineText only touch plain ASCII space);
	// normalize it to a plain space in the final string, since terminals
	// don't distinguish breaking from non-breaking spaces.
	return strings.ReplaceAll(out, nbsp, " "), positions, rr.liveScrollOffsets, rr.liveScrollViewport
}
