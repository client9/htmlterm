package render

import (
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// compositeOpenDatalists splices a suggestion popup onto lines for every
// <input list> in doc currently carrying e.datalistOpenAttr. It is
// compositeOpenSelects' sibling, and works the same way: a small block of
// lines composed separately, then spliced over the base lines at the input's
// own Rect via textcell.SpliceColumns. See docs/DATALIST.md, and
// docs/RENDERING.md's "Popups / z-order" section for the compositing model
// both share.
//
// Runs after capBlankRuns/forceHeight in RenderNode, so it operates on the
// exact lines/positions about to be emitted, and can extend positions with
// synthetic Rects for each suggestion row so the existing
// elementAt/DispatchClick hit-testing works on them unmodified. canGrow
// reports whether lines may be extended with extra blank rows to fit a popup
// with no room below its input, exactly as for compositeOpenSelects.
func (e *Engine) compositeOpenDatalists(doc *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	if e.datalistOpenAttr == "" || len(positions) == 0 {
		return lines, positions
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "input") && nodeHasAttr(n, e.datalistOpenAttr) {
			lines, positions = e.compositeDatalistPopup(n, doc, lines, positions, canGrow)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return lines, positions
}

// datalistFor returns the <datalist> element input's list attribute names, or
// nil if the attribute is absent, no element carries that id, or the element
// that does isn't a <datalist>. It mirrors document's own datalistFor; the
// two packages deliberately don't share code (see docs/ARCHITECTURE.md's
// package-split rationale, and select.go's selectOptionNodes for the same
// kind of duplication), so this copy must be kept in lockstep with that one.
func datalistFor(input, doc *html.Node) *html.Node {
	id := nodeAttr(input, "list")
	if id == "" {
		return nil
	}
	var found *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && nodeAttr(n, "id") == id {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == nil || !strings.EqualFold(found.Data, "datalist") {
		return nil
	}
	return found
}

// datalistMatchRows returns the <option> children of list carrying
// e.datalistMatchAttr, in document order: the suggestions the document layer
// has already decided match what's been typed. The renderer never applies the
// matching rule itself; see defaultDatalistMatchAttr.
func (e *Engine) datalistMatchRows(list *html.Node) []*html.Node {
	var out []*html.Node
	for c := list.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "option") && nodeHasAttr(c, e.datalistMatchAttr) {
			out = append(out, c)
		}
	}
	return out
}

// datalistOptionLabel returns opt's visible text in a suggestion popup: its
// label attribute, else its text content, else its value attribute.
//
// The value fallback is what separates this from selectOptionLabel
// (formcontrol.go), which stops at text content. A <select>'s options
// practically always have text, since that text is the whole control's
// display; a <datalist>'s practically never do, `<option value="apple">`
// being the ordinary form. Falling back to value is what keeps such a row
// from rendering blank.
func datalistOptionLabel(opt *html.Node) string {
	if label := selectOptionLabel(opt); label != "" {
		return label
	}
	return nodeAttr(opt, "value")
}

// compositeDatalistPopup splices input's suggestion rows directly beneath its
// own Rect, styled per the referenced <datalist>'s own CSS through
// overlay_box.go's resolveOverlayBoxStyle/drawOverlayFrame, and per each
// <option>'s own through resolvePopupRowStyle. The <datalist> is the styling
// hook because it renders nothing itself (the UA stylesheet gives it
// display:none), leaving its whole declaration block free to mean "the
// popup", the same way a <select>'s own declarations style its dropdown.
//
// It does nothing if input has no recorded Rect, meaning it wasn't laid out
// this frame, if its list attribute resolves to nothing, or if no option
// currently matches. Otherwise it renders as many rows as fit, with canGrow
// deciding whether to extend lines past their current end or clip to whatever
// room already exists, and clipping drops rows first, then the bottom border
// and padding, and the top border and padding last, so a clipped popup never
// renders headless. That is compositeSelectPopup's rule, followed here for
// the same reason: the two popups should not behave differently at the
// bottom of a fixed-height screen.
func (e *Engine) compositeDatalistPopup(input, doc *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	rect, ok := positions[input]
	if !ok {
		return lines, positions
	}
	list := datalistFor(input, doc)
	if list == nil {
		return lines, positions
	}
	opts := e.datalistMatchRows(list)
	if len(opts) == 0 {
		return lines, positions
	}

	style := e.resolveOverlayBoxStyle(list, e.width-rect.Col)

	const marker = "▸ "
	// The popup is at least as wide as the field it drops out of, so a
	// suggestion list never looks narrower than what it's completing.
	naturalWidth := rect.Width
	for _, opt := range opts {
		// Columns, not runes: a CJK or emoji label under-reports as runes and
		// leaves the highlight bar visibly ragged row to row. Same reasoning
		// as compositeSelectPopup's own width loop.
		if w := textcell.Width(datalistOptionLabel(opt)) + textcell.Width(marker); w > naturalWidth {
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
		// Natural sizing is capped to the room left on the row; an explicit
		// CSS width is respected even past the screen edge, mirroring
		// compositeSelectPopup and renderBlockContentBox's hasExplicitWidth.
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
	count := len(opts)
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
				topRows -= min(topRows, overflow)
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

	// The "▸" marker follows the highlighted row, the one document's
	// moveDatalistHighlight sets as the user arrows through the popup. That
	// is the same reserved attribute a <select>'s popup uses, so the same
	// `option:hover` rule styles both (see cssengine.Cascade.HoverAttr).
	// Unlike a <select> there is no "selected" fallback: a datalist option is
	// never selected, only suggested, so a popup with nothing highlighted
	// simply shows no marker.
	for i := range count {
		opt := opts[i]
		prefix := "  "
		if e.selectHighlightAttr != "" && nodeHasAttr(opt, e.selectHighlightAttr) {
			prefix = marker
		}
		padded := padPlainToWidth(prefix+datalistOptionLabel(opt), innerW)
		rowStyle := e.resolvePopupRowStyle(opt, style.base)
		var content string
		if rowStyle.has() {
			content = rowStyle.render(padded, e.profile)
		} else {
			content = "\x1b[7m" + padded + "\x1b[27m"
		}
		content = strings.Repeat(" ", pl) + content + strings.Repeat(" ", pr)
		content = applyOverlaySideBorders(content, style.bl, style.br, e.profile)
		lines[row] = textcell.SpliceColumns(lines[row], col, boxWidth, content)
		// The content sub-rectangle only, excluding border and padding, so a
		// click on the popup's own chrome isn't misattributed to a row.
		positions[opt] = Rect{Row: row, Col: col + blW + pl, Width: innerW, Height: 1}
		row++
	}

	if bottomRows > 0 {
		lines, _ = e.drawOverlayFrame(lines, row, col, boxWidth, style, false)
	}
	return lines, positions
}
