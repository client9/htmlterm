package render

import (
	"maps"
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

// resolveBoxBorders resolves the four border edges (glyph, color) and corner
// overrides for a block-level box from decls — shared by renderBlockContentBox
// and renderFlexContentBox (flex.go) so both box models pick up
// border-style/border-*-color/border-*-corner consistently.
func resolveBoxBorders(decls map[string]string) (bl, br, bt, bb blockBorder, tlCorner, trCorner, blCorner, brCorner string) {
	blChar, blPresent := resolveBorderEdgeChar(decls["border-left"], edgeGlyphLeft)
	brChar, brPresent := resolveBorderEdgeChar(decls["border-right"], edgeGlyphRight)
	btChar, btPresent := resolveBorderEdgeChar(decls["border-top"], edgeGlyphTop)
	bbChar, bbPresent := resolveBorderEdgeChar(decls["border-bottom"], edgeGlyphBottom)
	bl = blockBorder{char: blChar, color: decls["border-left-color"]}
	br = blockBorder{char: brChar, color: decls["border-right-color"]}
	bt = blockBorder{char: btChar, color: decls["border-top-color"]}
	bb = blockBorder{char: bbChar, color: decls["border-bottom-color"]}
	tlCorner = parseCSSString(decls["border-top-left-corner"])
	trCorner = parseCSSString(decls["border-top-right-corner"])
	blCorner = parseCSSString(decls["border-bottom-left-corner"])
	brCorner = parseCSSString(decls["border-bottom-right-corner"])
	if styleVal := decls["border-style"]; styleVal != "" {
		if ts, ok := namedTableStyle(styleVal); ok {
			if !blPresent {
				bl.char = ts.left
			}
			if !brPresent {
				br.char = ts.right
			}
			if ts.top != nil {
				if !btPresent {
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
				if !bbPresent {
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
	return bl, br, bt, bb, tlCorner, trCorner, blCorner, brCorner
}

// renderBlockContentBox renders the styled, bordered, and margined content of
// a block element. It preserves the exact operation order of the original
// string implementation (border
// resolution → margin/padding resolution → clampCellPadding → inline content
// → wrap → overflow/text-overflow → align → padLinesToWidth fallback →
// height padding → text-indent → vertical padding → horizontal padding →
// borders → top/bottom rules → margins → visibility:hidden blanking, last).
func (r *Engine) renderBlockContentBox(n *html.Node, decls map[string]string, availWidth int) (box, map[*html.Node]Rect) {
	if r.measuringNaturalWidth && availWidth > measureBlockWidthCap {
		availWidth = measureBlockWidthCap
	}
	bl, br, bt, bb, tlCorner, trCorner, blCorner, brCorner := resolveBoxBorders(decls)
	ml, mlAuto := resolveMarginSide(decls["margin-left"], availWidth)
	mr, mrAuto := resolveMarginSide(decls["margin-right"], availWidth)
	pl := parsePaddingLen(decls["padding-left"])
	pr := parsePaddingLen(decls["padding-right"])
	pt := parsePaddingLen(decls["padding-top"])
	pb := parsePaddingLen(decls["padding-bottom"])
	hBorderWidth := availWidth - ml - mr
	acc := extractInlineStyle(decls)
	textAlign := decls["text-align"]
	// ovX/ovY are overflow-x/overflow-y — see docs/SCROLLING.md's "Scrollbar
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
		// A width too small to fit this element's own border+padding is
		// clamped to a 1-column minimum rather than discarded — CSS itself
		// never lets border+padding shrink content below 0, so "too small
		// to fit" isn't a reason to fall back to full auto/shrink-wrap
		// sizing (which silently ignored the width declaration entirely).
		if inner < 1 {
			inner = 1
		}
		hBorderWidth = runeLen(bl.char) + pl + inner + pr + runeLen(br.char)
		hasExplicitWidth = true
	}
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		ml, mr = splitAutoMargins(remaining, ml, mr, mlAuto, mrAuto)
	}

	avail := hBorderWidth - runeLen(bl.char) - runeLen(br.char)
	// gutterWidth reserves a column for the scrollbar indicator up front,
	// before wrapping — see docs/SCROLLING.md's "Scrollbar gutter and indicator"
	// for why this must happen before wordWrapTokens runs (below), not as a
	// post-hoc overlay. Silently dropped (gutterWidth stays 0) if there
	// isn't room for it, rather than collapsing content to 0 width.
	gutterWidth := 0
	if heightLines > 0 && ovY == "scroll" {
		if w := r.scrollbarGutterWidth(n); avail-w >= 1 {
			gutterWidth = w
		}
	}
	hasScrollbarGutter := gutterWidth > 0
	// capStartDrawn/capEndDrawn are set (if hasScrollbarGutter) inside the
	// "scroll"/"auto" case below and read afterward at the
	// r.liveScrollViewport[n] = ... assignment — declared here, not with :=
	// in the case block, so they survive past that switch statement's scope.
	var capStartDrawn, capEndDrawn bool
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
		tokens = appendText(nil, newInlineStyle(), val, r.profile)
	} else {
		tokens = r.renderInlineAccTokens(n, acc, innerW)
	}
	// A leading child's margin-top (e.g. the first <h2> in a plain <div>) and
	// a trailing child's margin-bottom (e.g. the last one) show up here as
	// leading/trailing brk tokens. When this box's own top/bottom edge is
	// open (no border/padding on that side, no explicit height forcing its
	// own box size), real CSS collapses that margin through to this
	// element's own margin-top/margin-bottom - do the same here, stripping
	// those tokens (so they don't ALSO leave a blank line inside this box)
	// and widening (never narrowing) whatever margin this element already
	// has. When an edge is blocked (border/padding/height present), the
	// margin does NOT collapse through, but it must still take effect right
	// where it already is - as real blank lines inside the box, exactly like
	// any other block child's margin.
	//
	// Leading and trailing aren't symmetric here: a trailing brk run always
	// has this box's own real content immediately before it, so the first
	// one just closes that content's own line (no blank line of its own) and
	// every one after represents one real blank line - the existing mb+1
	// convention (wraptoken.go's ensureBreaks) already prices that in, so
	// leaving a blocked trailing run untouched already renders the right
	// number of blank lines with no further adjustment. A leading brk run
	// has nothing before it (it's the first thing in this box's own
	// content), so *every* leading brk - including the first - renders as
	// its own blank line; the first one is pushBoxDirect's mandatory
	// "start a fresh line" placeholder (meaningless when there's nothing to
	// separate from) and must always be dropped, blocked or not, or a
	// blocked margin-top would render one blank line too many.
	if leadingBrk := leadingBreaks(tokens); leadingBrk > 0 {
		tokens = tokens[1:]
		if marginLines := leadingBrk - 1; marginLines > 0 {
			if bt.char == "" && pt == 0 && heightLines == 0 {
				tokens = tokens[marginLines:]
				if marginLines > parseMargin(decls["margin-top"]) {
					decls["margin-top"] = strconv.Itoa(marginLines)
				}
			}
			// else: blocked - the remaining marginLines leading brk tokens
			// stay in tokens, already rendering exactly marginLines blank
			// lines.
		}
	}
	if bottomOpen := bb.char == "" && pb == 0 && heightLines == 0; bottomOpen {
		if trailingBrk := trailingBreaks(tokens); trailingBrk > 1 {
			tokens = tokens[:len(tokens)-trailingBrk]
			if collapsed := trailingBrk - 1; collapsed > parseMargin(decls["margin-bottom"]) {
				decls["margin-bottom"] = strconv.Itoa(collapsed)
			}
		}
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
				// docs/SCROLLING.md); r.scrollOffsets is nil for a plain
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
				// frame actually needed to slice — see docs/SCROLLING.md's
				// "Scrollbar gutter and indicator" for why "auto"
				// deliberately gets none, and why a too-narrow box (the
				// gutter wasn't actually reserved in innerW) must not draw
				// one either, or content would get an unreserved column
				// appended on top of it instead of a properly narrowed box.
				if hasScrollbarGutter {
					track := r.resolveScrollbarStyle(n, decls, "scrollbar-track")
					thumb := r.resolveScrollbarStyle(n, decls, "scrollbar-thumb")
					capStart, hasCapStart := r.resolveScrollbarCap(n, decls, "scrollbar-cap-start")
					capEnd, hasCapEnd := r.resolveScrollbarCap(n, decls, "scrollbar-cap-end")
					lines, capStartDrawn, capEndDrawn = appendScrollbarColumn(lines, offset, totalLines, heightLines, innerW, gutterWidth, track, thumb, capStart, capEnd, hasCapStart, hasCapEnd, r.profile)
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
	if r.liveContentOffsets == nil {
		r.liveContentOffsets = map[*html.Node]int{}
	}
	r.liveContentOffsets[n] = rowShift
	if heightLines > 0 && (ovY == "scroll" || ovY == "auto") {
		// Rect (assigned by whichever caller embeds this box as a token) is
		// the full CSS border box, which — unlike heightLines — includes any
		// border/padding rows added above; DispatchKey's PageUp/PageDown and
		// Focus's scrollIntoView both need the actual content-box viewport
		// height and the row offset from this box's own top to its first
		// visible content row, which only rowShift/heightLines here capture.
		if r.liveScrollViewport == nil {
			r.liveScrollViewport = map[*html.Node]Viewport{}
		}
		// GutterCol mirrors colShift's own reasoning just above (the offset
		// from this box's own Rect.Col — which anchors to the very start of
		// its composed lines, margin-left included, per Rect's own doc
		// comment — to where child content starts): the gutter sits
		// immediately after content, so it's colShift plus innerW. Only
		// meaningful (and only used by document.go's tryScrollCapClick)
		// when GutterWidth > 0.
		r.liveScrollViewport[n] = Viewport{
			Height:      heightLines,
			TopOffset:   rowShift,
			GutterCol:   colShift + innerW,
			GutterWidth: gutterWidth,
			CapStart:    capStartDrawn,
			CapEnd:      capEndDrawn,
		}
	}
	return b, positions
}

// scrollbarStyle is one resolved ::scrollbar-track or ::scrollbar-thumb: the
// glyph repeated across the gutter's width, plus its text style (color,
// background-color, font-weight — see resolveScrollbarStyle). Always build
// style via extractInlineStyle/newInlineStyle, never the zero inlineStyle{}
// — per inlineStyle's own doc comment, the zero value reads as opacity:0
// and silently blanks the glyph.
type scrollbarStyle struct {
	char  string
	style inlineStyle
}

// ScrollbarGutterWidth is the default column width reserved for the
// scrollbar gutter when overflow-y:scroll is set and no ::scrollbar width
// declaration overrides it — see docs/SCROLLING.md's "Scrollbar gutter and
// indicator" and docs/SCROLLBARS.md. Exported (via
// htmlterm.ScrollbarGutterWidth) so callers who pre-render content outside a
// scrollable Document/Renderer pass (e.g. to cache an expensive layout, then
// splice it into a live scrollable pane via Document.SetPreRendered) can
// reserve the same column up front — otherwise the pre-rendered content is
// wrapped 1 column wider than the live pane's actual content width once the
// gutter is reserved there, desyncing the two and producing exactly the
// scrollbar-off-by-one symptom this constant's existence is meant to
// prevent. A caller that also sets a custom ::scrollbar { width } on the
// live pane must account for that override itself — this constant only
// reflects the built-in default.
const ScrollbarGutterWidth = 1

// scrollbarPreset is one named scrollbar-style's baseline ::scrollbar-track/
// ::scrollbar-thumb declarations — the same shape pseudoElemDecls returns
// from a real stylesheet rule, so it can be merged with one identically.
type scrollbarPreset struct {
	track, thumb, capStart, capEnd map[string]string
}

// defaultScrollbarStyle is used when scrollbar-style is unset or names a
// preset that doesn't exist; it reproduces this feature's original,
// pre-scrollbar-style/pre-::scrollbar-track/thumb-defaults behavior exactly.
const defaultScrollbarStyle = "block"

// scrollbarPresets backs the scrollbar-style property: block|shaded|classic|ascii|line.
// Each preset supplies content (and, for classic, background-color) as a
// baseline that an element's own ::scrollbar-track/::scrollbar-thumb/
// ::scrollbar-cap-start/::scrollbar-cap-end rules still override
// property-by-property — see resolveScrollbarStyle/resolveScrollbarCap.
// capStart/capEnd make the cap buttons opt-out, not opt-in: every preset
// supplies an arrow glyph, and an element only goes without caps by
// explicitly setting content: none on the relevant pseudo-element (the same
// "none suppresses injection" convention ::before/::after already have) or
// by the gutter being too short to spare a row for one (see
// appendScrollbarColumn). classic's colors are a deliberately neutral gray
// pair (not CSS-configurable via the scrollbar-style keyword itself);
// override them with an explicit ::scrollbar-track/::scrollbar-thumb/
// ::scrollbar-cap-* rule instead of a new preset if a different palette is
// wanted.
var scrollbarPresets = map[string]scrollbarPreset{
	"block": {
		track:    map[string]string{"content": `"│"`},
		thumb:    map[string]string{"content": `"█"`},
		capStart: map[string]string{"content": `"▲"`},
		capEnd:   map[string]string{"content": `"▼"`},
	},
	"shaded": {
		track:    map[string]string{"content": `"░"`},
		thumb:    map[string]string{"content": `"█"`},
		capStart: map[string]string{"content": `"▲"`},
		capEnd:   map[string]string{"content": `"▼"`},
	},
	"classic": {
		track:    map[string]string{"content": `" "`, "background-color": "#444444"},
		thumb:    map[string]string{"content": `" "`, "background-color": "#aaaaaa"},
		capStart: map[string]string{"content": `"▲"`, "background-color": "#444444", "color": "#ffffff"},
		capEnd:   map[string]string{"content": `"▼"`, "background-color": "#444444", "color": "#ffffff"},
	},
	"ascii": {
		track:    map[string]string{"content": `"|"`},
		thumb:    map[string]string{"content": `"#"`},
		capStart: map[string]string{"content": `"^"`},
		capEnd:   map[string]string{"content": `"v"`},
	},
	"line": {
		track: map[string]string{"content": `"│"`},
		thumb: map[string]string{"content": `"┃"`},
		//thumb:    map[string]string{"content": `"▌"`},
		//thumb:    map[string]string{"content": `"║"`},
		capStart: map[string]string{"content": `"▲"`},
		capEnd:   map[string]string{"content": `"▼"`},
	},
}

// scrollbarGutterWidth resolves n's ::scrollbar { width } declaration (in
// ch/columns — see parseSizeVal), falling back to ScrollbarGutterWidth when
// unset, unparseable, or non-positive. Percentage widths are not meaningful
// for a gutter and are also treated as unset. Independent of scrollbar-style
// — none of the named presets set a width, only track/thumb glyph and color.
func (r *Engine) scrollbarGutterWidth(n *html.Node) int {
	decls := r.pseudoElemDecls(n, "scrollbar")
	if abs, pct, ok := parseSizeVal(decls["width"]); ok && pct == 0 && abs > 0 {
		return abs
	}
	return ScrollbarGutterWidth
}

// resolveScrollbarStyle resolves n's effective ::scrollbar-track or
// ::scrollbar-thumb style (which is "scrollbar-track" or "scrollbar-thumb")
// into a glyph plus text style. elemDecls is n's own resolved declarations
// (renderBlockContentBox already has this as decls — not re-resolved here),
// read only for scrollbar-style; it selects which scrollbarPresets entry
// supplies the baseline (falling back to defaultScrollbarStyle when unset or
// unrecognized). n's actual ::scrollbar-track/::scrollbar-thumb rule, if
// any, is then layered on top of that baseline property-by-property (a
// content/color/etc. the rule sets wins; anything the rule doesn't mention
// falls through to the preset) — this is what lets `scrollbar-style: classic`
// plus a lone `::scrollbar-thumb { color: red }` combine instead of one
// replacing the other outright.
func (r *Engine) resolveScrollbarStyle(n *html.Node, elemDecls map[string]string, which string) scrollbarStyle {
	preset, ok := scrollbarPresets[elemDecls["scrollbar-style"]]
	if !ok {
		preset = scrollbarPresets[defaultScrollbarStyle]
	}
	base := preset.track
	if which == "scrollbar-thumb" {
		base = preset.thumb
	}
	merged := make(map[string]string, len(base))
	maps.Copy(merged, base)
	maps.Copy(merged, r.pseudoElemDecls(n, which))
	ch := r.parseCSSContentString(merged["content"], n)
	return scrollbarStyle{char: ch, style: extractInlineStyle(merged)}
}

// resolveScrollbarCap resolves n's effective ::scrollbar-cap-start or
// ::scrollbar-cap-end style (which is "scrollbar-cap-start" or
// "scrollbar-cap-end") into a glyph plus text style, the same
// preset-baseline-plus-override merge resolveScrollbarStyle already does
// for ::scrollbar-track/::scrollbar-thumb (elemDecls is n's own resolved
// declarations, read only for scrollbar-style). Caps are opt-out, not
// opt-in: every scrollbarPresets entry supplies an arrow glyph for both
// ends, so ok is true unless an element's own ::scrollbar-cap-start/
// ::scrollbar-cap-end rule explicitly sets content: none/normal — the same
// "none suppresses injection" convention ::before/::after already have —
// or (handled by appendScrollbarColumn, not here) there's no room for the
// cap this frame.
func (r *Engine) resolveScrollbarCap(n *html.Node, elemDecls map[string]string, which string) (scrollbarStyle, bool) {
	preset, ok := scrollbarPresets[elemDecls["scrollbar-style"]]
	if !ok {
		preset = scrollbarPresets[defaultScrollbarStyle]
	}
	base := preset.capStart
	if which == "scrollbar-cap-end" {
		base = preset.capEnd
	}
	merged := make(map[string]string, len(base))
	maps.Copy(merged, base)
	maps.Copy(merged, r.pseudoElemDecls(n, which))
	// parseCSSContentString itself already treats "none"/"normal" as empty
	// (see its own doc comment), so an explicit ::scrollbar-cap-start/
	// ::scrollbar-cap-end { content: none; } rule overriding the preset's
	// own content here is what turns ch (and so ok) back to empty/false —
	// no separate check needed.
	ch := r.parseCSSContentString(merged["content"], n)
	if ch == "" {
		return scrollbarStyle{}, false
	}
	return scrollbarStyle{char: ch, style: extractInlineStyle(merged)}, true
}

// appendScrollbarColumn appends one scrollbar-gutter — gutterWidth columns
// wide, each column holding either track.char or thumb.char (styled per
// track.style/thumb.style) — to each of lines, using the standard
// proportional thumb-size/thumb-position formula. totalLines is the
// content's line count before it was sliced/padded to heightLines, so the
// thumb reflects the real scrollable range even though lines itself no
// longer does. Appends rather than overwrites, so real content is never
// clobbered — see docs/SCROLLING.md's rejected splice-overlay alternative
// for why that matters. When totalLines <= heightLines (nothing to actually
// scroll), thumbSize naturally comes out to heightLines, i.e. the thumb
// fills the whole track, matching a real scrollbar's own convention for
// "you can already see everything."
//
// innerW is the box's content width (excluding the gutter itself). Each line
// is padded or truncated to exactly innerW visible columns before the glyphs
// are appended — upstream width-normalization (alignLinesBox/padLinesToWidthBox)
// only runs conditionally, and even then never truncates a line that's
// already >= width (e.g. one holding an unbreakable overlong token), so
// without this the gutter column would land on a ragged, content-dependent
// column instead of a straight line at the pane's right edge.
//
// capStart/capEnd are the resolved ::scrollbar-cap-start/::scrollbar-cap-end
// glyphs (see resolveScrollbarCap), each gated by its own hasCapStart/
// hasCapEnd — caps are opt-out (on by default via scrollbarPresets), so
// either can still be individually false when a rule explicitly disabled it
// (content: none). When active,
// a cap claims row 0 (start) and/or the last row (end) verbatim, and the
// thumb-size/thumb-position formula runs over the interior track
// (heightLines minus however many caps are active) so the thumb never
// overlaps a cap. If there isn't at least 1 interior row left once active
// caps are subtracted, both caps are silently dropped for this render (not
// just the one that doesn't fit) — the same "drop the added chrome, keep
// content correct" precedent already used when the gutter itself doesn't
// fit. The returned bools report which caps were actually drawn, for the
// caller to record in Viewport (document.go's click hit-testing must not
// treat a dropped cap as clickable).
func appendScrollbarColumn(lines []string, offset, totalLines, heightLines, innerW, gutterWidth int, track, thumb, capStart, capEnd scrollbarStyle, hasCapStart, hasCapEnd bool, profile colorprofile.Profile) ([]string, bool, bool) {
	activeCaps := 0
	if hasCapStart {
		activeCaps++
	}
	if hasCapEnd {
		activeCaps++
	}
	if activeCaps > 0 && heightLines-activeCaps < 1 {
		hasCapStart, hasCapEnd = false, false
		activeCaps = 0
	}
	interior := heightLines - activeCaps

	thumbSize := interior
	if totalLines > heightLines {
		thumbSize = max(1, min(interior*interior/totalLines, interior))
	}
	thumbStart := 0
	if maxOffset := totalLines - heightLines; maxOffset > 0 {
		thumbStart = offset * (interior - thumbSize) / maxOffset
	}

	capOffset := 0
	if hasCapStart {
		capOffset = 1
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		var g scrollbarStyle
		switch {
		case hasCapStart && i == 0:
			g = capStart
		case hasCapEnd && i == heightLines-1:
			g = capEnd
		default:
			g = track
			if interiorIdx := i - capOffset; interiorIdx >= thumbStart && interiorIdx < thumbStart+thumbSize {
				g = thumb
			}
		}
		ln = truncateToWidth(ln, innerW, "")
		if pad := innerW - ansiVisibleLen(ln); pad > 0 {
			ln += strings.Repeat(" ", pad)
		}
		out[i] = ln + g.style.render(strings.Repeat(g.char, gutterWidth), profile)
	}
	return out, hasCapStart, hasCapEnd
}

// firstContentIsInline reports whether n's first non-whitespace content is
// inline (a text node or inline element). Returns false when the first child
// is a block-level element, meaning text-indent should not be applied here —
// the block child will apply its own inherited value on its own first line.
func (r *Engine) firstContentIsInline(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			if strings.TrimSpace(c.Data) != "" {
				return true
			}
			continue
		}
		if c.Type == html.ElementNode {
			display := r.resolveDecls(c)["display"]
			return display != "block" && display != "flex"
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

// isQuotedCSSValue reports whether v (after trimming) is a CSS quoted
// string token - the disambiguator resolveBorderEdgeChar uses between a
// literal border glyph (border-top: "═") and the standard border-edge
// shorthand grammar (border-top: solid red).
func isQuotedCSSValue(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return false
	}
	return (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')
}

// resolveBorderEdgeChar parses one of border-top/border-right/border-bottom/
// border-left's raw declared value. A quoted string is this engine's
// literal-glyph form and is unquoted via parseCSSString unchanged. Anything
// else has already passed through expandShorthand (css.go) as the standard
// CSS border-edge shorthand grammar, which leaves just a bare style keyword
// here - its width and color tokens, if any, were already split into
// border-*-color there - so glyph picks that preset's character for this
// specific edge (e.g. top.fill for border-top, left for border-left).
// present reports whether the declaration existed at all, even when it
// resolves to an empty character (an explicit "none"/"hidden" style) -
// callers must not let the border-style backfill override an edge that was
// deliberately cleared.
func resolveBorderEdgeChar(raw string, glyph func(tableStyle) string) (char string, present bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if isQuotedCSSValue(raw) {
		return parseCSSString(raw), true
	}
	if ts, ok := namedTableStyle(raw); ok {
		return glyph(ts), true
	}
	return "", false
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
func (r *Engine) parseCSSContentString(v string, n *html.Node) string {
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
func (r *Engine) wrapHyperlink(href, text string) string {
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
//
// Every line gets its own open+close pair, not just line 0/the last line:
// the terminal-facing consumer of this output (../tui/cellbridge.go's
// writeANSILine) decodes each screen row independently from a fresh state,
// the same way it re-derives SGR style per row rather than carrying it
// across rows — an open only on line 0 left every wrapped continuation
// line of a multi-line block/flex <a> (common for HTML-email "read
// more"/CTA buttons) with no URL attached to its cells at all: still
// underlined (SGR is correctly self-contained per line), but not
// clickable, and if line 0 itself scrolled out of view, no visible row of
// the link was clickable.
func (r *Engine) wrapHyperlinkBox(href string, b box) box {
	href = sanitizeTerminalText(href, false)
	if href == "" || r.noOSC8Links || r.profile <= colorprofile.Ascii || len(b.lines) == 0 {
		return b
	}
	lines := append([]string(nil), b.lines...)
	open := ansi.SetHyperlink(href)
	closeSeq := ansi.ResetHyperlink()
	for i, line := range lines {
		lines[i] = open + line + closeSeq
	}
	return box{lines: lines, width: linesWidth(lines), pre: b.pre}
}
