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
	CSS                 string
	Stylesheets         []string
	Width               int
	Height              int
	IgnoreDocumentCSS   bool
	Profile             colorprofile.Profile
	NoOSC8Links         bool
	MaxBlankLines       int
	StripHiddenInline   bool
	FocusAttr           string
	SelectOpenAttr      string
	SelectHighlightAttr string
	SelectionStartAttr  string
	SelectionEndAttr    string
}

// Engine renders already-parsed HTML trees or HTML strings to terminal output.
type Engine struct {
	baseRules []cssengine.Rule
	// uaRules is just the DefaultStylesheet's own parsed rules, the subset
	// of baseRules that came from the user-agent stylesheet rather than
	// Options.CSS or Options.Stylesheets. It's constant for the lifetime of
	// an Engine, unlike rules, since it never depends on the document being
	// rendered, and exists only to answer "revert" (see
	// cssengine.Cascade.UARules's doc comment).
	uaRules             []cssengine.Rule
	rules               []cssengine.Rule
	width               int
	height              int
	profile             colorprofile.Profile
	ignoreDocumentCSS   bool
	noOSC8Links         bool
	maxBlankLines       int
	stripHiddenInline   bool
	focusAttr           string
	selectOpenAttr      string
	selectHighlightAttr string
	selectionStartAttr  string
	selectionEndAttr    string
	counterMap          map[*html.Node]counterSnapshot
	directCache         map[*html.Node]map[string]string
	// minContentCache memoizes measureMinContentWidth (flex.go) per node. A
	// node's min-content width doesn't depend on the width it's offered, so it
	// is a pure function of the node within one render. It must be cached,
	// because the measurement is itself a full trial render that recurses
	// through nested flex containers. Without this, a flex container's own
	// min-content probe re-measures every descendant that already probed its
	// own children, costing ~2^depth for nested `flex: 1` layouts, measured at
	// 8x on nine levels. Per-render lifetime, same as directCache.
	minContentCache map[*html.Node]int
	// anonFlexNodes memoizes the synthetic element node standing in for one
	// run of loose text inside a flex container (flex.go's
	// anonymousFlexItem), keyed by the run's first text node. Items are
	// re-collected on every pass over a container, including every trial
	// measurement, and these nodes key the two caches above, so they have to
	// be the same pointer each time. Per-render lifetime, same as directCache.
	anonFlexNodes map[*html.Node]*html.Node
	// cbHeight is the containing block's definite content height in rows, the
	// basis a percentage height/min-height/max-height resolves against, or
	// indefiniteMainSize (flex.go) when there is nothing to resolve against.
	//
	// It is threaded as engine state rather than as a parameter, the same way
	// quoteDepth and measuringNaturalWidth are, because a containing block
	// height has to reach every box-producing call the way availWidth does and
	// there are thirteen renderInlineAccTokens call sites alone. Callers that
	// establish a new containing block save it, set it, and restore it, which
	// is the quoteDepth idiom. Rendering is single-goroutine (see
	// ARCHITECTURE.md's "No locking in the interactive layer"), so this is
	// safe.
	//
	// Only an explicit height establishes a definite basis. A min-height does
	// not: the real used height is then max(content, min-height), which isn't
	// known until the content is laid out, and CSS resolves a percentage
	// against such an indefinite basis to auto. That is the same rule
	// layoutFlexColumn's pctBasis already follows, for the same reason.
	//
	// The initial containing block is the viewport, so RenderNode seeds this
	// from the requested document height. With no requested height there is no
	// viewport to speak of and the root's basis is indefinite, which is why
	// `height: 100%` resolves under Options.Height and is inert without it.
	cbHeight              int
	quoteDepth            int
	nestedTableWidth      int
	nestedTableWidthSet   bool
	measuringNaturalWidth bool
	// shrinkToFit makes renderBlockContentBox/renderFlexContentBox size
	// themselves to their own content rather than filling the available
	// width they were handed, for the duration of a discardable measurement
	// render. A block box normally fills its containing block, real CSS
	// width:auto behavior, and this engine materializes that fill as actual
	// padding whenever anything needs a rectangle, whether a border,
	// text-align, or a closed box. So "render it at a generous width and read
	// the result's width back" measures the generous width, not the content.
	// flex.go's measureNaturalWidth needs the content answer, an item's flex
	// base size under flex-basis:auto or width:auto, and needs it recursively:
	// a bordered *child* of the item being measured has to shrink to its own
	// content too, or the item measures as wide as its widest filling
	// descendant.
	// Only ever set around a trial render whose output is thrown away.
	shrinkToFit bool
	// outOfFlow is the set of elements with position: absolute/fixed,
	// collected once up front by collectOutOfFlow (outofflow.go) before
	// normal layout runs. See outofflow.go's doc comment for why this is a
	// static pre-pass rather than incremental discovery. Consulted at every
	// layout call site that also checks display:none (render.go, inline.go,
	// flex.go), so an out-of-flow element reserves no space in normal flow,
	// then positioned and painted by applyOutOfFlow after layout finishes.
	outOfFlow map[*html.Node]bool
	// outOfFlowOrder is outOfFlow's membership in preorder, so an ancestor
	// always precedes its descendants. applyOutOfFlow (outofflow.go)
	// relies on that ordering directly for both its containing-block
	// dependency resolution and its z-index paint-order tiebreak.
	outOfFlowOrder []*html.Node

	scrollOffsets      map[*html.Node]int
	liveScrollOffsets  map[*html.Node]int
	liveScrollViewport map[*html.Node]Viewport
	liveContentOffsets map[*html.Node]int
	// liveContentOffsetsX is liveContentOffsets' horizontal counterpart: the
	// column shift from a block's own border box to its first content
	// column (padding-left + border-left + margin-left), needed wherever a
	// caret/cursor column has to be placed inside a bordered or padded box
	// (tui's focusCursorPos, Document's caretIndexFromClick).
	liveContentOffsetsX map[*html.Node]int

	scrollOffsetsX      map[*html.Node]int
	liveScrollOffsetsX  map[*html.Node]int
	liveScrollViewportX map[*html.Node]ViewportX
}

