package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// overlayBoxStyle is the resolved CSS box geometry/colors for a floating
// overlay — a <select> dropdown popup today; a future tooltip or menu popup
// could resolve one against its own trigger node. Composited onto
// already-serialized lines post-layout, independent of the box-tree pipeline
// — see docs/RENDERING.md's "Popups / z-order" section for why an overlay is
// never routed through renderBlockContentBox.
type overlayBoxStyle struct {
	bl, br, bt, bb                         blockBorder
	tlCorner, trCorner, blCorner, brCorner string
	pl, pr, pt, pb                         int
	// ml/mt are margin-left/margin-top, shifting the overlay's own start
	// column/row relative to its trigger element's Rect. margin-right/
	// margin-bottom are not supported — an overlay has no following sibling
	// content for them to push against, so they'd have no visible effect.
	ml, mt int
	base   inlineStyle
	// width/widthConstrained mirror resolveWidthConstraints: width is only
	// meaningful when widthConstrained is true (an explicit width/min-width/
	// max-width declaration), otherwise the caller's own natural-width
	// computation (e.g. option-label measuring) wins.
	width            int
	widthConstrained bool
}

// resolveOverlayBoxStyle resolves n's own cascaded declarations into an
// overlayBoxStyle. availWidth is the overlay's left-edge-relative budget
// (e.width - originCol), matching resolveWidthConstraints's/
// resolveMarginSide's existing availWidth convention.
func (e *Engine) resolveOverlayBoxStyle(n *html.Node, availWidth int) overlayBoxStyle {
	decls := e.resolveDecls(n)
	bl, br, bt, bb, tlCorner, trCorner, blCorner, brCorner := resolveBoxBorders(decls)
	ml, _ := resolveMarginSide(decls["margin-left"], availWidth)
	width, constrained := resolveWidthConstraints(decls, availWidth, availWidth)
	return overlayBoxStyle{
		bl: bl, br: br, bt: bt, bb: bb,
		tlCorner: tlCorner, trCorner: trCorner, blCorner: blCorner, brCorner: brCorner,
		pl:   parsePaddingLen(decls["padding-left"]),
		pr:   parsePaddingLen(decls["padding-right"]),
		pt:   parsePaddingLen(decls["padding-top"]),
		pb:   parsePaddingLen(decls["padding-bottom"]),
		ml:   ml,
		mt:   parseMargin(decls["margin-top"]),
		base: extractInlineStyle(decls),

		width:            width,
		widthConstrained: constrained,
	}
}

// drawOverlayFrame splices one edge's worth of border/padding rows for an
// overlay box of the given content boxWidth at (row, col) onto lines — the
// top border (if any) then style.pt blank padding rows when top is true, or
// style.pb blank padding rows then the bottom border (if any) when top is
// false. lines must already have enough rows allocated for whatever this
// call is about to write; row accounting (how many rows that is, and whether
// lines needs to grow to fit) is the caller's responsibility, since that
// depends on the caller's own clipping/growth rules (e.g.
// compositeSelectPopup's canGrow). Returns the updated lines and the row
// index immediately after what was drawn — the first content row, when
// top is true.
func (e *Engine) drawOverlayFrame(lines []string, row, col, boxWidth int, style overlayBoxStyle, top bool) ([]string, int) {
	interior := max(0, boxWidth-runeLen(style.bl.char)-runeLen(style.br.char))
	blank := func() string {
		content := style.base.render(strings.Repeat(" ", interior), e.profile)
		return applyOverlaySideBorders(content, style.bl, style.br, e.profile)
	}
	if top {
		if style.bt.char != "" {
			border := drawBlockHBorder(style.bt.char, style.bt.color, style.tlCorner, style.trCorner, boxWidth, e.profile)
			lines[row] = spliceColumns(lines[row], col, boxWidth, border)
			row++
		}
		for range style.pt {
			lines[row] = spliceColumns(lines[row], col, boxWidth, blank())
			row++
		}
		return lines, row
	}
	for range style.pb {
		lines[row] = spliceColumns(lines[row], col, boxWidth, blank())
		row++
	}
	if style.bb.char != "" {
		border := drawBlockHBorder(style.bb.char, style.bb.color, style.blCorner, style.brCorner, boxWidth, e.profile)
		lines[row] = spliceColumns(lines[row], col, boxWidth, border)
		row++
	}
	return lines, row
}

// applyOverlaySideBorders wraps content (already exactly boxWidth minus the
// left/right border-char width) with left/right's painted border characters,
// left as-is if neither is set.
func applyOverlaySideBorders(content string, left, right blockBorder, p colorprofile.Profile) string {
	if left.char == "" && right.char == "" {
		return content
	}
	paintL := makePainter(left.color, p)
	paintR := makePainter(right.color, p)
	return paintL(left.char) + content + paintR(right.char)
}

// overlayFrameRows returns how many extra rows style's border/padding adds on
// one edge — the top edge (border-top present + padding-top) when top is
// true, the bottom edge (padding-bottom + border-bottom present) otherwise.
// Shared by drawOverlayFrame's caller to size lines before drawing.
func overlayFrameRows(style overlayBoxStyle, top bool) int {
	if top {
		n := style.pt
		if style.bt.char != "" {
			n++
		}
		return n
	}
	n := style.pb
	if style.bb.char != "" {
		n++
	}
	return n
}
