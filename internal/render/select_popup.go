package render

import (
	"strings"

	"golang.org/x/net/html"
)

// compositeOpenSelects splices an open dropdown popup onto lines for every
// <select> in doc currently carrying e.selectOpenAttr — see docs/RENDERING.md's
// "Popups / z-order" section: the popup is composed as its own little block
// of lines, then spliced over the base lines at the select's own Rect via
// spliceColumns (textutil.go), the primitive built for exactly this and
// otherwise unused until now. Runs after capBlankRuns/forceHeight in
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

// compositeSelectPopup splices sel's option list directly beneath its own
// Rect, styled per sel's/its <option>s' own CSS (background-color, color,
// border, padding, margin, width on sel; background-color/color, plus
// `option:hover` for the highlighted row, per option) via overlay_box.go's
// resolveOverlayBoxStyle/drawOverlayFrame, falling back to the historical
// hardcoded reverse-video wrap for a marked row when nothing in that chain
// sets color/background-color — see docs/RENDERING.md's "Popups / z-order"
// for why this stays a line-splice overlay rather than a real box-tree node.
// Does nothing if sel has no recorded Rect (not laid out this frame) or no
// options, or renders as many options as fit — canGrow decides whether to
// extend lines with extra blank rows past its current end (the document's
// natural/automatic-height case) or clip to whatever room already exists
// (the fixed-height case, so as not to exceed the caller's requested
// viewport) — see compositeOpenSelects's doc comment. When clipping is
// forced, option rows are dropped first, then the bottom border/padding,
// and the top border/padding last — so a clipped popup never renders
// headless.
func (e *Engine) compositeSelectPopup(sel *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	rect, ok := positions[sel]
	if !ok {
		return lines, positions
	}
	options := selectOptionNodes(sel)
	if len(options) == 0 {
		return lines, positions
	}

	popupAvail := e.width - rect.Col
	style := e.resolveOverlayBoxStyle(sel, popupAvail)

	const marker = "▸ "
	labels := make([]string, len(options))
	naturalWidth := rect.Width
	for i, opt := range options {
		labels[i] = selectOptionLabel(opt)
		if w := len([]rune(marker + labels[i])); w > naturalWidth {
			naturalWidth = w
		}
	}
	contentWidth := naturalWidth
	if style.widthConstrained {
		contentWidth = style.width
	}

	blW, brW := runeLen(style.bl.char), runeLen(style.br.char)
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
	count := len(options)
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
	// from "selected" — see selectHighlightAttr's doc comment for why
	// browsing shouldn't move the committed value). Fall back to "selected"
	// when no option carries the highlight attr at all — a popup opened by
	// setting selectOpenAttr directly in markup, with no live
	// openSelectPopup call behind it, never gets one. The same highlight
	// attribute also drives `option:hover` matching in the cascade (see
	// cssengine.Cascade.HoverAttr) — resolveOptionRowStyle below picks up
	// any such rule automatically, with no separate lookup needed here.
	highlightAttr := e.selectHighlightAttr
	anyHighlighted := false
	if highlightAttr != "" {
		for _, opt := range options {
			if nodeHasAttr(opt, highlightAttr) {
				anyHighlighted = true
				break
			}
		}
	}
	for i := range count {
		opt := options[i]
		marked := false
		if anyHighlighted {
			marked = nodeHasAttr(opt, highlightAttr)
		} else {
			marked = nodeHasAttr(opt, "selected")
		}
		prefix := "  "
		if marked {
			prefix = marker
		}
		padded := padPlainToWidth(prefix+labels[i], innerW)
		rowStyle := e.resolveOptionRowStyle(opt, style.base)
		// The historical fallback (no color/background-color anywhere in
		// sel/opt/opt:hover's resolved decls) reverse-videos every row
		// uniformly — not just the marked one — with the "▸ " prefix as the
		// only per-row distinction; see TestSelectPopupComposition.
		var rowContent string
		if rowStyle.has() {
			rowContent = rowStyle.render(padded, e.profile)
		} else {
			rowContent = "\x1b[7m" + padded + "\x1b[27m"
		}
		rowContent = strings.Repeat(" ", pl) + rowContent + strings.Repeat(" ", pr)
		rowContent = applyOverlaySideBorders(rowContent, style.bl, style.br, e.profile)
		lines[row] = spliceColumns(lines[row], col, boxWidth, rowContent)
		positions[opt] = Rect{Row: row, Col: col + blW + pl, Width: innerW, Height: 1}
		row++
	}

	if bottomRows > 0 {
		lines, _ = e.drawOverlayFrame(lines, row, col, boxWidth, style, false)
	}
	return lines, positions
}

// resolveOptionRowStyle resolves opt's own cascaded declarations (color/
// background-color — including any matching `option:hover` declarations,
// merged in by the normal cascade whenever opt carries e.selectHighlightAttr,
// see cssengine.Cascade.HoverAttr) as this row's style, falling back to
// popupBase (sel's own resolved style) for whichever of fg/bg opt doesn't
// set itself.
func (e *Engine) resolveOptionRowStyle(opt *html.Node, popupBase inlineStyle) inlineStyle {
	s := extractInlineStyle(e.resolveDecls(opt))
	if s.fg == nil {
		s.fg = popupBase.fg
	}
	if s.bg == nil {
		s.bg = popupBase.bg
	}
	return s
}

// padPlainToWidth pads or truncates s (assumed to have no embedded ANSI
// sequences — every caller here builds it from plain extracted option text)
// to exactly width visible runes.
func padPlainToWidth(s string, width int) string {
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	if len(r) < width {
		return s + strings.Repeat(" ", width-len(r))
	}
	return s
}
