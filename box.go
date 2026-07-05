package htmlterm

import "strings"

// parsePaddingLen parses a CSS padding-<side> value as an absolute character
// count. Percentages are not supported for padding (parseSizeVal's pct return
// is discarded), matching prior behavior in both the block and table-cell box
// models.
func parsePaddingLen(v string) int {
	abs, _, ok := parseSizeVal(v)
	if !ok {
		return 0
	}
	return abs
}

// resolveMarginSide resolves a CSS margin-left/margin-right value against
// availWidth. isAuto reports whether the value is the literal "auto" keyword,
// in which case val is 0 and the caller resolves the actual value later (once
// the box's final rendered width is known) via splitAutoMargins.
func resolveMarginSide(v string, availWidth int) (val int, isAuto bool) {
	if strings.TrimSpace(v) == "auto" {
		return 0, true
	}
	if abs, pct, ok := parseSizeVal(v); ok {
		if pct > 0 {
			return int(pct * float64(availWidth)), false
		}
		return abs, false
	}
	return 0, false
}

// splitAutoMargins resolves margin-left/margin-right values of "auto" once a
// box's final rendered width is known. remaining is the leftover space
// (availWidth minus the box's own rendered width minus any non-auto margin
// already resolved by resolveMarginSide); ml/mr are that prior result, passed
// through unchanged on the non-auto side. Both auto splits the remainder
// evenly; a single auto side absorbs all of it — matching CSS auto-margin
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