// Viewport records a scroll container's visible content-area geometry.
// GutterCol, GutterWidth, CapStart, and CapEnd are only meaningful when
// GutterWidth > 0, meaning an overflow-y:scroll gutter really was reserved
// this frame (see block.go's hasScrollbarGutter). document.go's
// tryScrollCapClick uses them, combined with the element's own Rect, whose
// Col and Row this Viewport's offsets are relative to, same as TopOffset
// already is, to hit-test a click against the cap buttons' cells.
//
// CapStart and CapEnd reflect whether that cap was drawn this frame, not just
// whether a ::scrollbar-cap-start or ::scrollbar-cap-end rule set content: a
// cap dropped for lack of room (see appendScrollbarColumn) must not be
// clickable.
type Viewport struct {
	Height      int
	TopOffset   int
	GutterCol   int
	GutterWidth int
	CapStart    bool
	CapEnd      bool
}

// ViewportX is Viewport's horizontal counterpart, recorded for an
// overflow-x:scroll|auto element with an explicit width. See block.go's
// hasScrollbarGutterX and docs/SCROLLING.md's horizontal-scrolling addendum.
//
// Width and LeftOffset are the transpose of Height and TopOffset: LeftOffset
// is the column shift from the element's own Rect.Col to where its content
// starts, the same relationship colShift already has to Rect.Col elsewhere.
// GutterRow, GutterHeight, CapStart, and CapEnd are the transpose of
// GutterCol, GutterWidth, CapStart, and CapEnd. GutterRow is the row shift
// from Rect.Row to the gutter row, only meaningful when GutterHeight > 0,
// meaning a visible ::scrollbar-track-x or ::scrollbar-thumb-x row was drawn
// this frame. CapStart and CapEnd here mean the *left* and *right* cap,
// ::scrollbar-cap-start-x and ::scrollbar-cap-end-x, not top and bottom.
type ViewportX struct {
	Width        int
	LeftOffset   int
	GutterRow    int
	GutterHeight int
	CapStart     bool
	CapEnd       bool
}

// Request supplies per-frame state for RenderNode.
type Request struct {
	Width          int
	Height         int
	Rules          []cssengine.Rule
	ScrollOffsets  map[*html.Node]int
	ScrollOffsetsX map[*html.Node]int
}

// Result is the rendered output plus layout metadata needed by interactive
// document hosts.
type Result struct {
	Output          string
	Positions       map[*html.Node]Rect
	ScrollOffsets   map[*html.Node]int
	ScrollViewport  map[*html.Node]Viewport
	ScrollOffsetsX  map[*html.Node]int
	ScrollViewportX map[*html.Node]ViewportX
	ContentOffsets  map[*html.Node]int
	ContentOffsetsX map[*html.Node]int
}

