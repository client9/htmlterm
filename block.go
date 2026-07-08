package htmlterm

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// blockBorder holds the character and optional color for one edge of a block border.
type blockBorder struct {
	char  string
	color string
}

// applyLineEdgesBox prepends prefix and appends suffix to every line of b.
func applyLineEdgesBox(b box, prefix, suffix string) box {
	if prefix == "" && suffix == "" {
		return b
	}
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		lines[i] = prefix + line + suffix
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// applyLineEdges is a string-signature shim over applyLineEdgesBox, preserving
// the historical quirk that a trailing "\n" on content is stripped before the
// per-line transform and restored after — content has no trailing-newline
// concept in box form, so that behavior lives here rather than in the core.
func applyLineEdges(content, prefix, suffix string) string {
	if prefix == "" && suffix == "" {
		return content
	}
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	result := applyLineEdgesBox(newBox(content), prefix, suffix).join()
	if trailing {
		result += "\n"
	}
	return result
}

// alignLinesBox pads every line of b to width (lines already >= width are
// left unchanged), aligning left/right/center per dir.
func alignLinesBox(b box, dir string, width int) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		vl := ansiVisibleLen(line)
		if vl >= width {
			lines[i] = line
			continue
		}
		pad := width - vl
		switch dir {
		case "right":
			lines[i] = strings.Repeat(" ", pad) + line
		case "center":
			left := pad / 2
			lines[i] = strings.Repeat(" ", left) + line + strings.Repeat(" ", pad-left)
		default:
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// alignLines is a string-signature shim over alignLinesBox; see applyLineEdges
// for why the trailing-newline handling lives in the shim, not the core.
func alignLines(content, dir string, width int) string {
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	result := alignLinesBox(newBox(content), dir, width).join()
	if trailing {
		result += "\n"
	}
	return result
}

// padLinesToWidthBox pads every line of b shorter than width with trailing
// spaces; lines already >= width are left unchanged.
func padLinesToWidthBox(b box, width int) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		if vl := ansiVisibleLen(line); vl < width {
			lines[i] = line + strings.Repeat(" ", width-vl)
		} else {
			lines[i] = line
		}
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// padLinesToWidth is a string-signature shim over padLinesToWidthBox; see
// applyLineEdges for why the trailing-newline handling lives in the shim.
func padLinesToWidth(content string, width int) string {
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	result := padLinesToWidthBox(newBox(content), width).join()
	if trailing {
		result += "\n"
	}
	return result
}

func drawBlockHBorder(fill, color, leftCorner, rightCorner string, width int, p colorprofile.Profile) string {
	if fill == "" || fill == "none" || width <= 0 {
		return ""
	}
	lc := leftCorner
	if lc == "" {
		lc = fill
	}
	rc := rightCorner
	if rc == "" {
		rc = fill
	}
	return drawHRule([]int{max(0, width-runeLen(lc)-runeLen(rc))}, fill, color, lc, "", rc, p)
}

func resolveCSSSize(s string, availWidth int) (int, bool) {
	abs, pct, ok := parseSizeVal(s)
	if !ok {
		return 0, false
	}
	if pct > 0 {
		return int(pct * float64(availWidth)), true
	}
	return abs, true
}

func resolveWidthConstraints(decls map[string]string, availWidth, naturalWidth int) (width int, constrained bool) {
	width = naturalWidth
	if w, ok := resolveCSSSize(decls["width"], availWidth); ok {
		width = w
		constrained = true
	}
	if w, ok := resolveCSSSize(decls["max-width"], availWidth); ok && width > w {
		width = w
		constrained = true
	}
	if w, ok := resolveCSSSize(decls["min-width"], availWidth); ok && width < w {
		width = w
		constrained = true
	}
	return width, constrained
}

func maxVisibleLineWidth(s string) int {
	maxW := 0
	for _, line := range strings.Split(s, "\n") {
		if w := ansiVisibleLen(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// applyBlockBordersBox prepends the painted left border char and appends the
// painted right border char to every line of b.
func applyBlockBordersBox(b box, left, right blockBorder, p colorprofile.Profile) box {
	paintL := makePainter(left.color, p)
	paintR := makePainter(right.color, p)
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		lines[i] = paintL(left.char) + line + paintR(right.char)
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// parseMargin parses a CSS margin-top / margin-bottom value as a line count.
func parseMargin(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// renderBlockContent renders the styled, bordered, and margined content of a
// block element. Thin shim over renderBlockContentBox for callers not yet
// migrated to box.
func (r *Renderer) renderBlockContent(n *html.Node, decls map[string]string, availWidth int) string {
	b, _ := r.renderBlockContentBox(n, decls, availWidth)
	return b.join()
}

// renderBlockContentBox is renderBlockContent's box-based core. It preserves
// the exact operation order of the original string implementation (border
// resolution → margin/padding resolution → clampCellPadding → inline content
// → wrap → overflow/text-overflow → align → padLinesToWidth fallback →
// height padding → text-indent → vertical padding → horizontal padding →
// borders → top/bottom rules → margins → visibility:hidden blanking, last).
func (r *Renderer) renderBlockContentBox(n *html.Node, decls map[string]string, availWidth int) (box, map[*html.Node]Rect) {
	bl := blockBorder{char: parseCSSString(decls["border-left"]), color: decls["border-left-color"]}
	br := blockBorder{char: parseCSSString(decls["border-right"]), color: decls["border-right-color"]}
	bt := blockBorder{char: parseCSSString(decls["border-top"]), color: decls["border-top-color"]}
	bb := blockBorder{char: parseCSSString(decls["border-bottom"]), color: decls["border-bottom-color"]}
	tlCorner := parseCSSString(decls["border-top-left-corner"])
	trCorner := parseCSSString(decls["border-top-right-corner"])
	blCorner := parseCSSString(decls["border-bottom-left-corner"])
	brCorner := parseCSSString(decls["border-bottom-right-corner"])
	if styleVal := decls["border-style"]; styleVal != "" {
		if ts, ok := namedTableStyle(styleVal); ok {
			if bl.char == "" {
				bl.char = ts.left
			}
			if br.char == "" {
				br.char = ts.right
			}
			if ts.top != nil {
				if bt.char == "" {
					bt.char = ts.top.fill
				}
				if tlCorner == "" {
					tlCorner = ts.top.left
				}
				if trCorner == "" {
					trCorner = ts.top.right
				}
			}
			if ts.bottom != nil {
				if bb.char == "" {
					bb.char = ts.bottom.fill
				}
				if blCorner == "" {
					blCorner = ts.bottom.left
				}
				if brCorner == "" {
					brCorner = ts.bottom.right
				}
			}
			if ts.color != "" {
				if bl.color == "" {
					bl.color = ts.color
				}
				if br.color == "" {
					br.color = ts.color
				}
				if bt.color == "" {
					bt.color = ts.color
				}
				if bb.color == "" {
					bb.color = ts.color
				}
			}
		}
	}
	ml, mlAuto := resolveMarginSide(decls["margin-left"], availWidth)
	mr, mrAuto := resolveMarginSide(decls["margin-right"], availWidth)
	pl := parsePaddingLen(decls["padding-left"])
	pr := parsePaddingLen(decls["padding-right"])
	pt := parsePaddingLen(decls["padding-top"])
	pb := parsePaddingLen(decls["padding-bottom"])
	hBorderWidth := availWidth - ml - mr
	acc := extractInlineStyle(decls)
	textAlign := decls["text-align"]
	// ovX/ovY are overflow-x/overflow-y — see SCROLLING.md's "Scrollbar
	// gutter and indicator": expandShorthand (css.go) already expands a
	// plain overflow:<val> shorthand into both, so these are correct even
	// when only the shorthand was ever set; a more specific overflow-x/-y
	// declaration overrides just that axis via the normal per-property
	// cascade (cascade.go's directDecls). ovX gates the width-truncation
	// check below; ovY gates the height/scroll gate and gutter reservation.
	ovX := decls["overflow-x"]
	ovY := decls["overflow-y"]
	heightLines := 0
	if v := decls["height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			heightLines = abs
		}
	}

	hasExplicitWidth := false
	if totalW, constrained := resolveWidthConstraints(decls, availWidth, availWidth); constrained {
		inner := totalW - ml - runeLen(bl.char) - pl - pr - runeLen(br.char) - mr
		if inner > 0 {
			hBorderWidth = runeLen(bl.char) + pl + inner + pr + runeLen(br.char)
			hasExplicitWidth = true
		}
	}
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		ml, mr = splitAutoMargins(remaining, ml, mr, mlAuto, mrAuto)
	}

	avail := hBorderWidth - runeLen(bl.char) - runeLen(br.char)
	// gutterWidth reserves a column for the scrollbar indicator up front,
	// before wrapping — see SCROLLING.md's "Scrollbar gutter and indicator"
	// for why this must happen before wordWrapTokens runs (below), not as a
	// post-hoc overlay. Silently dropped (gutterWidth stays 0) if there
	// isn't room for it, rather than collapsing content to 0 width.
	gutterWidth := 0
	if heightLines > 0 && ovY == "scroll" && avail-scrollbarGutterWidth >= 1 {
		gutterWidth = scrollbarGutterWidth
	}
	hasScrollbarGutter := gutterWidth > 0
	var innerW int
	pl, pr, innerW = clampCellPadding(avail-gutterWidth, pl, pr)
	if innerW < 1 {
		innerW = 1
	}
	var tokens []wrapToken
	if n.Data == "textarea" {
		// <textarea>'s current value is its "value" attribute in this
		// package's simplified form-control model (matching
		// Element.Value/SetValue, which always reads/writes that attribute
		// for every control) — but real HTML's default value for a
		// never-touched textarea is its child text, with one leading
		// newline right after the opening tag ignored per spec, so fall
		// back to that when no value attribute has been set yet.
		// appendText already knows how to split embedded "\n"s (a
		// multi-line value) into brk tokens, so the rest of this function's
		// wrap/border/padding handling applies exactly as it would to any
		// other block's content.
		val := nodeAttr(n, "value")
		if val == "" {
			val = strings.TrimPrefix(rawContent(n), "\n")
		}
		tokens = appendText(nil, inlineStyle{}, val, r.profile)
	} else {
		tokens = r.renderInlineAccTokens(n, acc, innerW)
	}
	for len(tokens) > 0 && tokens[len(tokens)-1].brk {
		tokens = tokens[:len(tokens)-1]
	}
	wasWrapped := false
	ws := decls["white-space"]
	if ws != "pre" && ws != "pre-wrap" && len(tokens) > 0 {
		last := len(tokens) - 1
		if tokens[last].box == nil && !tokens[last].brk {
			t := tokens[last].text
			stripped := stripANSI(t)
			if strings.HasSuffix(stripped, " ") && !strings.HasSuffix(stripped, "  ") {
				if trimmed, ok := trimOneTrailingVisibleSpace(t); ok {
					tokens[last] = wrapToken{text: trimmed}
				}
			}
		}
	}
	// hasStructure mirrors the historical strings.Contains(rawContent, "\n")
	// guard: content already shaped by a block/br/table/list child (as
	// opposed to plain flowable text) never counts as "wrapped" below, even
	// when wordWrapTokens' result has multiple lines — those lines come from
	// forced structure, not width-driven reflow.
	hasStructure := false
	for _, tk := range tokens {
		if tk.brk || tk.box != nil {
			hasStructure = true
			break
		}
	}
	var b box
	var positions map[*html.Node]Rect
	if ws == "pre" || ws == "nowrap" {
		// tokensToString doesn't place anything, so no positions to track
		// here — an accepted gap for pre/nowrap content specifically.
		b = newBox(tokensToString(tokens))
	} else {
		breakMode := decls["overflow-wrap"]
		if breakMode == "" {
			breakMode = decls["word-break"]
		}
		b, positions = wordWrapTokens(tokens, innerW, breakMode, 0)
		wasWrapped = !hasStructure && len(b.lines) > 1
	}

	if (ovX == "hidden" || ovX == "clip") && hasExplicitWidth {
		toVal := decls["text-overflow"]
		if toVal == "" {
			toVal = "clip"
		}
		suffix := textOverflowSuffix(toVal)
		newLines := make([]string, len(b.lines))
		for i, ln := range b.lines {
			newLines[i] = truncateToWidth(ln, innerW, suffix)
		}
		b = box{lines: newLines, width: linesWidth(newLines)}
	}

	// closedBox is true when a top/bottom rule is combined with a right
	// border: the rule always spans the full box width, so the right
	// border must be pushed out to meet it rather than hugging content.
	closedBox := (bt.char != "" || bb.char != "") && br.char != ""
	needsAlign := textAlign != "" || closedBox || (hasExplicitWidth && ws != "nowrap")
	if needsAlign {
		b = alignLinesBox(b, textAlign, innerW)
	}

	if !needsAlign && wasWrapped && (pr > 0 || br.char != "") {
		b = padLinesToWidthBox(b, b.width)
	}

	minH := 0
	if v := decls["min-height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			minH = abs
		}
	}
	maxH := 0
	if v := decls["max-height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			maxH = abs
		}
	}
	if heightLines > 0 || minH > 0 || maxH > 0 {
		lines := b.lines
		blank := strings.Repeat(" ", innerW)
		if heightLines > 0 {
			// Fixed height takes priority over min/max.
			switch ovY {
			case "hidden", "clip":
				if len(lines) > heightLines {
					lines = lines[:heightLines]
				} else {
					for len(lines) < heightLines {
						lines = append(lines, blank)
					}
				}
			case "scroll", "auto":
				// A scrollable container's clip must also shift its
				// descendants' recorded positions by -offset (mirroring
				// mergePositions' existing shift-on-placement primitive), so
				// a scrolled-off descendant's Rect lands outside the visible
				// range instead of the pre-scroll range — kept, not deleted,
				// matching a scrolled-off real DOM element's
				// getBoundingClientRect(). r.liveScrollOffsets is this
				// frame's freshly rebuilt scroll-offset map (see
				// SCROLLING.md); r.scrollOffsets is nil for a plain
				// Renderer.Render call (no persistent Document to read a
				// prior offset from), so offset is simply 0 there.
				offset := r.scrollOffsets[n]
				totalLines := len(lines)
				maxOffset := max(0, totalLines-heightLines)
				offset = min(max(offset, 0), maxOffset)
				if r.liveScrollOffsets == nil {
					r.liveScrollOffsets = map[*html.Node]int{}
				}
				r.liveScrollOffsets[n] = offset
				if len(lines) > heightLines {
					lines = lines[offset : offset+heightLines]
					positions = mergePositions(nil, positions, -offset, 0)
				}
				for len(lines) < heightLines {
					lines = append(lines, blank)
				}
				// hasScrollbarGutter (not just ovY == "scroll") draws an
				// always-on gutter indicator, regardless of whether this
				// frame actually needed to slice — see SCROLLING.md's
				// "Scrollbar gutter and indicator" for why "auto"
				// deliberately gets none, and why a too-narrow box (the
				// gutter wasn't actually reserved in innerW) must not draw
				// one either, or content would get an unreserved column
				// appended on top of it instead of a properly narrowed box.
				if hasScrollbarGutter {
					lines = appendScrollbarColumn(lines, offset, totalLines, heightLines)
				}
			default:
				for len(lines) < heightLines {
					lines = append(lines, blank)
				}
			}
		} else {
			// max-height clips (requires overflow: hidden/clip).
			if maxH > 0 && len(lines) > maxH && (ovY == "hidden" || ovY == "clip") {
				lines = lines[:maxH]
			}
			// min-height always pads.
			if minH > 0 {
				for len(lines) < minH {
					lines = append(lines, blank)
				}
			}
		}
		b = box{lines: lines, width: linesWidth(lines)}
	}
	// text-indent: apply only when this element's first rendered content is
	// direct inline text (not a child block that will apply its own indent).
	if v := decls["text-indent"]; v != "" && r.firstContentIsInline(n) {
		indent := 0
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				indent = int(pct * float64(innerW))
			} else {
				indent = abs
			}
		}
		if indent > 0 && len(b.lines) > 0 {
			lines := b.lines
			lines[0] = strings.Repeat(" ", indent) + lines[0]
			b = box{lines: lines, width: linesWidth(lines)}
		}
	}
	if pt > 0 || pb > 0 {
		blank := strings.Repeat(" ", innerW)
		lines := b.lines
		if pt > 0 {
			padded := make([]string, 0, pt+len(lines))
			for range pt {
				padded = append(padded, blank)
			}
			lines = append(padded, lines...)
		}
		if pb > 0 {
			for range pb {
				lines = append(lines, blank)
			}
		}
		b = box{lines: lines, width: linesWidth(lines)}
	}
	if pl > 0 || pr > 0 {
		b = applyLineEdgesBox(b, strings.Repeat(" ", pl), strings.Repeat(" ", pr))
	}
	if bl.char != "" || br.char != "" {
		b = applyBlockBordersBox(b, bl, br, r.profile)
	}
	isEmpty := func() bool { return len(b.lines) == 1 && b.lines[0] == "" }
	topRuleDrawn := false
	if top := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth, r.profile); top != "" {
		if isEmpty() {
			b.lines = []string{top}
		} else {
			b.lines = append([]string{top}, b.lines...)
		}
		b.width = linesWidth(b.lines)
		topRuleDrawn = true
	}
	if bot := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth, r.profile); bot != "" {
		if isEmpty() {
			b.lines = []string{bot}
		} else {
			b.lines = append(b.lines, bot)
		}
		b.width = linesWidth(b.lines)
	}
	if ml > 0 || mr > 0 {
		b = applyLineEdgesBox(b, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
	}
	if decls["visibility"] == "hidden" {
		b = blankVisibleContentBox(b)
	}
	// Reset b.pre based on this element's own resolved white-space,
	// regardless of what children's boxes may have carried in: the old
	// cappedWriter model's EnterPre/ExitPre was scoped to exactly one
	// child's own content (inline.go's nested "block" case entered/exited
	// pre mode around writing just that child's block content into the
	// parent's writer) and never persisted past it — a <pre> nested inside
	// a non-pre ancestor loses its exemption the moment that ancestor's own
	// (non-pre) content reaches a writer instance that isn't in pre mode.
	// Crossing this function's own boundary is the box-model equivalent of
	// that per-element EnterPre/ExitPre scoping.
	if ws == "pre" || ws == "pre-wrap" {
		b.pre = make([]bool, len(b.lines))
		for i := range b.pre {
			b.pre[i] = true
		}
	} else {
		b.pre = nil
	}
	// Shift descendant positions (captured right after the initial wrap,
	// before any of the transformations above) by everything since applied
	// that moves rows/columns uniformly: pt prepends pt blank lines; a
	// drawn top rule prepends one more; pl and the left border character
	// each shift every line right; ml (baked in as literal padding, unlike
	// vertical margin) shifts right again. text-indent (row 0 only) and
	// text-align:right/center (variable per line) are NOT accounted for —
	// an accepted approximation, since the primary use (hit-testing form
	// controls) essentially never combines those with tracked descendants.
	rowShift := pt
	if topRuleDrawn {
		rowShift++
	}
	colShift := pl + runeLen(bl.char) + ml
	positions = mergePositions(nil, positions, rowShift, colShift)
	if heightLines > 0 && (ovY == "scroll" || ovY == "auto") {
		// Rect (assigned by whichever caller embeds this box as a token) is
		// the full CSS border box, which — unlike heightLines — includes any
		// border/padding rows added above; DispatchKey's PageUp/PageDown and
		// Focus's scrollIntoView both need the actual content-box viewport
		// height and the row offset from this box's own top to its first
		// visible content row, which only rowShift/heightLines here capture.
		if r.liveScrollViewport == nil {
			r.liveScrollViewport = map[*html.Node]scrollViewport{}
		}
		r.liveScrollViewport[n] = scrollViewport{height: heightLines, topOffset: rowShift}
	}
	return b, positions
}

