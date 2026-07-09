package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm/internal/cssengine"
	"golang.org/x/net/html"
)

// Options configures the internal render engine. The public htmlterm.Options
// type is converted to this type by the root package facade.
type Options struct {
	CSS               string
	Stylesheets       []string
	Width             int
	Height            int
	IgnoreDocumentCSS bool
	Profile           colorprofile.Profile
	NoOSC8Links       bool
	MaxBlankLines     int
	StripHiddenInline bool
	FocusAttr         string
}

// Engine renders already-parsed HTML trees or HTML strings to terminal output.
type Engine struct {
	baseRules           []cssengine.Rule
	rules               []cssengine.Rule
	width               int
	height              int
	profile             colorprofile.Profile
	ignoreDocumentCSS   bool
	noOSC8Links         bool
	maxBlankLines       int
	stripHiddenInline   bool
	focusAttr           string
	counterMap          map[*html.Node]counterSnapshot
	quoteDepth          int
	nestedTableWidth    int
	nestedTableWidthSet bool

	scrollOffsets      map[*html.Node]int
	liveScrollOffsets  map[*html.Node]int
	liveScrollViewport map[*html.Node]Viewport
	liveContentOffsets map[*html.Node]int
}

// Viewport records a scroll container's visible content-area geometry.
type Viewport struct {
	Height    int
	TopOffset int
}

// Request supplies per-frame state for RenderNode.
type Request struct {
	Width         int
	Height        int
	Rules         []cssengine.Rule
	ScrollOffsets map[*html.Node]int
}

// Result is the rendered output plus layout metadata needed by interactive
// document hosts.
type Result struct {
	Output         string
	Positions      map[*html.Node]Rect
	ScrollOffsets  map[*html.Node]int
	ScrollViewport map[*html.Node]Viewport
	ContentOffsets map[*html.Node]int
}

// New parses opts.CSS/opts.Stylesheets and returns a reusable render engine.
//
// Cascade order (lowest priority first): the built-in default stylesheet
// (DefaultStylesheet), opts.CSS, then each of opts.Stylesheets in order —
// mirroring how a page's own stylesheet is followed by however many <link>
// sheets it loads. Document <style> elements and inline style= attributes
// are layered on top of all of these at render time (see DocumentRules and
// cssengine.Cascade).
func New(opts Options) (*Engine, error) {
	var rules []cssengine.Rule
	addSheet := func(label, src string) error {
		parsed, err := cssengine.ParseStylesheet(src)
		if err != nil {
			return fmt.Errorf("htmlterm: %s: %w", label, err)
		}
		rules = append(rules, parsed...)
		return nil
	}
	if err := addSheet("default stylesheet", DefaultStylesheet); err != nil {
		return nil, err
	}
	if opts.CSS != "" {
		if err := addSheet("Options.CSS", opts.CSS); err != nil {
			return nil, err
		}
	}
	for i, sheet := range opts.Stylesheets {
		if err := addSheet(fmt.Sprintf("Options.Stylesheets[%d]", i), sheet); err != nil {
			return nil, err
		}
	}
	profile := opts.Profile
	if profile == 0 {
		profile = colorprofile.Detect(os.Stdout, os.Environ())
	}
	focusAttr := opts.FocusAttr
	if focusAttr == "" {
		focusAttr = defaultFocusAttr
	}
	return &Engine{
		baseRules:         rules,
		width:             opts.Width,
		height:            opts.Height,
		profile:           profile,
		ignoreDocumentCSS: opts.IgnoreDocumentCSS,
		noOSC8Links:       opts.NoOSC8Links,
		maxBlankLines:     opts.MaxBlankLines,
		stripHiddenInline: opts.StripHiddenInline,
		focusAttr:         focusAttr,
	}, nil
}

// RenderHTML parses htmlStr and renders it. It returns layout metadata for
// callers that need it; the public root facade discards everything except
// Output.
func (e *Engine) RenderHTML(htmlStr string) (Result, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return Result{}, fmt.Errorf("htmlterm: %w", err)
	}
	if e.stripHiddenInline {
		stripHiddenInline(doc)
	}
	return e.RenderNode(doc, Request{Width: e.width, Height: e.height, Rules: e.DocumentRules(doc)}), nil
}

func (e *Engine) Render(htmlStr string) (string, error) {
	result, err := e.RenderHTML(htmlStr)
	return result.Output, err
}

// DocumentRules returns the final rule set for doc: engine rules plus any
// active <style> rules, unless document CSS is ignored.
func (e *Engine) DocumentRules(doc *html.Node) []cssengine.Rule {
	if e.ignoreDocumentCSS {
		return e.baseRules
	}
	extra := cssengine.ExtractStyleRules(doc)
	if len(extra) == 0 {
		return e.baseRules
	}
	combined := make([]cssengine.Rule, len(e.baseRules)+len(extra))
	copy(combined, e.baseRules)
	copy(combined[len(e.baseRules):], extra)
	return combined
}

// RenderNode renders an already-parsed document using request-specific state.
func (e *Engine) RenderNode(doc *html.Node, req Request) Result {
	rules := req.Rules
	if rules == nil {
		rules = e.DocumentRules(doc)
	}
	rr := &Engine{
		baseRules:         e.baseRules,
		rules:             rules,
		width:             req.Width,
		height:            req.Height,
		profile:           e.profile,
		ignoreDocumentCSS: e.ignoreDocumentCSS,
		noOSC8Links:       e.noOSC8Links,
		maxBlankLines:     e.maxBlankLines,
		stripHiddenInline: e.stripHiddenInline,
		focusAttr:         e.focusAttr,
		scrollOffsets:     req.ScrollOffsets,
	}
	if rr.width == 0 {
		rr.width = e.width
	}
	if rr.height == 0 {
		rr.height = e.height
	}
	rr.counterMap = rr.buildCounterMap(doc)
	rr.quoteDepth = 0
	tokens := rr.renderRootTokens(doc)
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
	if rr.height > 0 {
		lines = forceHeight(lines, rr.height)
		if len(positions) > 0 {
			visible := make(map[*html.Node]Rect, len(positions))
			for n, rect := range positions {
				if rect.Row >= 0 && rect.Row < rr.height {
					visible[n] = rect
				}
			}
			positions = visible
		}
	}
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	return Result{
		Output:         strings.ReplaceAll(out, nbsp, " "),
		Positions:      positions,
		ScrollOffsets:  rr.liveScrollOffsets,
		ScrollViewport: rr.liveScrollViewport,
		ContentOffsets: rr.liveContentOffsets,
	}
}

// DefaultStylesheet is the built-in default stylesheet (lowest priority —
// Options.CSS and Options.Stylesheets are layered above it). Re-exported by
// the root package as htmlterm.DefaultStylesheet.
const DefaultStylesheet = `
table                   { display: table; }
[hidden], [aria-hidden=true] { display: none; }
p, blockquote, pre, h1, h2, h3, h4, h5, h6, div, section, article, header, footer, main, nav, aside, hgroup, search { display: block; }
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
fieldset                { display: block; border-style: solid; padding: 1; margin-bottom: 1; }
legend                  { display: block; font-weight: bold; }
input, button           { display: inline-block; }
textarea                { display: block; border-style: solid; padding-left: 1; padding-right: 1; }
button::before          { content: "[ "; }
button::after           { content: " ]"; }
`

const defaultFocusAttr = "data-htmlterm-focus"
