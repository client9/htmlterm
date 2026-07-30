package render

import (
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// applyRelativeOffsets walks doc for every element with position: relative
// (and a non-zero top/right/bottom/left offset) whose Rect was recorded in
// positions, and shifts its painted content — cutting its own rectangle out
// of lines, blanking the original, splicing it back in at the offset
// location — along with its own and its descendants' recorded Rects, so
// hit-testing (document.elementAt) matches what's actually drawn. width is
// the horizontal bound lines is clamped against — see docs/RENDERING.md's
// "Popups / z-order" section for the splice/extract primitives this reuses,
// and this function's own doc comment there for why this is a post-layout
// compositing pass rather than a change to renderBlockContentBox's layout
// math: the element still reserves its original space in normal flow,
// exactly like real CSS position: relative, it just paints shifted.
//
// Two callers: Engine.RenderNode, after capBlankRuns/forceHeight (so row
// indices are final) and before applyOutOfFlow/compositeOpenSelects (so an
// out-of-flow or open-<select> element that is itself position: relative
// anchors off the shifted position, not the original layout slot) — doc is
// the whole document and width is e.width there. And applyOutOfFlow's Phase
// A, on a single out-of-flow element's own freshly rendered box (doc is
// that element, lines/positions are its own local box/subPositions, width
// is that box's own width) — this is what makes a position: relative
// descendant nested inside position: absolute/fixed content shift at all;
// without it, such a descendant is never in Engine.RenderNode's top-level
// positions map (its whole subtree was skipped during normal layout, see
// outofflow.go), so the top-level call alone would silently never find it.
//
// Processed in document order; a later relatively-positioned element's
// splice can paint over an earlier one's shifted content (no z-index for
// position: relative itself, though an out-of-flow element with position:
// relative content nested inside it does still get z-index-ordered against
// other out-of-flow elements — see outofflow.go). A shift that would move
// content outside lines' current row/col bounds is clamped rather than
// growing the canvas.
func (e *Engine) applyRelativeOffsets(doc *html.Node, lines []string, positions map[*html.Node]Rect, width int) ([]string, map[*html.Node]Rect) {
	if len(positions) == 0 {
		return lines, positions
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if decls := e.resolveDecls(n); decls["position"] == "relative" {
				lines, positions = e.shiftRelativeElement(n, decls, lines, positions, width)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return lines, positions
}

// resolveRelativeOffset resolves the effective (dRow, dCol) shift for a
// position: relative element from its own already-resolved decls. Per spec,
// top wins over bottom and left wins over right when both sides of an axis
// are set (non-auto); resolveMarginSide (box.go) already treats "auto" as a
// 0 offset, which is exactly the fallback behavior wanted here. Percentage
// values resolve against width (left/right) or docHeight (top/bottom) — an
// approximation of the real spec's containing-block-relative percentage
// resolution, consistent with how percentages are already approximated
// elsewhere in this renderer.
func (e *Engine) resolveRelativeOffset(decls map[string]string, docHeight, width int) (dRow, dCol int) {
	switch {
	case decls["top"] != "":
		dRow, _ = resolveMarginSide(decls["top"], docHeight)
	case decls["bottom"] != "":
		v, _ := resolveMarginSide(decls["bottom"], docHeight)
		dRow = -v
	}
	switch {
	case decls["left"] != "":
		dCol, _ = resolveMarginSide(decls["left"], width)
	case decls["right"] != "":
		v, _ := resolveMarginSide(decls["right"], width)
		dCol = -v
	}
	return dRow, dCol
}

// shiftRelativeElement performs one element's shift: extract its own
// rectangle out of lines, blank the original, splice it back in at the
// offset (clamped to stay within lines' existing bounds, horizontally
// against width), and shift n's own and every descendant's recorded Rect to
// match.
func (e *Engine) shiftRelativeElement(n *html.Node, decls map[string]string, lines []string, positions map[*html.Node]Rect, width int) ([]string, map[*html.Node]Rect) {
	rect, ok := positions[n]
	if !ok {
		return lines, positions
	}
	dRow, dCol := e.resolveRelativeOffset(decls, len(lines), width)
	if dRow == 0 && dCol == 0 {
		return lines, positions
	}
	if rect.Row < 0 || rect.Row+rect.Height > len(lines) {
		return lines, positions
	}

	extracted := make([]string, rect.Height)
	for i := range rect.Height {
		extracted[i] = textcell.VisibleWindow(lines[rect.Row+i], rect.Col, rect.Width)
	}
	blank := strings.Repeat(" ", rect.Width)
	for i := range rect.Height {
		row := rect.Row + i
		lines[row] = textcell.SpliceColumns(lines[row], rect.Col, rect.Width, blank)
	}

	newRow := max(0, min(rect.Row+dRow, len(lines)-rect.Height))
	newCol := max(0, min(rect.Col+dCol, width-rect.Width))
	for i := range rect.Height {
		row := newRow + i
		lines[row] = textcell.SpliceColumns(lines[row], newCol, rect.Width, extracted[i])
	}

	shiftRow, shiftCol := newRow-rect.Row, newCol-rect.Col
	var shift func(nn *html.Node)
	shift = func(nn *html.Node) {
		if r, ok := positions[nn]; ok {
			r.Row += shiftRow
			r.Col += shiftCol
			positions[nn] = r
		}
		for c := nn.FirstChild; c != nil; c = c.NextSibling {
			shift(c)
		}
	}
	shift(n)

	return lines, positions
}