// scrollbarGutterWidth is the fixed column width reserved for the scrollbar
// gutter when overflow-y:scroll is set — see SCROLLING.md's "Scrollbar
// gutter and indicator". Not CSS-configurable in this pass.
const scrollbarGutterWidth = 1

// scrollbarTrackChar/scrollbarThumbChar are the fixed glyphs drawn in the
// scrollbar gutter — not CSS-configurable in this pass (see SCROLLING.md's
// explicit non-goals for the scrollbar).
const (
	scrollbarTrackChar = "│"
	scrollbarThumbChar = "█"
)

// appendScrollbarColumn appends one scrollbar-gutter column to each of
// lines — already exactly heightLines rows of uniform width — using the
// standard proportional thumb-size/thumb-position formula. totalLines is
// the content's line count before it was sliced/padded to heightLines, so
// the thumb reflects the real scrollable range even though lines itself no
// longer does. Appends rather than overwrites, so real content is never
// clobbered — see SCROLLING.md's rejected splice-overlay alternative for
// why that matters. When totalLines <= heightLines (nothing to actually
// scroll), thumbSize naturally comes out to heightLines, i.e. the thumb
// fills the whole track, matching a real scrollbar's own convention for
// "you can already see everything."
func appendScrollbarColumn(lines []string, offset, totalLines, heightLines int) []string {
	thumbSize := heightLines
	if totalLines > heightLines {
		thumbSize = max(1, min(heightLines*heightLines/totalLines, heightLines))
	}
	thumbStart := 0
	if maxOffset := totalLines - heightLines; maxOffset > 0 {
		thumbStart = offset * (heightLines - thumbSize) / maxOffset
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		ch := scrollbarTrackChar
		if i >= thumbStart && i < thumbStart+thumbSize {
			ch = scrollbarThumbChar
		}
		out[i] = ln + ch
	}
	return out
}

