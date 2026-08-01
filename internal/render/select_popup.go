package render

import (
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// compositeOpenSelects splices an open dropdown popup onto lines for every
// <select> in doc currently carrying e.selectOpenAttr. See
// docs/RENDERING.md's "Popups / z-order" section. The popup is composed as
// its own small block of lines, then spliced over the base lines at the
// select's own Rect via textcell.SpliceColumns, the primitive built for
// exactly this and otherwise unused until now. Runs after capBlankRuns/forceHeight in
// RenderNode, so it operates on the exact lines/positions about to be
// emitted, and can extend positions with synthetic Rects for each <option>
// so the existing elementAt/DispatchClick hit-testing works on them
// unmodified. canGrow reports whether lines may be extended with extra
// blank rows to fit a popup that doesn't otherwise have room below its
// select (true for natural/automatic height, false when Options.Height
// already fixed the document to an exact row count via forceHeight, in
// which case the popup is clipped to whatever room remains instead).
func (e *Engine) compositeOpenSelects(doc *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	if e.selectOpenAttr == "" || len(positions) == 0 {
		return lines, positions
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "select") && nodeHasAttr(n, e.selectOpenAttr) {
			lines, positions = e.compositeSelectPopup(n, lines, positions, canGrow)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return lines, positions
}

// popupRow is one row of an open <select>'s popup: either a navigable/
// clickable option (opt non-nil) or a non-navigable <optgroup> label header
// (opt nil, node the <optgroup> itself, for its own style lookup). indent is
// the extra left-padding applied to options nested inside a group, so a
// group's options visually sit under their header. See docs/SELECT.md.
type popupRow struct {
	opt    *html.Node
	node   *html.Node
	label  string
	indent int
}

// buildPopupRows walks sel's direct children into the flat row list
// compositeSelectPopup renders: a plain <option> becomes one option row; an
// <optgroup> becomes one label row, only if it has a non-empty label
// attribute, since an unlabeled optgroup still contributes its options but no
// header, followed by one indented option row per <option> child. It mirrors
// selectOptionNodes' one-level-deep descent into <optgroup>, but unlike that
// function also emits the label rows selectOptionNodes has no need to
// represent.
func buildPopupRows(sel *html.Node) []popupRow {
	var rows []popupRow
	for c := sel.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch {
		case strings.EqualFold(c.Data, "option"):
			rows = append(rows, popupRow{opt: c, node: c, label: selectOptionLabel(c)})
		case strings.EqualFold(c.Data, "optgroup"):
			if label := nodeAttr(c, "label"); label != "" {
				rows = append(rows, popupRow{node: c, label: label})
			}
			for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
				if gc.Type == html.ElementNode && strings.EqualFold(gc.Data, "option") {
					rows = append(rows, popupRow{opt: gc, node: gc, label: selectOptionLabel(gc), indent: 2})
				}
			}
		}
	}
	return rows
}

