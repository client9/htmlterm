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
// Rect, reverse-video styled so it visually reads as a popup on top of
// whatever content it overlaps — the only per-cell distinction it needs, per
// docs/RENDERING.md's "opaque rectangular overlay, no per-cell blending"
// decision. Does nothing if sel has no recorded Rect (not laid out this
// frame) or no options, or renders as many options as fit — canGrow decides
// whether to extend lines with extra blank rows past its current end (the
// document's natural/automatic-height case) or clip to whatever room
// already exists (the fixed-height case, so as not to exceed the caller's
// requested viewport) — see compositeOpenSelects's doc comment.
func (e *Engine) compositeSelectPopup(sel *html.Node, lines []string, positions map[*html.Node]Rect, canGrow bool) ([]string, map[*html.Node]Rect) {
	rect, ok := positions[sel]
	if !ok {
		return lines, positions
	}
	options := selectOptionNodes(sel)
	if len(options) == 0 {
		return lines, positions
	}
	const marker = "▸ "
	width := rect.Width
	labels := make([]string, len(options))
	for i, opt := range options {
		labels[i] = selectOptionLabel(opt)
		if w := len([]rune(marker + labels[i])); w > width {
			width = w
		}
	}
	if maxWidth := e.width - rect.Col; width > maxWidth {
		width = maxWidth
	}
	if width <= 0 {
		return lines, positions
	}
	startRow := rect.Row + rect.Height
	count := len(options)
	if needed := startRow + count - len(lines); needed > 0 {
		if !canGrow {
			count -= needed
		} else {
			for range needed {
				lines = append(lines, strings.Repeat(" ", e.width))
			}
		}
	}
	if count <= 0 {
		return lines, positions
	}
	// The "▸" marker follows the highlighted option (set by document's
	// moveSelectHighlight as the user arrows through the popup, separate
	// from "selected" — see selectHighlightAttr's doc comment for why
	// browsing shouldn't move the committed value). Fall back to "selected"
	// when no option carries the highlight attr at all — a popup opened by
	// setting selectOpenAttr directly in markup, with no live
	// openSelectPopup call behind it, never gets one.
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
		prefix := "  "
		marked := false
		if anyHighlighted {
			marked = nodeHasAttr(opt, highlightAttr)
		} else {
			marked = nodeHasAttr(opt, "selected")
		}
		if marked {
			prefix = marker
		}
		content := padPlainToWidth(prefix+labels[i], width)
		row := startRow + i
		lines[row] = spliceColumns(lines[row], rect.Col, width, "\x1b[7m"+content+"\x1b[27m")
		positions[opt] = Rect{Row: row, Col: rect.Col, Width: width, Height: 1}
	}
	return lines, positions
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