// New parses opts.CSS/opts.Stylesheets and returns a reusable render engine.
//
// Cascade order, lowest priority first: the built-in default stylesheet
// (DefaultStylesheet), opts.CSS, then each of opts.Stylesheets in order,
// mirroring how a page's own stylesheet is followed by however many <link>
// sheets it loads. Document <style> elements and inline style= attributes
// are layered on top of all of these at render time (see DocumentRules and
// cssengine.Cascade). DefaultStylesheet's own parsed rules are also kept
// separately, as uaRules, purely to give the "revert" cascade-wide keyword a
// user-agent-only rule set to fall back to (see cssengine.Cascade.UARules).
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
	uaRules, err := cssengine.ParseStylesheet(DefaultStylesheet)
	if err != nil {
		return nil, fmt.Errorf("htmlterm: default stylesheet: %w", err)
	}
	rules = append(rules, uaRules...)
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
	selectOpenAttr := opts.SelectOpenAttr
	if selectOpenAttr == "" {
		selectOpenAttr = defaultSelectOpenAttr
	}
	selectHighlightAttr := opts.SelectHighlightAttr
	if selectHighlightAttr == "" {
		selectHighlightAttr = defaultSelectHighlightAttr
	}
	selectionStartAttr := opts.SelectionStartAttr
	if selectionStartAttr == "" {
		selectionStartAttr = defaultSelectionStartAttr
	}
	selectionEndAttr := opts.SelectionEndAttr
	if selectionEndAttr == "" {
		selectionEndAttr = defaultSelectionEndAttr
	}
	return &Engine{
		baseRules:           rules,
		uaRules:             uaRules,
		width:               opts.Width,
		height:              opts.Height,
		profile:             profile,
		ignoreDocumentCSS:   opts.IgnoreDocumentCSS,
		noOSC8Links:         opts.NoOSC8Links,
		maxBlankLines:       opts.MaxBlankLines,
		stripHiddenInline:   opts.StripHiddenInline,
		focusAttr:           focusAttr,
		selectOpenAttr:      selectOpenAttr,
		selectHighlightAttr: selectHighlightAttr,
		selectionStartAttr:  selectionStartAttr,
		selectionEndAttr:    selectionEndAttr,
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
		baseRules:           e.baseRules,
		uaRules:             e.uaRules,
		rules:               rules,
		width:               req.Width,
		height:              req.Height,
		profile:             e.profile,
		ignoreDocumentCSS:   e.ignoreDocumentCSS,
		noOSC8Links:         e.noOSC8Links,
		maxBlankLines:       e.maxBlankLines,
		stripHiddenInline:   e.stripHiddenInline,
		focusAttr:           e.focusAttr,
		selectOpenAttr:      e.selectOpenAttr,
		selectHighlightAttr: e.selectHighlightAttr,
		selectionStartAttr:  e.selectionStartAttr,
		selectionEndAttr:    e.selectionEndAttr,
		scrollOffsets:       req.ScrollOffsets,
		scrollOffsetsX:      req.ScrollOffsetsX,
	}
	if rr.width == 0 {
		rr.width = e.width
	}
	if rr.height == 0 {
		rr.height = e.height
	}
	rr.counterMap = rr.buildCounterMap(doc)
	rr.directCache = make(map[*html.Node]map[string]string)
	rr.minContentCache = make(map[*html.Node]int)
	rr.anonFlexNodes = make(map[*html.Node]*html.Node)
	rr.quoteDepth = 0
	// The initial containing block's height is the viewport's. rr.height is
	// that viewport in rows, a content-box count, which is exactly what a
	// percentage height resolves against. SizeNatural (-1) and SizeAutomatic
	// (0) both leave it indefinite, and indefiniteMainSize is itself -1, so
	// SizeNatural already carries the right meaning; the explicit branch keeps
	// that from being a coincidence two constants have to preserve.
	rr.cbHeight = indefiniteMainSize
	if rr.height > 0 {
		rr.cbHeight = rr.height
	}
	rr.outOfFlow, rr.outOfFlowOrder = rr.collectOutOfFlow(doc)
	tokens := rr.renderRootTokens(doc)
	trailingNewline := len(tokens) > 0 && tokens[len(tokens)-1].brk
	// The document root itself is never in a preserving white-space mode: a
	// <pre> reaches here as an already-rendered box token.
	b, positions := wordWrapTokens(tokens, rr.width, "", 0, false)
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
	lines, positions = rr.applyRelativeOffsets(doc, lines, positions, rr.width)
	lines, positions = rr.applyOutOfFlow(lines, positions, rr.height <= 0)
	lines, positions = rr.compositeOpenSelects(doc, lines, positions, rr.height <= 0)
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	return Result{
		Output:          strings.ReplaceAll(out, nbsp, " "),
		Positions:       positions,
		ScrollOffsets:   rr.liveScrollOffsets,
		ScrollViewport:  rr.liveScrollViewport,
		ScrollOffsetsX:  rr.liveScrollOffsetsX,
		ScrollViewportX: rr.liveScrollViewportX,
		ContentOffsets:  rr.liveContentOffsets,
		ContentOffsetsX: rr.liveContentOffsetsX,
	}
}