// compositeSelectPopup splices sel's row list (see popupRow/buildPopupRows)
// directly beneath its own Rect, styled per sel's/its <option>s' (and
// <optgroup>s', for label rows) own CSS (background-color, color, border,
// padding, margin, width on sel; background-color/color, plus `option:hover`
// for the highlighted row, per option) via overlay_box.go's
// resolveOverlayBoxStyle/drawOverlayFrame, falling back to the historical
// hardcoded reverse-video wrap for a marked option row when nothing in that
// chain sets color or background-color. Label rows get no such fallback; see
// the render loop below. See docs/RENDERING.md's "Popups / z-order" for
// why this stays a line-splice overlay rather than a real box-tree node.
//
// It does nothing if sel has no recorded Rect, meaning it wasn't laid out
// this frame, or has no rows. Otherwise it renders as many rows as fit, with
// canGrow deciding whether to extend lines with extra blank rows past their
// current end, the document's natural or automatic-height case, or to clip to
// whatever room already exists, the fixed-height case, so as not to exceed
// the caller's requested viewport. See compositeOpenSelects's doc comment.
// When clipping is forced, rows, whether option or label, are dropped first,
// then the bottom border and padding, and the top border and padding last, so
// a clipped popup never renders headless.
func (e *Engine) compositeSelectPopup(sel *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	rect, ok := positions[sel]
	if !ok {
		return lines, positions
	}
	rows := buildPopupRows(sel)
	if len(rows) == 0 {
		return lines, positions
	}

	popupAvail := e.width - rect.Col
	style := e.resolveOverlayBoxStyle(sel, popupAvail)

	const marker = "▸ "
	naturalWidth := rect.Width
	for _, r := range rows {
		// Column width, not rune count: the popup sizes itself to its
		// widest label and every row is then padded to that. Measured in
		// runes, a CJK/emoji label under-reports, and the reverse-video
		// highlight bar comes out visibly ragged row to row.
		w := textcell.Width(r.label) + r.indent
		if r.opt != nil {
			w += textcell.Width(marker)
		}
		if w > naturalWidth {
			naturalWidth = w
		}
	}
	contentWidth := naturalWidth
	if style.widthConstrained {
		contentWidth = style.width
	}

	blW, brW := textcell.Width(style.bl.char), textcell.Width(style.br.char)
	col := rect.Col + style.ml
	if !style.widthConstrained {
		// Natural (auto) sizing is capped to whatever room remains on this
		// row, matching the pre-CSS behavior; an explicit CSS width, like a
		// real block's, is respected even if it overflows past the screen
		// edge (mirroring renderBlockContentBox's own hasExplicitWidth
		// handling in block.go).
		if maxBoxWidth := e.width - col; contentWidth+style.pl+style.pr+blW+brW > maxBoxWidth {
			contentWidth -= (contentWidth + style.pl + style.pr + blW + brW) - maxBoxWidth
		}
	}
	if contentWidth < 1 {
		contentWidth = 1
	}
	pl, pr, innerW := clampCellPadding(contentWidth, style.pl, style.pr)
	boxWidth := blW + pl + innerW + pr + brW
	if boxWidth <= 0 {
		return lines, positions
	}
	if col+boxWidth > e.width {
		col = max(0, e.width-boxWidth)
	}

	topRows := overlayFrameRows(style, true)
	bottomRows := overlayFrameRows(style, false)
	startRow := rect.Row + rect.Height + style.mt
	count := len(rows)
	totalRows := topRows + count + bottomRows

	available := max(0, len(lines)-startRow)
	if totalRows > available {
		if canGrow {
			for range totalRows - available {
				lines = append(lines, strings.Repeat(" ", e.width))
			}
		} else {
			overflow := totalRows - available
			drop := min(count, overflow)
			count -= drop
			overflow -= drop
			if overflow > 0 {
				drop = min(bottomRows, overflow)
				bottomRows -= drop
				overflow -= drop
			}
			if overflow > 0 {
				drop = min(topRows, overflow)
				topRows -= drop
			}
			totalRows = topRows + count + bottomRows
		}
	}
	if totalRows <= 0 {
		return lines, positions
	}

	row := startRow
	if topRows > 0 {
		lines, row = e.drawOverlayFrame(lines, row, col, boxWidth, style, true)
	}

	// The "▸" marker follows the highlighted option (set by document's
	// moveSelectHighlight as the user arrows through the popup, separate
	// from "selected"; see selectHighlightAttr's doc comment for why
	// browsing shouldn't move the committed value). Fall back to "selected"
	// when no option carries the highlight attr at all: a popup opened by
	// setting selectOpenAttr directly in markup, with no live
	// openSelectPopup call behind it, never gets one. The same highlight
	// attribute also drives `option:hover` matching in the cascade (see
	// cssengine.Cascade.HoverAttr), and resolvePopupRowStyle below picks up
	// any such rule automatically, with no separate lookup needed here.
	// Group label rows, where opt is nil, are never highlighted or marked,
	// since they aren't navigable, so neither state can apply to one.
	highlightAttr := e.selectHighlightAttr
	anyHighlighted := false
	if highlightAttr != "" {
		for _, r := range rows {
			if r.opt != nil && nodeHasAttr(r.opt, highlightAttr) {
				anyHighlighted = true
				break
			}
		}
	}
	for i := range count {
		r := rows[i]
		if r.opt == nil {
			// A group label header: plain text, no marker or highlight, and no
			// reverse-video fallback, unlike the option rows below. That
			// fallback exists to make the highlighted or selected option
			// visually distinct against a styleless popup, and a
			// non-navigable label row has nothing to distinguish itself from.
			padded := padPlainToWidth(strings.Repeat(" ", r.indent)+r.label, innerW)
			rowStyle := e.resolvePopupRowStyle(r.node, style.base)
			rowContent := padded
			if rowStyle.has() {
				rowContent = rowStyle.render(padded, e.profile)
			}
			rowContent = strings.Repeat(" ", pl) + rowContent + strings.Repeat(" ", pr)
			rowContent = applyOverlaySideBorders(rowContent, style.bl, style.br, e.profile)
			lines[row] = textcell.SpliceColumns(lines[row], col, boxWidth, rowContent)
			row++
			continue
		}
		marked := false
		if anyHighlighted {
			marked = nodeHasAttr(r.opt, highlightAttr)
		} else {
			marked = nodeHasAttr(r.opt, "selected")
		}
		prefix := strings.Repeat(" ", r.indent+2)
		if marked {
			prefix = strings.Repeat(" ", r.indent) + marker
		}
		padded := padPlainToWidth(prefix+r.label, innerW)
		rowStyle := e.resolvePopupRowStyle(r.node, style.base)
		// The historical fallback, reached when no color or background-color
		// appears anywhere in sel's, opt's, or opt:hover's resolved decls,
		// reverse-videos every row uniformly rather than only the marked one,
		// leaving the "▸ " prefix as the only per-row distinction. See
		// TestSelectPopupComposition.
		var rowContent string
		if rowStyle.has() {
			rowContent = rowStyle.render(padded, e.profile)
		} else {
			rowContent = "\x1b[7m" + padded + "\x1b[27m"
		}
		rowContent = strings.Repeat(" ", pl) + rowContent + strings.Repeat(" ", pr)
		rowContent = applyOverlaySideBorders(rowContent, style.bl, style.br, e.profile)
		lines[row] = textcell.SpliceColumns(lines[row], col, boxWidth, rowContent)
		positions[r.opt] = Rect{Row: row, Col: col + blW + pl, Width: innerW, Height: 1}
		row++
	}

	if bottomRows > 0 {
		lines, _ = e.drawOverlayFrame(lines, row, col, boxWidth, style, false)
	}
	return lines, positions
}

