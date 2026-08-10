package render

import (
	"sort"
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// collectOutOfFlow walks doc once, before normal layout runs, finding every
// element whose resolved position is absolute or fixed. Static and upfront
// rather than discovered incrementally during layout (the way
// compositeOpenSelects's own walk discovers open <select>s) because an
// out-of-flow element can itself contain further out-of-flow descendants.
// One whole-document walk finds all of them regardless of nesting depth,
// with no fixed-point re-walk needed. Preorder, so the returned order slice
// always visits an ancestor before its descendants: applyOutOfFlow's Phase A
// relies on that directly (a containing-block ancestor that is itself
// out-of-flow is always resolved before any out-of-flow descendant that
// depends on it), and it doubles as a stable document-order tiebreak for
// Phase B's z-index sort. The returned map is consulted at every layout call
// site that also checks display:none (render.go, inline.go, flex.go), so an
// out-of-flow element reserves no space in normal flow. See
// docs/RENDERING.md's "position: absolute / position: fixed" section.
func (e *Engine) collectOutOfFlow(doc *html.Node) (map[*html.Node]bool, []*html.Node) {
	nodes := map[*html.Node]bool{}
	var order []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch e.resolveDecls(n)["position"] {
			case "absolute", "fixed":
				nodes[n] = true
				order = append(order, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return nodes, order
}

// nearestPositionedAncestor walks n's ancestor chain for the nearest element
// with position: relative, absolute, or fixed, the real-CSS definition of an
// absolutely positioned element's containing block. Returns nil if none
// exists, in which case the caller falls back to the whole document, the
// initial containing block, matching spec.
func (e *Engine) nearestPositionedAncestor(n *html.Node) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type != html.ElementNode {
			continue
		}
		switch e.resolveDecls(p)["position"] {
		case "relative", "absolute", "fixed":
			return p
		}
	}
	return nil
}

// axis names one of the two axes an out-of-flow element is positioned on,
// bundling the four properties that decide its placement there so
// resolveAbsoluteAxis and centersOnAxis can't disagree about which
// properties belong together.
type axis struct {
	start, end             string // top/bottom, or left/right
	startMargin, endMargin string // margin-top/margin-bottom, or margin-left/margin-right
}

var (
	blockAxis  = axis{start: "top", end: "bottom", startMargin: "margin-top", endMargin: "margin-bottom"}
	inlineAxis = axis{start: "left", end: "right", startMargin: "margin-left", endMargin: "margin-right"}
)

// centersOnAxis reports whether an out-of-flow element should be centered in
// its containing block on ax: both margins on that axis are the literal
// "auto" and neither offset is set. That is CSS 2.1 §10.3.7's own rule for
// the inline axis. Real CSS centers on the block axis only when both top and
// bottom are also set, which this doesn't require, so a `margin: auto` box
// with no offsets at all centers on both axes here rather than only
// horizontally. The stricter reading would leave `dialog:modal` needing an
// explicit `top: 0; bottom: 0` pair to sit in the middle of the screen, which
// is worse than the deviation. See COMPATIBILITY.md.
func centersOnAxis(decls map[string]string, ax axis) bool {
	if decls[ax.start] != "" || decls[ax.end] != "" {
		return false
	}
	_, startAuto := resolveMarginSide(decls[ax.startMargin], 0)
	_, endAuto := resolveMarginSide(decls[ax.endMargin], 0)
	return startAuto && endAuto
}

// resolveAbsoluteAxis resolves one axis of an out-of-flow element's offset
// against its containing block, in cells. cbSize is the containing block's
// own size on this axis, used for percentage resolution and for the
// end-anchored and centered cases below, and boxSize is the element's own
// already-rendered size on this axis.
//
// top and left win over bottom and right when both are set to a non-auto
// value, matching real CSS. Two auto margins with neither offset set centers
// the box (see centersOnAxis). With none of the four set the result defaults
// to 0, the containing block's own leading edge. That is a documented
// simplification of spec's "static position" algorithm, which would otherwise
// place the element roughly where it would have flowed.
func resolveAbsoluteAxis(ax axis, decls map[string]string, boxSize, cbSize int) int {
	switch {
	case decls[ax.start] != "":
		v, _ := resolveMarginSide(decls[ax.start], cbSize)
		return v
	case decls[ax.end] != "":
		v, _ := resolveMarginSide(decls[ax.end], cbSize)
		return cbSize - boxSize - v
	case centersOnAxis(decls, ax):
		return max(0, (cbSize-boxSize)/2)
	default:
		return 0
	}
}

// parseZIndex parses the z-index property, mirroring flex.go's parseOrder
// exactly: default 0, invalid or missing values also parse as 0. A calc()/
// min()/max()/clamp() value resolves through resolveCSSMathInteger first,
// rounded to the nearest integer per CSS's own rule (see that function's
// doc comment); a plain literal keeps going through strconv.Atoi exactly
// as before, so "z-index: 1.5", not valid CSS <integer> syntax, still
// fails rather than being newly accepted as 1 or 2.
func parseZIndex(decls map[string]string) int {
	s := strings.TrimSpace(decls["z-index"])
	if n, ok := resolveCSSMathInteger(s); ok {
		return n
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// renderOutOfFlowBox renders n's own box for applyOutOfFlow's Phase A,
// sized the way real CSS sizes an out-of-flow box rather than always filling
// its containing block.
//
// With no declared width, and not both offsets on the inline axis set, the
// box shrinks to fit its own content. CSS calls this shrink-to-fit, and this
// engine already has the measurement: flex.go's measureNaturalWidth, which
// renders once under Engine.shrinkToFit purely to read the content width
// back. Its flexItem parameter is just a node-plus-decls carrier here, since
// grow, shrink, and order go unread by it. The measured width then becomes
// the available width for the real render below, so the box fills exactly
// its own content and inner blocks still fill that, rather than each
// shrinking independently the way a single recursive shrink-to-fit render
// would leave them.
//
// Both offsets set is the exception, and keeps the fill behavior: CSS
// stretches such a box between its two edges, and stretching to the
// containing block is the closer answer of the two available here (see
// resolveAbsoluteAxis, which reads only the start offset in that case).
//
// When this element is centering itself on the inline axis, its auto
// margins are dropped before rendering. renderBlockContentBox splits auto
// margins itself for an explicit-width box and bakes the result into the
// box's own lines as leading spaces, which would center it a second time on
// top of resolveAbsoluteAxis's own offset. Dropping them here makes
// outofflow.go the single owner of out-of-flow centering, and has the side
// benefit that the element's recorded Rect is the box itself rather than a
// containing-block-wide band with the box padded inside it, so a click
// hit-tests against what's actually painted.
func (e *Engine) renderOutOfFlowBox(n *html.Node, decls map[string]string, cb Rect) (box, map[*html.Node]Rect) {
	renderDecls := decls
	if centersOnAxis(decls, inlineAxis) {
		renderDecls = make(map[string]string, len(decls))
		for k, v := range decls {
			renderDecls[k] = v
		}
		delete(renderDecls, inlineAxis.startMargin)
		delete(renderDecls, inlineAxis.endMargin)
	}

	availWidth := cb.Width
	bothOffsets := decls[inlineAxis.start] != "" && decls[inlineAxis.end] != ""
	if decls["width"] == "" && !bothOffsets {
		if w := e.measureNaturalWidth(flexItem{node: n, decls: renderDecls}, cb.Width); w > 0 && w < availWidth {
			availWidth = w
		}
	}
	return e.renderBlockContentBox(n, renderDecls, availWidth)
}

// resolvedOutOfFlow is one out-of-flow element's computed geometry from
// applyOutOfFlow's Phase A, carried into Phase B for z-index-ordered
// painting. rect is already clamped and clipped to its final painted bounds
// by clampOutOfFlowRect, rather than being a raw offset computation, so that
// a descendant using this element as its own containing block, via
// resolvedRects in applyOutOfFlow, resolves against where this element ends
// up painted rather than a pre-clamp position it may never occupy.
type resolvedOutOfFlow struct {
	node         *html.Node
	rect         Rect
	box          box
	subPositions map[*html.Node]Rect
	zIndex       int
	// modalDepth is a modal <dialog>'s position in the top layer, taken from
	// the value of its marker attribute, or 0 for everything else. Every
	// modal gets the same z-index from the UA stylesheet, so without this
	// they would paint in document order and a dialog opened second could be
	// painted over, or have its ::backdrop wiped, by one that sits earlier in
	// the source. Focus, click containment, and Escape all follow ShowModal
	// call order (see document's modalDepth), and paint order has to agree.
	modalDepth int
}

// modalPaintDepth reports n's top-layer position from its modal marker
// attribute, or 0 if it isn't a modal <dialog>. The value is written by
// document's ShowModal; this only orders by it. An unparseable value sorts as
// 0, the same forgiving default parseZIndex uses.
func (e *Engine) modalPaintDepth(n *html.Node) int {
	if e.dialogModalAttr == "" {
		return 0
	}
	depth, err := strconv.Atoi(nodeAttr(n, e.dialogModalAttr))
	if err != nil || depth < 0 {
		return 0
	}
	return depth
}

// clampOutOfFlowRect clamps rect to its final painted bounds: Col to
// [0, width-rect.Width], since columns are a hard terminal-width boundary and
// are never grown, the same convention compositeSelectPopup already uses; Row
// to >= 0, since a document can't grow upward; and, only when canGrow is
// false, meaning a fixed Options.Height viewport, Height clipped so
// Row+Height doesn't exceed docHeight, dropping trailing rows to match
// compositeSelectPopup's own clip-from-the-end convention. When canGrow is
// true, Height is left
// alone: the document grows to fit instead, handled later by
// applyOutOfFlow's line-growth loop, not here.
func clampOutOfFlowRect(rect Rect, width, docHeight int, canGrow bool) Rect {
	rect.Col = max(0, min(rect.Col, width-rect.Width))
	rect.Row = max(0, rect.Row)
	if !canGrow && rect.Row+rect.Height > docHeight {
		rect.Height = max(0, docHeight-rect.Row)
	}
	return rect
}

// applyOutOfFlow positions and paints every position: absolute/fixed
// element collected by collectOutOfFlow (e.outOfFlowOrder), in two phases.
//
// Phase A resolves each element's containing block, renders its own box via
// renderBlockContentBox, which is spec-correct rather than a simplification
// since real CSS blockifies an out-of-flow element regardless of its own
// display value; applies applyRelativeOffsets to that box's own local lines
// and subPositions, so a position: relative descendant nested inside this
// out-of-flow element's subtree still shifts, its whole subtree having been
// skipped during normal layout so Engine.RenderNode's own top-level
// applyRelativeOffsets call never saw it; and computes its final,
// already-clamped Rect via clampOutOfFlowRect. Elements are processed in
// e.outOfFlowOrder, preorder, ancestors before descendants, so an absolute
// element whose containing block is itself out-of-flow always finds that
// ancestor's clamped geometry already resolved in resolvedRects.
//
// Phase B paints the results in z-index order (ties broken by the same
// document order collectOutOfFlow already established, via
// sort.SliceStable) using the same textcell.SpliceColumns-based painter's-algorithm
// compositing applyRelativeOffsets and compositeOpenSelects already use. A
// later splice fully overwrites whatever an earlier one painted into the
// same cells, with no partial blending.
//
// Runs after applyRelativeOffsets (so a position: relative containing-block
// ancestor's shifted Rect is what out-of-flow descendants resolve against,
// matching real CSS) and before compositeOpenSelects (so an out-of-flow
// <select>, which has no Rect from normal layout, having been skipped there,
// gets one before the popup pass looks it up). canGrow mirrors
// compositeOpenSelects's own parameter: true lets a natural or
// automatic-height document grow to fit, and false, a fixed Options.Height,
// clips rows that don't fit instead.
func (e *Engine) applyOutOfFlow(lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	if len(e.outOfFlowOrder) == 0 {
		return lines, positions
	}

	docHeight := len(lines)
	root := Rect{Row: 0, Col: 0, Width: e.width, Height: docHeight}
	resolvedRects := make(map[*html.Node]Rect, len(e.outOfFlowOrder))
	results := make([]resolvedOutOfFlow, 0, len(e.outOfFlowOrder))

	for _, n := range e.outOfFlowOrder {
		decls := e.resolveDecls(n)
		cb := root
		if decls["position"] == "absolute" {
			if ancestor := e.nearestPositionedAncestor(n); ancestor != nil {
				if r, ok := resolvedRects[ancestor]; ok {
					cb = r
				} else if r, ok := positions[ancestor]; ok {
					cb = r
				}
			}
		}

		bx, subPositions := e.renderOutOfFlowBox(n, decls, cb)
		bx.lines, subPositions = e.applyRelativeOffsets(n, bx.lines, subPositions, bx.width)
		height := len(bx.lines)
		row := cb.Row + resolveAbsoluteAxis(blockAxis, decls, height, cb.Height)
		col := cb.Col + resolveAbsoluteAxis(inlineAxis, decls, bx.width, cb.Width)

		rect := clampOutOfFlowRect(Rect{Row: row, Col: col, Width: bx.width, Height: height}, e.width, docHeight, canGrow)
		resolvedRects[n] = rect
		results = append(results, resolvedOutOfFlow{
			node: n, rect: rect, box: bx, subPositions: subPositions,
			zIndex: parseZIndex(decls), modalDepth: e.modalPaintDepth(n),
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].zIndex != results[j].zIndex {
			return results[i].zIndex < results[j].zIndex
		}
		// Tie-break on top-layer depth before falling through to the stable
		// sort's own document-order tiebreak. See resolvedOutOfFlow.modalDepth.
		return results[i].modalDepth < results[j].modalDepth
	})

	for _, res := range results {
		// Grown before the backdrop paints, not after: paintOutOfFlow would
		// otherwise append the rows this element needs once the backdrop had
		// already covered every row that existed, leaving a modal's scrim
		// stopping short of the dialog it belongs to.
		lines = e.growLines(lines, res.rect.Row+res.rect.Height)
		lines = e.paintBackdrop(res.node, lines)
		lines, positions = e.paintOutOfFlow(res, lines, positions)
	}
	return lines, positions
}

// growLines appends blank full-width rows until lines is at least n long,
// the growth an out-of-flow element needs when it lands past the document's
// current end and canGrow allowed it to stay there.
func (e *Engine) growLines(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, strings.Repeat(" ", e.width))
	}
	return lines
}

// paintBackdrop fills the whole viewport with n's ::backdrop background
// immediately before n itself is painted, so the backdrop sits under its own
// element and over everything painted earlier. A no-op unless n is a modal
// <dialog> that has a ::backdrop rule setting a color, which is what makes
// the whole feature opt-in.
//
// Only a modal gets one, matching real CSS, where ::backdrop applies to
// top-layer elements alone rather than to any positioned box.
//
// The fill is opaque: it replaces the content behind it rather than tinting
// it. A browser's own default backdrop is semi-transparent, and there is no
// equivalent here, because nothing can re-tint an already-serialized line of
// ANSI (opacity applies to a color at style-resolution time, long before
// this). That is why there is no UA default rule for ::backdrop at all: a
// default scrim would destroy the page behind every modal, so an author who
// wants one asks for it. See docs/DIALOG.md.
func (e *Engine) paintBackdrop(n *html.Node, lines []string) []string {
	if e.dialogModalAttr == "" || !nodeHasAttr(n, e.dialogModalAttr) {
		return lines
	}
	decls := e.resolveDecls(n)
	style := extractInlineStyle(e.pseudoElemDecls(n, "backdrop", customPropSubset(decls)))
	if !style.has() {
		return lines
	}
	row := style.render(strings.Repeat(" ", e.width), e.profile)
	for i := range lines {
		lines[i] = textcell.SpliceColumns(lines[i], 0, e.width, row)
	}
	return lines
}

// paintOutOfFlow splices one resolvedOutOfFlow's already-clamped rect onto
// lines and records the final Rects for it and its descendants. See
// applyOutOfFlow's doc comment for the two-phase design this is Phase B's
// per-element step of. Growing lines to fit res.rect, when Phase A's
// canGrow left Height unclipped, is the only bounds-handling left to do
// here. Col, Row, and Height were already clamped and clipped in Phase A via
// clampOutOfFlowRect, precisely so a dependent descendant's own Phase A
// containing-block lookup sees the same bounds this paints at.
func (e *Engine) paintOutOfFlow(res resolvedOutOfFlow, lines []string, positions map[*html.Node]Rect) ([]string, map[*html.Node]Rect) {
	rect := res.rect
	if rect.Width <= 0 || rect.Height <= 0 {
		return lines, positions
	}
	lines = e.growLines(lines, rect.Row+rect.Height)
	for i := range rect.Height {
		row := rect.Row + i
		lines[row] = textcell.SpliceColumns(lines[row], rect.Col, rect.Width, res.box.lines[i])
	}
	positions = mergePositions(positions, res.subPositions, rect.Row, rect.Col)
	positions[res.node] = rect
	return lines, positions
}
