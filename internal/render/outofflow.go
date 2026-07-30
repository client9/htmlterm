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
// out-of-flow element can itself contain further out-of-flow descendants —
// one whole-document walk finds all of them regardless of nesting depth,
// with no fixed-point re-walk needed. Preorder, so the returned order slice
// always visits an ancestor before its descendants: applyOutOfFlow's Phase A
// relies on that directly (a containing-block ancestor that is itself
// out-of-flow is always resolved before any out-of-flow descendant that
// depends on it), and it doubles as a stable document-order tiebreak for
// Phase B's z-index sort. The returned map is consulted at every layout call
// site that also checks display:none (render.go, inline.go, flex.go), so an
// out-of-flow element reserves no space in normal flow — see
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
// with position: relative/absolute/fixed — the real-CSS definition of an
// absolutely positioned element's containing block. Returns nil if none
// exists, in which case the caller falls back to the whole document (the
// initial containing block), matching spec.
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

// resolveAbsoluteAxis resolves one axis (top/bottom or left/right) of an
// out-of-flow element's offset against its containing block, in cells.
// cbSize is the containing block's own size on this axis (used for
// percentage resolution and for the end-anchored case below); boxSize is
// the element's own already-rendered size on this axis. top/left win over
// bottom/right when both are set (non-auto) — matching real CSS — and
// neither set defaults to 0, the containing block's own leading edge: a
// documented simplification of spec's "static position" algorithm, which
// would otherwise place the element roughly where it would have flowed.
func resolveAbsoluteAxis(startProp, endProp string, decls map[string]string, boxSize, cbSize int) int {
	switch {
	case decls[startProp] != "":
		v, _ := resolveMarginSide(decls[startProp], cbSize)
		return v
	case decls[endProp] != "":
		v, _ := resolveMarginSide(decls[endProp], cbSize)
		return cbSize - boxSize - v
	default:
		return 0
	}
}

// parseZIndex parses the z-index property, mirroring flex.go's parseOrder
// exactly: default 0, invalid or missing values also parse as 0.
func parseZIndex(decls map[string]string) int {
	n, err := strconv.Atoi(strings.TrimSpace(decls["z-index"]))
	if err != nil {
		return 0
	}
	return n
}

// resolvedOutOfFlow is one out-of-flow element's computed geometry from
// applyOutOfFlow's Phase A, carried into Phase B for z-index-ordered
// painting. rect is already clamped/clipped to its final painted bounds
// (clampOutOfFlowRect) — not just a raw offset computation — specifically
// so that a descendant using this element as its own containing block
// (resolvedRects in applyOutOfFlow) resolves against where this element
// actually ends up painted, not a pre-clamp position it may never occupy.
type resolvedOutOfFlow struct {
	node         *html.Node
	rect         Rect
	box          box
	subPositions map[*html.Node]Rect
	zIndex       int
}

// clampOutOfFlowRect clamps rect to its final painted bounds: Col to
// [0, width-rect.Width] (columns are a hard terminal-width boundary, never
// grown — the same convention compositeSelectPopup already uses), Row to
// >= 0 (a document can't grow upward), and — only when canGrow is false, a
// fixed Options.Height viewport — Height clipped so Row+Height doesn't
// exceed docHeight (trailing rows dropped, matching compositeSelectPopup's
// own clip-from-the-end convention). When canGrow is true, Height is left
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
// renderBlockContentBox (real CSS blockifies an out-of-flow element
// regardless of its own display value, so this is spec-correct, not a
// simplification), applies applyRelativeOffsets to that box's own local
// lines/subPositions (so a position: relative descendant nested inside this
// out-of-flow element's subtree still shifts — its whole subtree was
// skipped during normal layout, so Engine.RenderNode's own top-level
// applyRelativeOffsets call never saw it), and computes its final,
// already-clamped Rect (clampOutOfFlowRect) — processed in e.outOfFlowOrder
// (preorder: ancestors before descendants), so an absolute element whose
// containing block is itself out-of-flow always finds that ancestor's
// clamped geometry already resolved in resolvedRects.
//
// Phase B paints the results in z-index order (ties broken by the same
// document order collectOutOfFlow already established, via
// sort.SliceStable) using the same textcell.SpliceColumns-based painter's-algorithm
// compositing applyRelativeOffsets and compositeOpenSelects already use — a
// later splice fully overwrites whatever an earlier one painted into the
// same cells, no partial blending.
//
// Runs after applyRelativeOffsets (so a position: relative containing-block
// ancestor's shifted Rect is what out-of-flow descendants resolve against,
// matching real CSS) and before compositeOpenSelects (so an out-of-flow
// <select> — which has no Rect from normal layout, having been skipped
// there — gets one before the popup pass looks it up). canGrow mirrors
// compositeOpenSelects's own parameter: true lets a natural/automatic-height
// document grow to fit, false (a fixed Options.Height) clips rows that don't
// fit instead.
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

		bx, subPositions := e.renderBlockContentBox(n, decls, cb.Width)
		bx.lines, subPositions = e.applyRelativeOffsets(n, bx.lines, subPositions, bx.width)
		height := len(bx.lines)
		row := cb.Row + resolveAbsoluteAxis("top", "bottom", decls, height, cb.Height)
		col := cb.Col + resolveAbsoluteAxis("left", "right", decls, bx.width, cb.Width)

		rect := clampOutOfFlowRect(Rect{Row: row, Col: col, Width: bx.width, Height: height}, e.width, docHeight, canGrow)
		resolvedRects[n] = rect
		results = append(results, resolvedOutOfFlow{node: n, rect: rect, box: bx, subPositions: subPositions, zIndex: parseZIndex(decls)})
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].zIndex < results[j].zIndex })

	for _, res := range results {
		lines, positions = e.paintOutOfFlow(res, lines, positions)
	}
	return lines, positions
}

// paintOutOfFlow splices one resolvedOutOfFlow's already-clamped rect onto
// lines and records its (and its descendants') final Rect(s) — see
// applyOutOfFlow's doc comment for the two-phase design this is Phase B's
// per-element step of. Growing lines to fit res.rect (when Phase A's
// canGrow left Height unclipped) is the only bounds-handling left to do
// here — Col/Row/Height were already clamped/clipped in Phase A via
// clampOutOfFlowRect, precisely so a dependent descendant's own Phase A
// containing-block lookup sees the same bounds this paints at.
func (e *Engine) paintOutOfFlow(res resolvedOutOfFlow, lines []string, positions map[*html.Node]Rect) ([]string, map[*html.Node]Rect) {
	rect := res.rect
	if rect.Width <= 0 || rect.Height <= 0 {
		return lines, positions
	}
	for len(lines) < rect.Row+rect.Height {
		lines = append(lines, strings.Repeat(" ", e.width))
	}
	for i := range rect.Height {
		row := rect.Row + i
		lines[row] = textcell.SpliceColumns(lines[row], rect.Col, rect.Width, res.box.lines[i])
	}
	positions = mergePositions(positions, res.subPositions, rect.Row, rect.Col)
	positions[res.node] = rect
	return lines, positions
}