// firstContentIsInline reports whether n's first non-whitespace content is
// inline (a text node or inline element). Returns false when the first child
// is a block-level element, meaning text-indent should not be applied here —
// the block child will apply its own inherited value on its own first line.
func (r *Renderer) firstContentIsInline(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			if strings.TrimSpace(c.Data) != "" {
				return true
			}
			continue
		}
		if c.Type == html.ElementNode {
			return r.resolveDecls(c)["display"] != "block"
		}
	}
	return false
}

// blankVisibleContent removes all ANSI escapes and replaces every non-newline
// character with a space, preserving line structure for layout purposes.
// Thin shim over blankVisibleContentBox for callers not yet migrated to box.
func blankVisibleContent(s string) string {
	return blankVisibleContentBox(newBox(s)).join()
}

// blankLineVisible strips ANSI from line and replaces every remaining rune
// with a space, preserving rune count.
func blankLineVisible(line string) string {
	return strings.Repeat(" ", utf8.RuneCountInString(stripANSI(line)))
}

// blankVisibleContentBox is blankVisibleContent's box-based core.
func blankVisibleContentBox(b box) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		lines[i] = blankLineVisible(line)
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// parseCSSString unquotes a CSS quoted string token (e.g. `"│"` → `│`).
// Returns "" for unquoted values, keywords, or empty input.
func parseCSSString(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return ""
	}
	if (v[0] != '"' || v[len(v)-1] != '"') && (v[0] != '\'' || v[len(v)-1] != '\'') {
		return ""
	}
	inner := v[1 : len(v)-1]
	if !strings.ContainsRune(inner, '\\') {
		return sanitizeTerminalText(inner, true)
	}
	// Decode CSS string escape sequences.
	var b strings.Builder
	runes := []rune(inner)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i+1 >= len(runes) {
			b.WriteRune(runes[i])
			continue
		}
		i++
		next := runes[i]
		// \<newline> — line continuation, consume and skip the newline
		if next == '\n' {
			continue
		}
		// \<hex>{1,6}<optional-space> — Unicode code point
		if isHexRune(next) {
			hexStart := i
			for i+1 < len(runes) && isHexRune(runes[i+1]) && i-hexStart < 5 {
				i++
			}
			cp, _ := strconv.ParseInt(string(runes[hexStart:i+1]), 16, 32)
			b.WriteRune(rune(cp))
			// consume optional single whitespace after hex escape
			if i+1 < len(runes) && isSpaceRune(runes[i+1]) {
				i++
			}
			continue
		}
		// \<other> — the character itself
		b.WriteRune(next)
	}
	return sanitizeTerminalText(b.String(), true)
}

