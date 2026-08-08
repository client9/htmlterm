package render

import (
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
)

// box is rendered content that hasn't been assigned an absolute position yet
// Position is assigned by whichever caller embeds it into a parent. lines
// holds one entry per output row (ANSI-styled, no trailing "\n"); width is
// the visible column width, uniform across lines, matching what today's
// string-based helpers already enforce via padding in practice. pre marks
// lines that came from pre-formatted content, meaning white-space: pre or
// pre-wrap. It is nil when no such lines are present, and otherwise parallel
// to lines, so the
// final blank-run-capping pass can exempt them the way cappedWriter's
// preDepth used to.
type box struct {
	lines []string
	width int
	pre   []bool
}

// newBox splits s into a box, width computed via linesWidth so callers that
// haven't padded every line to a uniform width yet still get a usable (if
// not yet uniform) box.
func newBox(s string) box {
	lines := strings.Split(s, "\n")
	return box{lines: lines, width: linesWidth(lines)}
}

// linesWidth is the max textcell.VisibleLen across lines: the box.width a set of
// lines would get if wrapped in a box.
func linesWidth(lines []string) int {
	w := 0
	for _, line := range lines {
		if vl := textcell.VisibleLen(line); vl > w {
			w = vl
		}
	}
	return w
}

// join reassembles b's lines into a single string, one "\n" between each
// line and none at the end. It is the inverse of newBox for content with no
// trailing newline.
func (b box) join() string {
	return strings.Join(b.lines, "\n")
}

// forceHeight clips or pads lines to exactly height rows (height > 0):
// truncates from the end if there are more, appends empty lines if there are
// fewer. Used for Options.Height/renderTree's root-level height constraint,
// which, unlike a per-element "height" (renderBlockContentBox), has no
// paired "overflow" declaration to gate clipping on: it's a host-supplied
// viewport size, not CSS, so it always both pads and truncates.
func forceHeight(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// parsePaddingLen parses a CSS padding-<side> value as an absolute character
// count. Percentages are not supported for padding, matching prior behavior
// in both the block and table-cell box models: passing basisOK=false to
// resolveLength means a bare "50%" fails to resolve exactly as it always
// did, and a calc() with a "%" literal anywhere in it fails closed the same
// way, with no separate case needed for it. A calc() with no percentage in
// it, e.g. `padding-left: calc(2 * 2)`, still resolves normally, since it
// never needed a basis to begin with. A negative result clamps to 0,
// matching real CSS's own "negative padding is invalid" rule; this couldn't
// arise before calc() existed.
func parsePaddingLen(v string) int {
	n, ok := resolveLength(v, 0, false)
	if !ok || n < 0 {
		return 0
	}
	return n
}

// resolveMarginSide resolves a CSS margin-left/margin-right value against
// availWidth. isAuto reports whether the value is the literal "auto" keyword,
// in which case val is 0 and the caller resolves the actual value later (once
// the box's final rendered width is known) via splitAutoMargins. Unlike
// padding, a negative result is left alone rather than clamped: negative
// margins are valid CSS.
func resolveMarginSide(v string, availWidth int) (val int, isAuto bool) {
	if strings.TrimSpace(v) == "auto" {
		return 0, true
	}
	if n, ok := resolveLength(v, availWidth, true); ok {
		return n, false
	}
	return 0, false
}

// splitAutoMargins resolves margin-left/margin-right values of "auto" once a
// box's final rendered width is known. remaining is the leftover space
// (availWidth minus the box's own rendered width minus any non-auto margin
// already resolved by resolveMarginSide); ml/mr are that prior result, passed
// through unchanged on the non-auto side. Both auto splits the remainder
// evenly, and a single auto side absorbs all of it, matching CSS auto-margin
// resolution.
func splitAutoMargins(remaining, ml, mr int, mlAuto, mrAuto bool) (int, int) {
	if remaining < 0 {
		remaining = 0
	}
	switch {
	case mlAuto && mrAuto:
		ml = remaining / 2
		mr = remaining - ml
	case mlAuto:
		ml = remaining
	case mrAuto:
		mr = remaining
	}
	return ml, mr
}

// clampCellPadding returns effective (pl, pr, contentW) for a box of the given
// width, shrinking padding (right first, then left) so content gets at least
// 1 character. Shared by block box layout (block.go) and table-cell layout
// (table_render.go) so both clamp padding overflow identically instead of one
// letting the box overflow past its available width.
func clampCellPadding(width, pl, pr int) (int, int, int) {
	if width <= 0 {
		return 0, 0, 0
	}
	contentW := width - pl - pr
	if contentW < 1 {
		excess := 1 - contentW
		if pr >= excess {
			pr -= excess
		} else {
			pl = max(0, pl-(excess-pr))
			pr = 0
		}
		contentW = 1
	}
	return pl, pr, contentW
}