// resolvePopupRowStyle resolves n's own cascaded color and background-color
// declarations as this row's style, falling back to popupBase, sel's own
// resolved style, for whichever of foreground and background n doesn't set
// itself. Those declarations include any matching `option:hover` ones, merged
// in by the normal cascade whenever an option n carries
// e.selectHighlightAttr (see cssengine.Cascade.HoverAttr).
//
// It is used for both option rows, where n is the <option>, and group label
// rows, where n is the <optgroup>. An `optgroup` selector styling its label
// row this way has no real-CSS equivalent, since real CSS barely styles
// <optgroup> at all. It is the same kind of terminal-native repurposing
// `option:hover` already is.
func (e *Engine) resolvePopupRowStyle(n *html.Node, popupBase inlineStyle) inlineStyle {
	s := extractInlineStyle(e.resolveDecls(n))
	if s.fg == nil {
		s.fg = popupBase.fg
	}
	if s.bg == nil {
		s.bg = popupBase.bg
	}
	return s
}

// padPlainToWidth pads or truncates s to exactly width visible *columns*. s
// is assumed to have no embedded ANSI sequences, which holds because every
// caller here builds it from plain extracted option text.
//
// Columns, not runes: every popup row is padded to the same width and then
// rendered under one highlight or background span, so a row measured in runes
// comes out visibly shorter or longer than its neighbours, leaving a ragged
// reverse-video bar. Truncation goes through textcell.VisiblePrefix, which
// declines to include a double-width cluster that would overshoot the
// boundary, so the pad below closes the resulting one-column gap rather than
// letting the row overflow by one.
func padPlainToWidth(s string, width int) string {
	if textcell.Width(s) > width {
		s = textcell.VisiblePrefix(s, width)
	}
	if pad := width - textcell.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