// scanFnArgs returns the index of the ')' that closes the CSS function call
// whose arguments start at s[0], skipping over quoted strings. Returns -1 if
// no unquoted ')' is found.
func scanFnArgs(s string) int {
	for i := 0; i < len(s); {
		c := s[i]
		if c == ')' {
			return i
		}
		if c == '"' || c == '\'' {
			q := c
			i++
			for i < len(s) {
				if s[i] == '\\' {
					i++
					if i < len(s) {
						i++
					}
					continue
				}
				if s[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		i++
	}
	return -1
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

// parseCSSContentString extracts the text from a CSS content property value.
// Supports: quoted strings, attr(), counter(), counters(), open-quote,
// close-quote, no-open-quote, no-close-quote. Returns "" for none/normal.
func (r *Renderer) parseCSSContentString(v string, n *html.Node) string {
	v = strings.TrimSpace(v)
	if v == "none" || v == "normal" || v == "" {
		return ""
	}
	cs := r.counterMap[n]
	var b strings.Builder
	for v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			break
		}
		switch {
		case strings.HasPrefix(v, "attr("):
			end := scanFnArgs(v[5:])
			if end < 0 {
				return b.String()
			}
			attrName := strings.TrimSpace(v[5 : 5+end])
			v = v[5+end+1:]
			if n != nil {
				b.WriteString(sanitizeTerminalText(nodeAttr(n, attrName), true))
			}

		case strings.HasPrefix(v, "counters("):
			end := scanFnArgs(v[9:])
			if end < 0 {
				return b.String()
			}
			name, sep, style := parseCounterFnArgs(v[9 : 9+end])
			v = v[9+end+1:]
			vals := cs.values(name)
			parts := make([]string, len(vals))
			for i, val := range vals {
				parts[i] = formatCounterValue(val, style)
			}
			b.WriteString(strings.Join(parts, sep))

		case strings.HasPrefix(v, "counter("):
			end := scanFnArgs(v[8:])
			if end < 0 {
				return b.String()
			}
			name, _, style := parseCounterFnArgs(v[8 : 8+end])
			v = v[8+end+1:]
			b.WriteString(formatCounterValue(cs.value(name), style))

		case strings.HasPrefix(v, "no-open-quote"):
			v = v[len("no-open-quote"):]
			r.quoteDepth++

		case strings.HasPrefix(v, "no-close-quote"):
			v = v[len("no-close-quote"):]
			if r.quoteDepth > 0 {
				r.quoteDepth--
			}

		case strings.HasPrefix(v, "open-quote"):
			v = v[len("open-quote"):]
			pairs := parseQuotes(r.resolveDecls(n)["quotes"])
			depth := r.quoteDepth
			if depth >= len(pairs) {
				depth = len(pairs) - 1
			}
			b.WriteString(pairs[depth][0])
			r.quoteDepth++

		case strings.HasPrefix(v, "close-quote"):
			v = v[len("close-quote"):]
			if r.quoteDepth > 0 {
				r.quoteDepth--
			}
			pairs := parseQuotes(r.resolveDecls(n)["quotes"])
			depth := r.quoteDepth
			if depth >= len(pairs) {
				depth = len(pairs) - 1
			}
			b.WriteString(pairs[depth][1])

		case v[0] == '"' || v[0] == '\'':
			// quoted string — find the closing quote
			q := v[0]
			i := 1
			for i < len(v) {
				if v[i] == '\\' {
					i++
					if i < len(v) {
						i++
					}
					continue
				}
				if v[i] == q {
					i++
					break
				}
				i++
			}
			b.WriteString(parseCSSString(v[:i]))
			v = v[i:]

		default:
			// unrecognised token — skip one word
			i := strings.IndexAny(v, " \t\n\r")
			if i < 0 {
				return b.String()
			}
			v = v[i:]
		}
	}
	return b.String()
}

// wrapHyperlink wraps text in an OSC 8 terminal hyperlink sequence.
func (r *Renderer) wrapHyperlink(href, text string) string {
	href = sanitizeTerminalText(href, false)
	if href == "" || r.noOSC8Links || r.profile <= colorprofile.Ascii {
		return text
	}
	return ansi.SetHyperlink(href) + text + ansi.ResetHyperlink()
}

// wrapHyperlinkBox is wrapHyperlink's box-based equivalent, preserving b.pre
// (the string-signature version, joining and re-splitting, would silently
// drop it — box.join() has no pre-tagging concept). The OSC 8 sequences are
// zero-width and never contain "\n", so this never changes b's line count.
func (r *Renderer) wrapHyperlinkBox(href string, b box) box {
	href = sanitizeTerminalText(href, false)
	if href == "" || r.noOSC8Links || r.profile <= colorprofile.Ascii || len(b.lines) == 0 {
		return b
	}
	lines := append([]string(nil), b.lines...)
	lines[0] = ansi.SetHyperlink(href) + lines[0]
	lines[len(lines)-1] += ansi.ResetHyperlink()
	return box{lines: lines, width: linesWidth(lines), pre: b.pre}
}
