package render

import (
	"maps"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/client9/htmlterm/internal/textcell"
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
// per-line transform and restored after. Content has no trailing-newline
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

// alignLinesBox pads every line of b to width, aligning left, right, or
// center per dir. Lines already at or past width are left unchanged.
func alignLinesBox(b box, dir string, width int) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		vl := textcell.VisibleLen(line)
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

// alignLines is a string-signature shim over alignLinesBox. See applyLineEdges
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
// spaces. Lines already at or past width are left unchanged.
func padLinesToWidthBox(b box, width int) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		if vl := textcell.VisibleLen(line); vl < width {
			lines[i] = line + strings.Repeat(" ", width-vl)
		} else {
			lines[i] = line
		}
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// padLinesToWidth is a string-signature shim over padLinesToWidthBox. See
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

// drawBlockHBorder draws a block box's own top or bottom border rule: an
// optional corner glyph at each end, and fill tiled across whatever width
// remains. An unset corner stays empty rather than falling back to fill: for
// a single-character fill the two look identical (the fill glyph reaches the
// very end either way), but falling back would print fill's *entire* string
// as a second corner for a multi-character fill like `border-top: "=-"`,
// double-counting it at both ends instead of tiling it seamlessly through.
func drawBlockHBorder(fill, color, leftCorner, rightCorner string, width int, p colorprofile.Profile) string {
	if fill == "" || fill == "none" || width <= 0 {
		return ""
	}
	segWidth := width - textcell.Width(leftCorner) - textcell.Width(rightCorner)
	return drawHRule([]int{max(0, segWidth)}, fill, color, leftCorner, "", rightCorner, p)
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

// resolveCSSHeight is resolveCSSSize for a vertical size, whose basis can be
// indefinite in a way a width's never is. cbHeight is the containing block's
// definite content height in rows, or indefiniteMainSize when there is none.
//
// A percentage against an indefinite basis is not a length. CSS resolves it to
// auto for a preferred size and none for a maximum, so reporting false and
// leaving the caller on its own unset path is exactly that rule. Without the
// guard a percentage would multiply against indefiniteMainSize's -1 and come
// back negative, and against a plain 0 it would come back as a definite zero
// rows, which is a real height and the wrong answer. This is the same
// distinction resolveFlexCSSSize draws on the flex main axis.
func resolveCSSHeight(s string, cbHeight int) (int, bool) {
	abs, pct, ok := parseSizeVal(s)
	if !ok {
		return 0, false
	}
	if pct > 0 {
		if cbHeight <= 0 {
			return 0, false
		}
		return int(pct * float64(cbHeight)), true
	}
	return abs, true
}

// isBorderBox reports whether decls resolves box-sizing to border-box. Any
// other value, including unset, is content-box, matching real CSS's own
// initial value. border-box is not a Go-level default anywhere in this
// package; it only ever comes from the cascade, typically the UA
// stylesheet's own `*, ::before, ::after { box-sizing: border-box; }` rule
// (defaultstylesheet.go), the same reset Bootstrap's Reboot and
// modern-normalize ship, moved down a cascade layer. See
// docs/proposals/BOX_SIZING.md.
func isBorderBox(decls map[string]string) bool {
	return decls["box-sizing"] == "border-box"
}

// verticalChrome is how many rows of a box's total painted height are its
// own chrome rather than its content: a drawn top/bottom border rule plus
// top/bottom padding. It is blockVerticalChrome's arithmetic core, taking
// the caller's already-resolved border edges and padding directly rather
// than resolving them from decls itself. blockVerticalChrome (flex.go) is
// the decls-based convenience wrapper most callers use; this version exists
// for a caller like renderBlockContentBox that already has bt/bb/pt/pb in
// scope from resolving its own borders and padding, and would otherwise pay
// for resolveBoxBorders again for every height/min-height/max-height
// conversion in a single render.
func verticalChrome(bt, bb blockBorder, pt, pb int) int {
	rows := pt + pb
	if bt.char != "" {
		rows++
	}
	if bb.char != "" {
		rows++
	}
	return rows
}

// borderBoxContentHeight converts a border-box height total to a content-box
// line count, by subtracting decls' own vertical chrome (blockVerticalChrome:
// top/bottom border rows and vertical padding), floored at 1 content line for
// the same reason resolveWidthConstraints floors a too-small border-box
// width: CSS never lets border and padding shrink content below 0, so "too
// small to fit" isn't a reason to fall back to auto sizing. Callers only need
// this when isBorderBox(decls) is true; a content-box height needs no
// conversion at all, since the declared value already is the content height.
//
// Shared by renderBlockContentBox (block.go) and resolveFlexContainerHeight/
// resolveFlexContainerHeightFloor (flex.go), so a flex container's own
// height, not just an ordinary block's, converts the same way. Without that
// second use, a grown nested flex container would overshoot its allotment by
// its own chrome: flex.go's renderFlexItemBoxSized already injects a
// container's synthesized height as a raw border-box total for the common
// border-box-by-default case (see its own doc comment), trusting whichever
// renderer reads decls["height"] next to subtract that chrome itself.
func borderBoxContentHeight(decls map[string]string, h int) int {
	h -= blockVerticalChrome(decls)
	if h < 1 {
		h = 1
	}
	return h
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

// controlContentDecls narrows a synthesized form control's own width
// declaration to the content box its block layout already resolved, for the
// block-level path in renderBlockContentBox. The control's synthesized text,
// meaning renderInput's padInputText and <progress>/<meter>'s bar, sizes
// itself from resolveWidthConstraints, and the width property is an *outer*
// size everywhere in this engine. Without this, a bordered `width: 20` input
// would paint 20 columns of field inside a 20-column box and overflow by its
// own border and padding. innerW is what's actually left for content.
//
// Only applied when the box's width really was constrained. An unconstrained
// control must keep falling back to its own intrinsic size, an <input>'s
// "size" attribute or the bar's default width, which is what a natural-width
// measurement has to see, a flex item's basis among others. Handing it
// innerW there would report whatever width it happened to be offered.
func controlContentDecls(decls map[string]string, innerW, availWidth int) map[string]string {
	if _, constrained := resolveWidthConstraints(decls, availWidth, 0); !constrained {
		return decls
	}
	out := maps.Clone(decls)
	out["width"] = strconv.Itoa(max(0, innerW))
	// The min/max pair has already had its say, being part of what resolved
	// innerW. Leaving it in place would re-clamp the content box against
	// bounds meant for the outer one.
	delete(out, "min-width")
	delete(out, "max-width")
	return out
}

func maxVisibleLineWidth(s string) int {
	maxW := 0
	for _, line := range strings.Split(s, "\n") {
		if w := textcell.VisibleLen(line); w > maxW {
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
// It accepts only a bare integer: a percentage or a "ch" suffix falls back to
// 0, the same as any other unparseable value, since none of the three parse
// as a plain Go int. margin-top/margin-bottom themselves go through
// resolveVerticalMargin instead, which shares parseSizeVal's full length
// vocabulary rather than this narrower one. This narrower parser survives
// only because list.go also calls it for padding-top/padding-bottom, which
// have never accepted a percentage or "ch" here and are out of this fix's
// scope.
func parseMargin(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// resolveVerticalMargin resolves a CSS margin-top/margin-bottom value as a
// line count, through the same parseSizeVal vocabulary every other length in
// this engine uses: a bare integer, a "ch" suffix (identical to the bare
// integer), or a percentage. parseMargin, used before this existed, only
// ever recognized the first, so `margin-top: 2ch` and `margin-top: 50%` both
// silently resolved to 0.
//
// basis is the containing block's own content WIDTH, not its height. That is
// real CSS's own rule for margin-top/margin-bottom (CSS 2.1 §8.3, unchanged
// in the current Box Model spec): despite widening a box vertically, a
// vertical margin's percentage resolves against the containing block's
// inline size, the same basis padding-top/padding-bottom and the horizontal
// margins already use. It is not a rule this engine invented to dodge an
// indefinite height; it is what a browser does. Unlike a percentage height,
// this needs no indefinite-basis guard: every box in this engine has a
// definite width, the terminal's own column count bounding the whole tree,
// so basis is always a real number here.
//
// "auto" and any other unparseable value resolve to 0, matching
// margin-top/margin-bottom's own initial value: outside a flex or grid
// item, "auto" has no defined resolution on this axis and simply computes to
// 0, the same fallback parseMargin already gave every invalid value.
func resolveVerticalMargin(s string, basis int) int {
	abs, pct, ok := parseSizeVal(s)
	if !ok {
		return 0
	}
	if pct > 0 {
		return int(pct * float64(basis))
	}
	return abs
}

// resolveBoxBorders resolves the four border edges, as glyph and color, and
// the corner overrides for a block-level box from decls. It is shared by
// renderBlockContentBox and renderFlexContentBox (flex.go) so both box models
// pick up border-style, border-*-color, and border-*-corner consistently.
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
// string implementation: border resolution → margin/padding resolution →
// clampCellPadding → inline content → wrap → overflow/text-overflow → align →
// padLinesToWidth fallback → height padding → text-indent → vertical padding
// → horizontal padding → borders → top/bottom rules → margins →
// visibility:hidden blanking, last.
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
	// ovX and ovY are overflow-x and overflow-y. See docs/SCROLLING.md's
	// "Scrollbar gutter and indicator". expandShorthand (css.go) already
	// expands a plain overflow:<val> shorthand into both, so these are correct
	// even when only the shorthand was ever set, and a more specific
	// overflow-x or overflow-y declaration overrides just that axis via the
	// normal per-property cascade (cascade.go's directDecls). ovX gates the
	// width-truncation check below; ovY gates the height and scroll gate and
	// the gutter reservation.
	ovX := decls["overflow-x"]
	ovY := decls["overflow-y"]
	// toContentLines converts a resolved height/min-height/max-height value
	// to a content-box line count. content-box (real CSS's initial value)
	// needs no conversion: the declared value already is the content height,
	// with border rows and vertical padding added on top further down.
	// border-box does, subtracting the same vertical chrome
	// borderBoxContentHeight computes, floored at 1 for the same reason a
	// too-small border-box width floors. Computed directly from bt/bb/pt/pb,
	// already resolved above, rather than through borderBoxContentHeight's
	// own decls-based lookup: height, min-height, and max-height can each
	// call this, and re-resolving this box's own borders up to three times
	// in one render for a value that never changes would be wasted work.
	vChrome := verticalChrome(bt, bb, pt, pb)
	borderBox := isBorderBox(decls)
	toContentLines := func(h int) int {
		if !borderBox {
			return h
		}
		h -= vChrome
		if h < 1 {
			h = 1
		}
		return h
	}
	heightLines := 0
	if v := decls["height"]; v != "" {
		if h, ok := resolveCSSHeight(v, r.cbHeight); ok && h > 0 {
			heightLines = toContentLines(h)
		}
	}
	// This box's own content box is the containing block its descendants
	// resolve their percentage heights against, and it is definite only when
	// this box declared a height. Everything below renders content, directly
	// or through the inline walker, so the swap has to happen before any of it
	// and be restored afterward: a sibling rendered next is back under the
	// original containing block, not this one's. See Engine.cbHeight.
	outerCBHeight := r.cbHeight
	if heightLines > 0 {
		r.cbHeight = heightLines
	} else {
		r.cbHeight = indefiniteMainSize
	}
	defer func() { r.cbHeight = outerCBHeight }()

	hasExplicitWidth := false
	if totalW, constrained := resolveWidthConstraints(decls, availWidth, availWidth); constrained {
		var inner int
		if isBorderBox(decls) {
			// border-box (the UA default): totalW is the box's whole outer
			// width, margins included, so border and padding come out of it.
			inner = totalW - ml - textcell.Width(bl.char) - pl - pr - textcell.Width(br.char) - mr
		} else {
			// content-box (real CSS's initial value, reached only by an
			// explicit override): totalW is the margin box, same as border-box,
			// since margin-subtraction predates box-sizing and is independent of
			// it (see COMPATIBILITY.md and ARCHITECTURE.md's "width: 100%" block
			// border box model invariant); only border and padding are excluded
			// from totalW here rather than subtracted out of it. So this box's
			// outer width, border and padding included, can legitimately exceed
			// availWidth once margins are accounted for. That overflow is left
			// alone here, the same as any other box whose unbreakable content
			// exceeds its width: it paints past its own border rather than being
			// clipped implicitly. See docs/proposals/BOX_SIZING.md.
			inner = totalW - ml - mr
		}
		// A width too small to fit this element's own border and padding is
		// clamped to a 1-column minimum rather than discarded. CSS itself
		// never lets border and padding shrink content below 0, so "too small
		// to fit" isn't a reason to fall back to full auto or shrink-wrap
		// sizing, which silently ignored the width declaration entirely.
		if inner < 1 {
			inner = 1
		}
		hBorderWidth = textcell.Width(bl.char) + pl + inner + pr + textcell.Width(br.char)
		hasExplicitWidth = true
	}
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		ml, mr = splitAutoMargins(remaining, ml, mr, mlAuto, mrAuto)
	}

	avail := hBorderWidth - textcell.Width(bl.char) - textcell.Width(br.char)
	// gutterWidth reserves a column for the scrollbar indicator up front,
	// before wrapping. See docs/SCROLLING.md's "Scrollbar gutter and indicator"
	// for why this must happen before wordWrapTokens runs below, not as a
	// post-hoc overlay. It is silently dropped, leaving gutterWidth at 0, if
	// there isn't room for it, rather than collapsing content to 0 width.
	gutterWidth := 0
	if heightLines > 0 && ovY == "scroll" {
		if w := r.scrollbarGutterWidth(n, decls); avail-w >= 1 {
			gutterWidth = w
		}
	}
	hasScrollbarGutter := gutterWidth > 0
	// capStartDrawn and capEndDrawn are set, when hasScrollbarGutter, inside
	// the "scroll"/"auto" case below and read afterward at the
	// r.liveScrollViewport[n] = ... assignment. They are declared here, not
	// with := in the case block, so they survive that switch statement's scope.
	var capStartDrawn, capEndDrawn bool
	var innerW int
	pl, pr, innerW = clampCellPadding(avail-gutterWidth, pl, pr)
	if innerW < 1 {
		innerW = 1
	}
	var tokens []wrapToken
	if n.Data == "textarea" {
		// <textarea>'s current value is its "value" attribute in this
		// package's simplified form-control model, matching
		// Element.Value and SetValue, which always read and write that
		// attribute for every control. But real HTML's default value for a
		// never-touched textarea is its child text, with one leading
		// newline right after the opening tag ignored per spec, so fall
		// back to that when no value attribute has been set yet.
		// appendText already knows how to split the embedded "\n"s of a
		// multi-line value into brk tokens, so the rest of this function's
		// wrap, border, and padding handling applies exactly as it would to
		// any other block's content.
		val := nodeAttr(n, "value")
		if val == "" {
			val = strings.TrimPrefix(rawContent(n), "\n")
		}
		if start, end, has := r.selectionRange(n); has {
			// Splice in the ::selection highlight (see
			// docs/proposals/CARET_SELECTION.md) by tokenizing the
			// before, selected, and after spans separately, at the
			// [start, end) rune offsets into the whole, possibly multi-line
			// value. appendText already knows how to split embedded "\n"s
			// within each span into brk tokens, same as the single-call
			// form below.
			runes := []rune(val)
			start = min(max(start, 0), len(runes))
			end = min(max(end, 0), len(runes))
			selStyle := r.resolveSelectionStyle(n, decls)
			tokens = appendText(tokens, newInlineStyle(), string(runes[:start]), r.profile)
			tokens = appendText(tokens, selStyle, string(runes[start:end]), r.profile)
			tokens = appendText(tokens, newInlineStyle(), string(runes[end:]), r.profile)
		} else {
			tokens = appendText(nil, newInlineStyle(), val, r.profile)
		}
	} else if text, synthesized := r.synthesizedControlText(n, controlContentDecls(decls, innerW, availWidth), acc, innerW); synthesized {
		// <input>, <select>, <progress>, and <meter> as a block-level box.
		// Their content comes from their attributes, not from child nodes,
		// having none worth walking, so renderInlineAccTokens would produce an
		// empty box. Reached whenever such a control is blockified: a flex item
		// always is, per Flexbox §4, and author CSS can do it directly with
		// `display: block`. The text arrives already styled, acc being applied
		// inside, so it's appended under the neutral style, same as
		// <textarea>'s value above.
		tokens = appendText(nil, newInlineStyle(), text, r.profile)
	} else {
		tokens = r.renderInlineAccTokens(n, acc, innerW)
	}
	// A leading child's margin-top, such as the first <h2> in a plain <div>,
	// and a trailing child's margin-bottom, such as the last one, show up here
	// as leading and trailing brk tokens. When this box's own top or bottom
	// edge is open, meaning no border or padding on that side and no explicit
	// height forcing its own box size, real CSS collapses that margin through
	// to this element's own margin-top or margin-bottom. Do the same here,
	// stripping those tokens so they don't ALSO leave a blank line inside this
	// box, and widening, never narrowing, whatever margin this element already
	// has. When an edge is blocked by a border, padding, or height, the margin
	// does NOT collapse through, but it must still take effect right where it
	// already is, as real blank lines inside the box, exactly like any other
	// block child's margin.
	//
	// Leading and trailing aren't symmetric here. A trailing brk run always
	// has this box's own real content immediately before it, so the first
	// one closes that content's own line, contributing no blank line of its
	// own, and every one after represents one real blank line. The existing
	// mb+1 convention (wraptoken.go's ensureBreaks) already prices that in, so
	// leaving a blocked trailing run untouched already renders the right
	// number of blank lines with no further adjustment. A leading brk run
	// has nothing before it, being the first thing in this box's own content,
	// so *every* leading brk, including the first, renders as its own blank
	// line. The first one is pushBoxDirect's mandatory "start a fresh line"
	// placeholder, meaningless when there's nothing to separate from, and must
	// always be dropped, blocked or not, or a blocked margin-top would render
	// one blank line too many.
	if leadingBrk := leadingBreaks(tokens); leadingBrk > 0 {
		tokens = tokens[1:]
		if marginLines := leadingBrk - 1; marginLines > 0 {
			if bt.char == "" && pt == 0 && heightLines == 0 {
				tokens = tokens[marginLines:]
				if marginLines > resolveVerticalMargin(decls["margin-top"], availWidth) {
					decls["margin-top"] = strconv.Itoa(marginLines)
				}
			}
			// else: blocked. The remaining marginLines leading brk tokens
			// stay in tokens, already rendering exactly marginLines blank
			// lines.
		}
	}
	if bottomOpen := bb.char == "" && pb == 0 && heightLines == 0; bottomOpen {
		if trailingBrk := trailingBreaks(tokens); trailingBrk > 1 {
			tokens = tokens[:len(tokens)-trailingBrk]
			if collapsed := trailingBrk - 1; collapsed > resolveVerticalMargin(decls["margin-bottom"], availWidth) {
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
			stripped := textcell.Strip(t)
			if strings.HasSuffix(stripped, " ") && !strings.HasSuffix(stripped, "  ") {
				if trimmed, ok := textcell.TrimTrailingSpace(t); ok {
					tokens[last] = wrapToken{text: trimmed}
				}
			}
		}
	}
	// hasStructure mirrors the historical strings.Contains(rawContent, "\n")
	// guard: content already shaped by a block, br, table, or list child,
	// as opposed to plain flowable text, never counts as "wrapped" below, even
	// when wordWrapTokens' result has multiple lines. Those lines come from
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
		// tokensToString doesn't place anything, so there are no positions
		// to track here. An accepted gap for pre and nowrap content
		// specifically.
		b = newBox(tokensToString(tokens))
	} else {
		breakMode := decls["overflow-wrap"]
		if breakMode == "" {
			breakMode = decls["word-break"]
		}
		b, positions = wordWrapTokens(tokens, innerW, breakMode, 0, ws == "pre" || ws == "pre-wrap")
		wasWrapped = !hasStructure && len(b.lines) > 1
	}

	// gutterHeightX, gutterRowX, capStartXDrawn, and capEndXDrawn are declared
	// here, not with := in the case below, so they survive this switch's
	// scope, for the ViewportX recording near the end of this function. Same
	// reasoning as hasScrollbarGutter, capStartDrawn, and capEndDrawn above.
	var gutterHeightX, gutterRowX int
	var capStartXDrawn, capEndXDrawn bool
	if hasExplicitWidth {
		switch ovX {
		case "hidden", "clip":
			suffix := textOverflowSuffix(decls["text-overflow"])
			newLines := make([]string, len(b.lines))
			for i, ln := range b.lines {
				newLines[i] = textcell.TruncateToWidth(ln, innerW, suffix)
			}
			b = box{lines: newLines, width: linesWidth(newLines)}
		case "scroll", "auto":
			// See applyHorizontalScroll's own doc comment for the offset
			// clamp, windowing, position-shift, and gutter-row behavior this
			// delegates to; it's shared with renderFlexContentBox.
			gutterRowX = len(b.lines)
			var lines []string
			lines, positions, gutterHeightX, capStartXDrawn, capEndXDrawn = r.applyHorizontalScroll(n, decls, b.lines, positions, innerW, b.width, heightLines, ovX)
			b = box{lines: lines, width: linesWidth(lines)}
		}
	}

	// Under a shrink-to-fit measurement render (see Engine.shrinkToFit),
	// this box reports its own content width instead of filling the width it
	// was handed. Applied here, after wrapping and overflow handling, and
	// only when no explicit width was declared, since an explicit width is the
	// answer rather than something to measure. Every step below — align
	// padding, height and padding blank lines, the top and bottom rules —
	// then sizes itself from the narrowed innerW and hBorderWidth, so the
	// whole box shrinks together instead of content hugging inside a
	// full-width border.
	if r.shrinkToFit && !hasExplicitWidth && b.width < innerW {
		innerW = max(1, b.width)
		hBorderWidth = textcell.Width(bl.char) + pl + innerW + gutterWidth + pr + textcell.Width(br.char)
	}

	// closedBox is true when a top or bottom rule is combined with a right
	// border. The rule always spans the full box width, so the right
	// border must be pushed out to meet it rather than hugging content.
	closedBox := (bt.char != "" || bb.char != "") && br.char != ""
	needsAlign := textAlign != "" || closedBox || (hasExplicitWidth && ws != "nowrap")
	// Captured before anything decorates the box. Every step below — aligning
	// padding to innerW, horizontal padding, the left and right border
	// characters — turns an empty content line into a line of real characters,
	// so this is the last point at which "did this element produce any content
	// at all" can still be asked. See the contentEmpty block further down for
	// what it's for.
	//
	// <textarea> is exempt. Its box is a field the user types into, sized in real
	// HTML by its `rows` attribute, default 2, rather than by its content, so an
	// empty one is an ordinary state of a widget that still occupies space, not
	// an empty box. This engine doesn't implement `rows`, so the single line it
	// already has is the whole of that reservation, and giving it up would leave
	// a focused, editable box with nowhere to show the caret.
	contentEmpty := n.Data != "textarea" && len(b.lines) == 1 && b.lines[0] == ""

	if needsAlign {
		b = alignLinesBox(b, textAlign, innerW)
	}

	if !needsAlign && wasWrapped && (pr > 0 || br.char != "") {
		b = padLinesToWidthBox(b, b.width)
	}

	// Resolved against the *outer* containing block, not against r.cbHeight,
	// which the block above has already swapped to this box's own content
	// height. A percentage min-height/max-height on this box is a fraction of
	// its parent's height, the same basis its percentage height used.
	minH := 0
	if v := decls["min-height"]; v != "" {
		if h, ok := resolveCSSHeight(v, outerCBHeight); ok && h > 0 {
			minH = toContentLines(h)
		}
	}
	maxH := 0
	if v := decls["max-height"]; v != "" {
		if h, ok := resolveCSSHeight(v, outerCBHeight); ok && h > 0 {
			maxH = toContentLines(h)
		}
	}
	if heightLines > 0 || minH > 0 || maxH > 0 {
		lines := b.lines
		blank := strings.Repeat(" ", innerW)
		// An empty box's one line is "", not innerW spaces, unless needsAlign
		// already padded it above. Every row this block appends below is a
		// full-width blank, so the pre-existing row needs the same width or
		// the box comes out with one short line among otherwise-full ones
		// (visible as a ragged left edge under a solid background or a
		// closing border rule two rows down).
		if contentEmpty {
			lines = []string{blank}
		}
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
				// See applyVerticalScroll's own doc comment for the offset
				// clamp, position-shift, padding, and gutter-draw behavior
				// this delegates to; it's shared with renderFlexContentBox.
				lines, positions, capStartDrawn, capEndDrawn = r.applyVerticalScroll(n, decls, lines, positions, heightLines, innerW, gutterWidth, hasScrollbarGutter)
			default:
				for len(lines) < heightLines {
					lines = append(lines, blank)
				}
			}
		} else {
			// max-height clips, which requires overflow: hidden or clip.
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
	// text-indent applies only when this element's first rendered content is
	// direct inline text, not a child block that will apply its own indent.
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
	// The top and bottom rules are resolved here rather than at the point
	// they're applied, because whether they exist decides whether an empty box
	// keeps its content row below. They only depend on hBorderWidth, which
	// nothing changes after the shrink-to-fit narrowing above.
	topRule := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth, r.profile)
	botRule := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth, r.profile)

	// An empty block box has zero content height in CSS. An empty bordered
	// <div> draws its top and bottom rule adjacent with nothing between them,
	// and a lone border-top, which is what <hr> is, is a single rule. Block
	// content here is always at least one line, that line being how block flow
	// separates one box from the next, so an empty box has to give that line up
	// explicitly or it reserves a blank row nothing asked for: a bordered empty
	// div came out three rows rather than two, and `<hr style="width: 20">` two
	// rather than one.
	//
	// This applies only when something else will actually occupy a row. A box
	// with no padding and no rules keeps its blank line instead of collapsing to
	// nothing: a zero-line box has no way to separate itself from its siblings
	// in block flow, and `<div></div>` has always been one empty line. A
	// declared height or min-height is exactly the request to reserve rows, so
	// neither is dropped. Both have already been applied as real lines by the
	// height step above, and the guard is what keeps `min-height: 1` from
	// reading as an empty box here.
	if contentEmpty && heightLines == 0 && minH == 0 && (pt+pb > 0 || topRule != "" || botRule != "") {
		b = box{}
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
	// No empty-box special case is needed on either rule: an empty box has
	// already surrendered its content line above, so it arrives here with no
	// lines at all and each rule becomes one.
	topRuleDrawn := false
	if topRule != "" {
		b.lines = append([]string{topRule}, b.lines...)
		b.width = linesWidth(b.lines)
		topRuleDrawn = true
	}
	if botRule != "" {
		b.lines = append(b.lines, botRule)
		b.width = linesWidth(b.lines)
	}
	if ml > 0 || mr > 0 {
		b = applyLineEdgesBox(b, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
	}
	if isHiddenVisibility(decls["visibility"]) {
		b = blankVisibleContentBox(b)
	}
	// Reset b.pre based on this element's own resolved white-space,
	// regardless of what children's boxes may have carried in. The old
	// cappedWriter model's EnterPre/ExitPre was scoped to exactly one
	// child's own content — inline.go's nested "block" case entered and
	// exited pre mode around writing just that child's block content into
	// the parent's writer — and never persisted past it: a <pre> nested
	// inside a non-pre ancestor loses its exemption the moment that
	// ancestor's own non-pre content reaches a writer instance that isn't in
	// pre mode. Crossing this function's own boundary is the box-model
	// equivalent of that per-element EnterPre/ExitPre scoping.
	if ws == "pre" || ws == "pre-wrap" {
		b.pre = make([]bool, len(b.lines))
		for i := range b.pre {
			b.pre[i] = true
		}
	} else {
		b.pre = nil
	}
	// Shift descendant positions, captured right after the initial wrap and
	// before any of the transformations above, by everything since applied
	// that moves rows or columns uniformly: pt prepends pt blank lines, a
	// drawn top rule prepends one more, pl and the left border character
	// each shift every line right, and ml, baked in as literal padding unlike
	// vertical margin, shifts right again. text-indent, which affects row 0
	// only, and text-align:right/center, which varies per line, are NOT
	// accounted for. That is an accepted approximation, since the primary use,
	// hit-testing form controls, essentially never combines those with tracked
	// descendants.
	rowShift := pt
	if topRuleDrawn {
		rowShift++
	}
	colShift := pl + textcell.Width(bl.char) + ml
	positions = mergePositions(nil, positions, rowShift, colShift)
	if r.liveContentOffsets == nil {
		r.liveContentOffsets = map[*html.Node]int{}
	}
	r.liveContentOffsets[n] = rowShift
	if r.liveContentOffsetsX == nil {
		r.liveContentOffsetsX = map[*html.Node]int{}
	}
	r.liveContentOffsetsX[n] = colShift
	if heightLines > 0 && (ovY == "scroll" || ovY == "auto") {
		// See recordScrollViewport's own doc comment; it's shared with
		// renderFlexContentBox.
		r.recordScrollViewport(n, heightLines, rowShift, colShift, innerW, gutterWidth, capStartDrawn, capEndDrawn)
	}
	if hasExplicitWidth && (ovX == "scroll" || ovX == "auto") {
		// See recordScrollViewportX's own doc comment; it's shared with
		// renderFlexContentBox.
		r.recordScrollViewportX(n, innerW, colShift, rowShift, gutterRowX, gutterHeightX, capStartXDrawn, capEndXDrawn)
	}
	return b, positions
}

// scrollbarStyle is one resolved ::scrollbar-track or ::scrollbar-thumb: the
// glyph repeated across the gutter's width, plus its text style, meaning
// color, background-color, and font-weight (see resolveScrollbarStyle). Always
// build style via extractInlineStyle or newInlineStyle, never the zero
// inlineStyle{}: per inlineStyle's own doc comment, the zero value reads as
// opacity:0 and silently blanks the glyph.
type scrollbarStyle struct {
	char  string
	style inlineStyle
}

// ScrollbarGutterWidth is the default column width reserved for the
// scrollbar gutter when overflow-y:scroll is set and no ::scrollbar width
// declaration overrides it. See docs/SCROLLING.md's "Scrollbar gutter and
// indicator" and docs/SCROLLBARS.md. It is exported, via
// htmlterm.ScrollbarGutterWidth, so callers who pre-render content outside a
// scrollable Document or Renderer pass — to cache an expensive layout, say,
// then splice it into a live scrollable pane via Document.SetPreRendered —
// can reserve the same column up front. Otherwise the pre-rendered content is
// wrapped 1 column wider than the live pane's actual content width once the
// gutter is reserved there, desyncing the two and producing exactly the
// scrollbar-off-by-one symptom this constant exists to prevent. A caller that
// also sets a custom ::scrollbar { width } on the live pane must account for
// that override itself; this constant only reflects the built-in default.
const ScrollbarGutterWidth = 1

// ScrollbarGutterHeight is ScrollbarGutterWidth's horizontal-scrollbar
// counterpart: the default row count reserved for a horizontal scrollbar
// gutter when overflow-x:scroll is set, and no ::scrollbar-x height
// declaration overrides it, on an element with an explicit width and no
// height of its own. See the "scroll"/"auto" case in
// renderBlockContentBox's overflow-x handling, and docs/SCROLLING.md.
const ScrollbarGutterHeight = 1

// scrollbarPreset is one named scrollbar-style's baseline ::scrollbar-track
// and ::scrollbar-thumb declarations. It is the same shape pseudoElemDecls
// returns from a real stylesheet rule, so it can be merged with one
// identically.
type scrollbarPreset struct {
	track, thumb, capStart, capEnd map[string]string
}

// defaultScrollbarStyle is used when scrollbar-style is unset or names a
// preset that doesn't exist. It reproduces this feature's original behavior,
// from before scrollbar-style and the ::scrollbar-track/thumb defaults
// existed, exactly.
const defaultScrollbarStyle = "block"

// scrollbarPresets backs the scrollbar-style property:
// block|shaded|classic|ascii|line. Each preset supplies content, and for
// classic a background-color, as a baseline that an element's own
// ::scrollbar-track, ::scrollbar-thumb, ::scrollbar-cap-start, and
// ::scrollbar-cap-end rules still override property-by-property. See
// resolveScrollbarStyle and resolveScrollbarCap.
//
// capStart and capEnd make the cap buttons opt-out, not opt-in. Every preset
// supplies an arrow glyph, and an element only goes without caps by
// explicitly setting content: none on the relevant pseudo-element, the same
// "none suppresses injection" convention ::before and ::after already have,
// or by the gutter being too short to spare a row for one (see
// appendScrollbarColumn). classic's colors are a deliberately neutral gray
// pair, not CSS-configurable via the scrollbar-style keyword itself. Override
// them with an explicit ::scrollbar-track, ::scrollbar-thumb, or
// ::scrollbar-cap-* rule, rather than a new preset, if a different palette is
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

// scrollbarGutterWidth resolves n's ::scrollbar { width } declaration, in ch
// or columns (see parseSizeVal), falling back to ScrollbarGutterWidth when
// unset, unparseable, or non-positive. Percentage widths are not meaningful
// for a gutter and are also treated as unset. This is independent of
// scrollbar-style: none of the named presets set a width, only the track and
// thumb glyph and color. elemDecls is n's own resolved declarations, which
// renderBlockContentBox already has as decls and which are not re-resolved
// here, read only for its custom properties, to resolve var() inside the
// ::scrollbar rule.
func (r *Engine) scrollbarGutterWidth(n *html.Node, elemDecls map[string]string) int {
	decls := r.pseudoElemDecls(n, "scrollbar", customPropSubset(elemDecls))
	if abs, pct, ok := parseSizeVal(decls["width"]); ok && pct == 0 && abs > 0 {
		return abs
	}
	return ScrollbarGutterWidth
}

// applyVerticalScroll implements overflow-y: scroll/auto's real-scrolling
// behavior: it clamps the live per-element scroll offset, slices lines to
// the visible heightLines window, shifts positions to match, and draws the
// scrollbar gutter when one was reserved. Shared by renderBlockContentBox and
// renderFlexContentBox, which otherwise arrive at lines by different routes
// (wrapped inline content versus flex item layout) but hit an identical
// scroll problem once heightLines and lines are in hand. See
// docs/SCROLLING.md's Section 1 "Rendering" and "Scrollbar gutter and
// indicator" for the design this implements.
//
// A scrolled container's clip must also shift its descendants' recorded
// positions by -offset, mirroring mergePositions' existing
// shift-on-placement primitive, so a scrolled-off descendant's Rect lands
// outside the visible range instead of the pre-scroll range. It is kept
// rather than deleted, matching a scrolled-off real DOM element's
// getBoundingClientRect(). r.liveScrollOffsets is this frame's freshly
// rebuilt scroll-offset map. r.scrollOffsets is nil for a plain
// Renderer.Render call, which has no persistent Document to read a prior
// offset from, so offset is 0 there.
//
// lines shorter than heightLines are padded with blank lines before the
// gutter is drawn, so the gutter's own track always spans the full declared
// height even when the caller's content came up short. For
// renderBlockContentBox this is load-bearing: wrapped inline content is never
// grown to fill a declared height on its own. For renderFlexContentBox it is
// a defensive no-op today, since layoutFlexRow/layoutFlexColumn already pad
// to heightLines themselves, but padding here too means that guarantee
// moving is not required to stay a cross-file invariant.
//
// capStartDrawn and capEndDrawn report which cap buttons, if any, were
// actually drawn (see appendScrollbarColumn), for the caller's own
// liveScrollViewport bookkeeping via recordScrollViewport.
func (r *Engine) applyVerticalScroll(n *html.Node, decls map[string]string, lines []string, positions map[*html.Node]Rect, heightLines, innerW, gutterWidth int, hasScrollbarGutter bool) ([]string, map[*html.Node]Rect, bool, bool) {
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
	blank := strings.Repeat(" ", innerW)
	for len(lines) < heightLines {
		lines = append(lines, blank)
	}
	// hasScrollbarGutter, rather than just an overflow-y of exactly "scroll",
	// draws an always-on gutter indicator, regardless of whether this frame
	// needed to slice. See docs/SCROLLING.md's "Scrollbar gutter and
	// indicator" for why "auto" deliberately gets none, and why a too-narrow
	// box, one where the gutter wasn't actually reserved in innerW, must not
	// draw one either: content would get an unreserved column appended on top
	// of it instead of a properly narrowed box.
	var capStartDrawn, capEndDrawn bool
	if hasScrollbarGutter {
		track := r.resolveScrollbarStyle(n, decls, "scrollbar-track")
		thumb := r.resolveScrollbarStyle(n, decls, "scrollbar-thumb")
		capStart, hasCapStart := r.resolveScrollbarCap(n, decls, "scrollbar-cap-start")
		capEnd, hasCapEnd := r.resolveScrollbarCap(n, decls, "scrollbar-cap-end")
		lines, capStartDrawn, capEndDrawn = appendScrollbarColumn(lines, offset, totalLines, heightLines, innerW, gutterWidth, track, thumb, capStart, capEnd, hasCapStart, hasCapEnd, r.profile)
	}
	return lines, positions, capStartDrawn, capEndDrawn
}

// recordScrollViewport stores this frame's Viewport geometry for n, an
// overflow-y:scroll|auto container with a resolved height. Shared by
// renderBlockContentBox and renderFlexContentBox: a scroll container's
// Viewport shape has nothing box-model-specific in it, just the values both
// callers already have in hand at the point they call this, after resolving
// their own rowShift/colShift.
//
// Rect, assigned by whichever caller embeds this box as a token, is the full
// CSS border box, which unlike heightLines includes any border and padding
// rows added around content. DispatchKey's PageUp/PageDown and Focus's
// scrollIntoView both need the actual content-box viewport height and the
// row offset from this box's own top to its first visible content row, which
// only rowShift and heightLines capture.
//
// GutterCol is the offset from this box's own Rect.Col, the same
// relationship rowShift already has to Rect.Row, to where child content
// starts. The gutter sits immediately after content, so it's colShift plus
// innerW. Only meaningful when GutterWidth > 0, and only used then by
// document.go's tryScrollCapClick.
func (r *Engine) recordScrollViewport(n *html.Node, heightLines, rowShift, colShift, innerW, gutterWidth int, capStartDrawn, capEndDrawn bool) {
	if r.liveScrollViewport == nil {
		r.liveScrollViewport = map[*html.Node]Viewport{}
	}
	r.liveScrollViewport[n] = Viewport{
		Height:      heightLines,
		TopOffset:   rowShift,
		GutterCol:   colShift + innerW,
		GutterWidth: gutterWidth,
		CapStart:    capStartDrawn,
		CapEnd:      capEndDrawn,
	}
}

// applyHorizontalScroll is applyVerticalScroll's horizontal counterpart:
// clamps the live per-element column offset, windows every line to
// [offsetX, offsetX+innerW), shifts positions to match, and appends the
// scrollbar gutter row when this box has no vertical gutter of its own to
// conflict with. Shared by renderBlockContentBox and renderFlexContentBox.
//
// maxLineWidth is the caller's own already-computed max line width (b.width
// or content.width, both linesWidth's output from moments earlier in either
// caller), not rescanned here: lines hasn't been touched since that width was
// computed, so a second linear scan over it would only reproduce the same
// number.
//
// Unlike overflow-y's height gate, there's no separate "heightLines" concept
// to slice against here. For an ordinary block box, innerW already bounds
// normally-wrapped content to begin with, so this only ever does something
// for content wordWrapTokens couldn't shrink to fit, meaning white-space:pre
// or nowrap, or an unbreakable overlong token: a wrapped box's own
// maxLineWidth already comes out <= innerW, which naturally clamps offsetX to
// 0. A flex container's lines can come out wider than innerW far more
// routinely, from flex-shrink:0 items or the automatic minimum size floor
// (flexMainAxisFloor); see docs/SCROLLING.md.
//
// heightLines and ovX are the caller's own overflow-y height and overflow-x
// value, used only to compute drawGutterRow := heightLines==0 &&
// ovX=="scroll" internally rather than making each caller repeat that
// formula: a visible horizontal gutter row is only drawn when ovX is exactly
// "scroll", mirroring overflow-y's own "auto gets no indicator" convention,
// and only when the box has no fixed height of its own. An active vertical
// gutter or scroll region already claims the box's bottom row for its own
// purposes, and this renderer has no corner-cell concept to let both visible
// gutters coexist there, the same problem a real GUI scrollbar solves with a
// dedicated corner square. The offset and scrolling above still work in that
// combination; only the drawn indicator is skipped.
func (r *Engine) applyHorizontalScroll(n *html.Node, decls map[string]string, lines []string, positions map[*html.Node]Rect, innerW, maxLineWidth, heightLines int, ovX string) ([]string, map[*html.Node]Rect, int, bool, bool) {
	drawGutterRow := heightLines == 0 && ovX == "scroll"
	offsetX := r.scrollOffsetsX[n]
	maxOffsetX := max(0, maxLineWidth-innerW)
	offsetX = min(max(offsetX, 0), maxOffsetX)
	if r.liveScrollOffsetsX == nil {
		r.liveScrollOffsetsX = map[*html.Node]int{}
	}
	r.liveScrollOffsetsX[n] = offsetX
	newLines := make([]string, len(lines))
	for i, ln := range lines {
		window := textcell.VisibleWindow(ln, offsetX, innerW)
		if pad := innerW - textcell.VisibleLen(window); pad > 0 {
			window += strings.Repeat(" ", pad)
		}
		newLines[i] = window
	}
	positions = mergePositions(nil, positions, 0, -offsetX)
	var gutterHeightX int
	var capStartXDrawn, capEndXDrawn bool
	if drawGutterRow {
		gutterHeightX = r.scrollbarGutterHeight(n, decls)
		trackX := r.resolveScrollbarStyle(n, decls, "scrollbar-track-x")
		thumbX := r.resolveScrollbarStyle(n, decls, "scrollbar-thumb-x")
		capStartX, hasCapStartX := r.resolveScrollbarCap(n, decls, "scrollbar-cap-start-x")
		capEndX, hasCapEndX := r.resolveScrollbarCap(n, decls, "scrollbar-cap-end-x")
		newLines, capStartXDrawn, capEndXDrawn = appendScrollbarRow(newLines, offsetX, maxLineWidth, innerW, gutterHeightX, trackX, thumbX, capStartX, capEndX, hasCapStartX, hasCapEndX, r.profile)
	}
	return newLines, positions, gutterHeightX, capStartXDrawn, capEndXDrawn
}

// recordScrollViewportX is recordScrollViewport's horizontal counterpart,
// shared the same way between renderBlockContentBox and
// renderFlexContentBox. gutterRowX is the row count of the caller's own
// content lines captured before applyHorizontalScroll's gutter row (if any)
// was appended on top, the same "before" point colShift/rowShift are always
// measured from; GutterRow is only meaningful when GutterHeight > 0, and only
// used then by document.go's tryScrollCapClickX.
func (r *Engine) recordScrollViewportX(n *html.Node, innerW, colShift, rowShift, gutterRowX, gutterHeightX int, capStartXDrawn, capEndXDrawn bool) {
	if r.liveScrollViewportX == nil {
		r.liveScrollViewportX = map[*html.Node]ViewportX{}
	}
	r.liveScrollViewportX[n] = ViewportX{
		Width:        innerW,
		LeftOffset:   colShift,
		GutterRow:    rowShift + gutterRowX,
		GutterHeight: gutterHeightX,
		CapStart:     capStartXDrawn,
		CapEnd:       capEndXDrawn,
	}
}

// scrollbarGutterHeight is scrollbarGutterWidth's horizontal counterpart. It
// resolves n's ::scrollbar-x { height } declaration, a bare row count (see
// parseSizeVal) matching how the "height" property itself is parsed
// elsewhere in this package rather than the ch or column unit ::scrollbar's
// own width uses, falling back to ScrollbarGutterHeight when unset,
// unparseable, or non-positive. Percentage heights are treated as unset,
// mirroring scrollbarGutterWidth's own percentage handling.
func (r *Engine) scrollbarGutterHeight(n *html.Node, elemDecls map[string]string) int {
	decls := r.pseudoElemDecls(n, "scrollbar-x", customPropSubset(elemDecls))
	if abs, pct, ok := parseSizeVal(decls["height"]); ok && pct == 0 && abs > 0 {
		return abs
	}
	return ScrollbarGutterHeight
}

// resolveScrollbarStyle resolves n's effective ::scrollbar-track or
// ::scrollbar-thumb style into a glyph plus text style. which is
// "scrollbar-track" or "scrollbar-thumb", or their horizontal-scrollbar
// counterparts "scrollbar-track-x" and "scrollbar-thumb-x".
//
// elemDecls is n's own resolved declarations, which renderBlockContentBox
// already has as decls and which are not re-resolved here, read only for
// scrollbar-style. That selects which scrollbarPresets entry supplies the
// baseline, falling back to defaultScrollbarStyle when unset or unrecognized.
// n's actual ::scrollbar-track or ::scrollbar-thumb rule, if any, is then
// layered on top of that baseline property-by-property: a content, color, or
// other property the rule sets wins, and anything the rule doesn't mention
// falls through to the preset. That is what lets `scrollbar-style: classic`
// plus a lone `::scrollbar-thumb { color: red }` combine instead of one
// replacing the other outright.
func (r *Engine) resolveScrollbarStyle(n *html.Node, elemDecls map[string]string, which string) scrollbarStyle {
	preset, ok := scrollbarPresets[elemDecls["scrollbar-style"]]
	if !ok {
		preset = scrollbarPresets[defaultScrollbarStyle]
	}
	base := preset.track
	if which == "scrollbar-thumb" || which == "scrollbar-thumb-x" {
		base = preset.thumb
	}
	merged := make(map[string]string, len(base))
	maps.Copy(merged, base)
	maps.Copy(merged, r.pseudoElemDecls(n, which, customPropSubset(elemDecls)))
	ch := r.parseCSSContentString(merged["content"], n)
	return scrollbarStyle{char: ch, style: extractInlineStyle(merged)}
}

// resolveScrollbarCap resolves n's effective ::scrollbar-cap-start or
// ::scrollbar-cap-end style into a glyph plus text style, through the same
// preset-baseline-plus-override merge resolveScrollbarStyle already does for
// ::scrollbar-track and ::scrollbar-thumb. which is "scrollbar-cap-start" or
// "scrollbar-cap-end", or their horizontal-scrollbar counterparts
// "scrollbar-cap-start-x" and "scrollbar-cap-end-x", and elemDecls is n's own
// resolved declarations, read only for scrollbar-style.
//
// Caps are opt-out, not opt-in. Every scrollbarPresets entry supplies an arrow
// glyph for both ends, so ok is true unless an element's own
// ::scrollbar-cap-start or ::scrollbar-cap-end rule explicitly sets content:
// none or normal, the same "none suppresses injection" convention ::before and
// ::after already have, or there's no room for the cap this frame, which
// appendScrollbarColumn handles rather than this function.
func (r *Engine) resolveScrollbarCap(n *html.Node, elemDecls map[string]string, which string) (scrollbarStyle, bool) {
	preset, ok := scrollbarPresets[elemDecls["scrollbar-style"]]
	if !ok {
		preset = scrollbarPresets[defaultScrollbarStyle]
	}
	base := preset.capStart
	if which == "scrollbar-cap-end" || which == "scrollbar-cap-end-x" {
		base = preset.capEnd
	}
	merged := make(map[string]string, len(base))
	maps.Copy(merged, base)
	maps.Copy(merged, r.pseudoElemDecls(n, which, customPropSubset(elemDecls)))
	// parseCSSContentString itself already treats "none" and "normal" as
	// empty (see its own doc comment), so an explicit ::scrollbar-cap-start
	// or ::scrollbar-cap-end { content: none; } rule overriding the preset's
	// own content here is what turns ch empty, and so ok false. No separate
	// check is needed.
	ch := r.parseCSSContentString(merged["content"], n)
	if ch == "" {
		return scrollbarStyle{}, false
	}
	return scrollbarStyle{char: ch, style: extractInlineStyle(merged)}, true
}

// defaultBarPreset is progressPresets and meterPresets' fallback name when
// progress-style or meter-style is unset or names a preset that doesn't
// exist. Same value and reasoning as defaultScrollbarStyle, kept as its
// own constant since progress-style and meter-style are properties in their
// own right, not scrollbar-style itself.
const defaultBarPreset = "block"

// progressPreset is one named progress-style's baseline ::progress-bar and
// ::progress-value declarations. It mirrors scrollbarPreset's track and thumb
// shape for <progress>'s two-glyph bar. See docs/proposals/PROGRESS_METER.md.
type progressPreset struct {
	bar, value map[string]string
}

// progressPresets backs the progress-style property:
// block|shaded|classic|ascii|line. Those are the same five names
// scrollbar-style already uses, for the same "one small, memorable vocabulary
// across this codebase's terminal-native styling surfaces" reason
// docs/TABLES.md's border presets and docs/SCROLLBARS.md's scrollbar presets
// already share. An element's own ::progress-bar or ::progress-value rule
// still overrides property-by-property on top of these; see mergePresetStyle.
var progressPresets = map[string]progressPreset{
	"block":   {bar: map[string]string{"content": `"░"`}, value: map[string]string{"content": `"█"`}},
	"shaded":  {bar: map[string]string{"content": `"░"`}, value: map[string]string{"content": `"▓"`}},
	"classic": {bar: map[string]string{"content": `" "`, "background-color": "#444444"}, value: map[string]string{"content": `" "`, "background-color": "#aaaaaa"}},
	"ascii":   {bar: map[string]string{"content": `"-"`}, value: map[string]string{"content": `"#"`}},
	"line":    {bar: map[string]string{"content": `"─"`}, value: map[string]string{"content": `"━"`}},
}

// meterPreset is one named meter-style's baseline ::meter-bar/
// ::meter-optimum-value/::meter-suboptimum-value/::meter-even-less-good-value
// declarations. <meter> gets three value pseudo-elements instead of
// ::progress-value's one because real UAs already color-code by region, and
// collapsing them to a single glyph would lose that distinction.
type meterPreset struct {
	bar, optimum, suboptimum, evenLessGood map[string]string
}

// meterPresets backs the meter-style property, sharing progressPresets'
// exact five names. classic's region colors, green, yellow, and red, are the
// same ones WebKit's own UA stylesheet uses by default for <meter>, picked
// for recognizability rather than as an invented palette. They are reused
// unchanged across all five presets here, with only the glyph and background
// varying per preset, matching the proposal's preset table.
var meterPresets = map[string]meterPreset{
	"block": {
		bar:          map[string]string{"content": `"░"`},
		optimum:      map[string]string{"content": `"█"`, "color": "#2e7d32"},
		suboptimum:   map[string]string{"content": `"█"`, "color": "#f9a825"},
		evenLessGood: map[string]string{"content": `"█"`, "color": "#c62828"},
	},
	"shaded": {
		bar:          map[string]string{"content": `"░"`},
		optimum:      map[string]string{"content": `"▓"`, "color": "#2e7d32"},
		suboptimum:   map[string]string{"content": `"▓"`, "color": "#f9a825"},
		evenLessGood: map[string]string{"content": `"▓"`, "color": "#c62828"},
	},
	"classic": {
		bar:          map[string]string{"content": `" "`, "background-color": "#444444"},
		optimum:      map[string]string{"content": `" "`, "background-color": "#2e7d32"},
		suboptimum:   map[string]string{"content": `" "`, "background-color": "#f9a825"},
		evenLessGood: map[string]string{"content": `" "`, "background-color": "#c62828"},
	},
	"ascii": {
		bar:          map[string]string{"content": `"-"`},
		optimum:      map[string]string{"content": `"#"`, "color": "#2e7d32"},
		suboptimum:   map[string]string{"content": `"#"`, "color": "#f9a825"},
		evenLessGood: map[string]string{"content": `"#"`, "color": "#c62828"},
	},
	"line": {
		bar:          map[string]string{"content": `"─"`},
		optimum:      map[string]string{"content": `"━"`, "color": "#2e7d32"},
		suboptimum:   map[string]string{"content": `"━"`, "color": "#f9a825"},
		evenLessGood: map[string]string{"content": `"━"`, "color": "#c62828"},
	},
}

// mergePresetStyle merges base, a progressPresets or meterPresets entry's
// baseline declarations for one pseudo-element, with n's own ::which rule,
// which always wins property-by-property, anything it doesn't set falling
// through to base. It then resolves the merged content, color, and other
// properties into a glyph plus text style. This generalizes the merge
// contract resolveScrollbarStyle and resolveScrollbarCap already use, since
// progress and meter need it across six pseudo-elements — ::progress-bar and
// -value, ::meter-bar, -optimum-value, -suboptimum-value, and
// -even-less-good-value — instead of scrollbar's four.
func (r *Engine) mergePresetStyle(n *html.Node, elemDecls, base map[string]string, which string) scrollbarStyle {
	merged := make(map[string]string, len(base))
	maps.Copy(merged, base)
	maps.Copy(merged, r.pseudoElemDecls(n, which, customPropSubset(elemDecls)))
	ch := r.parseCSSContentString(merged["content"], n)
	return scrollbarStyle{char: ch, style: extractInlineStyle(merged)}
}

// resolveProgressStyle resolves a <progress>'s effective ::progress-bar and
// ::progress-value styles. The baseline is
// progressPresets[elemDecls["progress-style"]], or defaultBarPreset when that
// is unset or unrecognized, with n's own ::progress-bar and ::progress-value
// rule layered on top property-by-property.
func (r *Engine) resolveProgressStyle(n *html.Node, elemDecls map[string]string) (bar, value scrollbarStyle) {
	preset, ok := progressPresets[elemDecls["progress-style"]]
	if !ok {
		preset = progressPresets[defaultBarPreset]
	}
	bar = r.mergePresetStyle(n, elemDecls, preset.bar, "progress-bar")
	value = r.mergePresetStyle(n, elemDecls, preset.value, "progress-value")
	return bar, value
}

// resolveMeterStyle is resolveProgressStyle's <meter> counterpart, resolving
// all four of ::meter-bar, ::meter-optimum-value, ::meter-suboptimum-value,
// and ::meter-even-less-good-value the same way.
func (r *Engine) resolveMeterStyle(n *html.Node, elemDecls map[string]string) (bar, optimum, suboptimum, evenLessGood scrollbarStyle) {
	preset, ok := meterPresets[elemDecls["meter-style"]]
	if !ok {
		preset = meterPresets[defaultBarPreset]
	}
	bar = r.mergePresetStyle(n, elemDecls, preset.bar, "meter-bar")
	optimum = r.mergePresetStyle(n, elemDecls, preset.optimum, "meter-optimum-value")
	suboptimum = r.mergePresetStyle(n, elemDecls, preset.suboptimum, "meter-suboptimum-value")
	evenLessGood = r.mergePresetStyle(n, elemDecls, preset.evenLessGood, "meter-even-less-good-value")
	return bar, optimum, suboptimum, evenLessGood
}

// formControlBarDefaultWidth is the fallback inner width for a <progress> or
// <meter> bar when, unexpectedly, no width resolves at all. The UA stylesheet
// always sets width:20, so resolveWidthConstraints should normally already
// return this exact value as decls["width"]'s resolved size. See
// docs/proposals/PROGRESS_METER.md's Sizing section for why 20 was chosen.
const formControlBarDefaultWidth = 20

// appendScrollbarColumn appends one scrollbar gutter to each of lines, using
// the standard proportional thumb-size and thumb-position formula. The gutter
// is gutterWidth columns wide, each column holding either track.char or
// thumb.char, styled per track.style or thumb.style. totalLines is the
// content's line count before it was sliced or padded to heightLines, so the
// thumb reflects the real scrollable range even though lines itself no
// longer does. It appends rather than overwrites, so real content is never
// clobbered; see docs/SCROLLING.md's rejected splice-overlay alternative
// for why that matters. When totalLines <= heightLines, meaning there is
// nothing to scroll, thumbSize comes out to heightLines, so the thumb fills
// the whole track, matching a real scrollbar's own convention for "you can
// already see everything".
//
// innerW is the box's content width, excluding the gutter itself. Each line
// is padded or truncated to exactly innerW visible columns before the glyphs
// are appended. Upstream width-normalization, in alignLinesBox and
// padLinesToWidthBox, only runs conditionally, and even then never truncates
// a line already at or past width, such as one holding an unbreakable
// overlong token, so without this the gutter column would land on a ragged,
// content-dependent column instead of a straight line at the pane's right
// edge.
//
// capStart and capEnd are the resolved ::scrollbar-cap-start and
// ::scrollbar-cap-end glyphs (see resolveScrollbarCap), each gated by its own
// hasCapStart or hasCapEnd. Caps are opt-out, on by default via
// scrollbarPresets, so either can still be individually false when a rule
// explicitly disabled it with content: none. When active, a cap claims row 0
// for the start and the last row for the end verbatim, and the thumb-size and
// thumb-position formula runs over the interior track, heightLines minus
// however many caps are active, so the thumb never overlaps a cap. If there
// isn't at least 1 interior row left once active caps are subtracted, both
// caps are silently dropped for this render, not just the one that doesn't
// fit, following the same "drop the added chrome, keep content correct"
// precedent already used when the gutter itself doesn't fit. The returned
// bools report which caps were drawn, for the caller to record in Viewport,
// since document.go's click hit-testing must not treat a dropped cap as
// clickable.
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
		ln = textcell.TruncateToWidth(ln, innerW, "")
		if pad := innerW - textcell.VisibleLen(ln); pad > 0 {
			ln += strings.Repeat(" ", pad)
		}
		out[i] = ln + g.style.render(strings.Repeat(g.char, gutterWidth), profile)
	}
	return out, hasCapStart, hasCapEnd
}

// appendScrollbarRow is appendScrollbarColumn's horizontal-scrollbar
// counterpart. It appends gutterHeight identical rows below lines, each
// innerW columns wide with one glyph per column, track.char or thumb.char per
// the standard proportional thumb formula transposed onto the width axis.
// offsetX, maxLineWidth, and innerW play the roles offset, totalLines, and
// heightLines play for the vertical version. capStart and capEnd occupy the
// leftmost and rightmost column of every gutter row instead of the topmost
// and bottommost gutter row, with the same "not enough room ⇒ drop both caps"
// rule appendScrollbarColumn already has, transposed to innerW-activeCaps < 1
// instead of heightLines-activeCaps < 1. Every gutter row is identical,
// unlike a vertical gutter column wider than 1, whose glyph is repeated
// across its columns, since gutterHeight rows have no independent
// column-axis content of their own to differ by.
func appendScrollbarRow(lines []string, offsetX, maxLineWidth, innerW, gutterHeight int, track, thumb, capStart, capEnd scrollbarStyle, hasCapStart, hasCapEnd bool, profile colorprofile.Profile) ([]string, bool, bool) {
	activeCaps := 0
	if hasCapStart {
		activeCaps++
	}
	if hasCapEnd {
		activeCaps++
	}
	if activeCaps > 0 && innerW-activeCaps < 1 {
		hasCapStart, hasCapEnd = false, false
		activeCaps = 0
	}
	interior := innerW - activeCaps

	thumbSize := interior
	if maxLineWidth > innerW {
		thumbSize = max(1, min(interior*interior/maxLineWidth, interior))
	}
	thumbStart := 0
	if maxOffset := maxLineWidth - innerW; maxOffset > 0 {
		thumbStart = offsetX * (interior - thumbSize) / maxOffset
	}

	capOffset := 0
	if hasCapStart {
		capOffset = 1
	}
	var row strings.Builder
	for col := range innerW {
		var g scrollbarStyle
		switch {
		case hasCapStart && col == 0:
			g = capStart
		case hasCapEnd && col == innerW-1:
			g = capEnd
		default:
			g = track
			if interiorIdx := col - capOffset; interiorIdx >= thumbStart && interiorIdx < thumbStart+thumbSize {
				g = thumb
			}
		}
		row.WriteString(g.style.render(g.char, profile))
	}
	rowStr := row.String()
	out := make([]string, len(lines)+gutterHeight)
	copy(out, lines)
	for i := range gutterHeight {
		out[len(lines)+i] = rowStr
	}
	return out, hasCapStart, hasCapEnd
}

// firstContentIsInline reports whether n's first non-whitespace content is
// inline, meaning a text node or inline element. It returns false when the
// first child is a block-level element, so text-indent should not be applied
// here: the block child will apply its own inherited value on its own first
// line.
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

// blankLineVisible replaces line's visible content with spaces, preserving
// its **column** count rather than its rune count, which is what this used to
// do. The distinction is the point of visibility:hidden over display:none: a
// hidden element must go on occupying exactly the room it occupied, so the
// content after it doesn't move. Blanking "日本語", 3 runes and 6 columns, to
// 3 spaces slid the rest of the line 3 columns left.
func blankLineVisible(line string) string {
	return strings.Repeat(" ", textcell.VisibleLen(line))
}

// blankVisibleContentBox is blankVisibleContent's box-based core.
func blankVisibleContentBox(b box) box {
	lines := make([]string, len(b.lines))
	for i, line := range b.lines {
		lines[i] = blankLineVisible(line)
	}
	return box{lines: lines, width: linesWidth(lines)}
}

// isQuotedCSSValue reports whether v, after trimming, is a CSS quoted
// string token. It is the disambiguator resolveBorderEdgeChar uses between a
// literal border glyph, as in border-top: "═", and the standard border-edge
// shorthand grammar, as in border-top: solid red.
func isQuotedCSSValue(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return false
	}
	return (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')
}

// resolveBorderEdgeChar parses the raw declared value of one of border-top,
// border-right, border-bottom, or border-left. A quoted string is this
// engine's literal-glyph form and is unquoted via parseCSSString unchanged.
// Anything else has already passed through expandShorthand (css.go) as the
// standard CSS border-edge shorthand grammar, which leaves just a bare style
// keyword here, its width and color tokens if any having already been split
// into border-*-color there. So glyph picks that preset's character for this
// specific edge: top.fill for border-top, left for border-left, and so on.
//
// present reports whether the declaration existed at all, even when it
// resolves to an empty character, as an explicit "none" or "hidden" style
// does. Callers must not let the border-style backfill override an edge that
// was deliberately cleared.
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

// parseCSSString unquotes a CSS quoted string token, turning `"│"` into `│`.
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
		// \<newline>: line continuation, consume and skip the newline
		if next == '\n' {
			continue
		}
		// \<hex>{1,6}<optional-space>: Unicode code point
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
		// \<other>: the character itself
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
// It supports quoted strings, attr(), counter(), counters(), open-quote,
// close-quote, no-open-quote, and no-close-quote. Returns "" for none and
// normal.
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
			// quoted string: find the closing quote
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
			// unrecognised token: skip one word
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

// wrapHyperlinkBox is wrapHyperlink's box-based equivalent, preserving b.pre.
// The string-signature version, which joins and re-splits, would silently
// drop it, since box.join() has no pre-tagging concept. The OSC 8 sequences
// are zero-width and never contain "\n", so this never changes b's line count.
//
// Every line gets its own open and close pair, not just line 0 or the last
// line. The terminal-facing consumer of this output, ../tui/cellbridge.go's
// writeANSILine, decodes each screen row independently from a fresh state,
// the same way it re-derives SGR style per row rather than carrying it
// across rows. An open only on line 0 left every wrapped continuation
// line of a multi-line block or flex <a>, common for HTML-email "read more"
// and CTA buttons, with no URL attached to its cells at all: still
// underlined, since SGR is correctly self-contained per line, but not
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
