package render

import "testing"

// Reproduces the truncation-boundary bug: visiblePrefixWithTrailingEscapes
// (and truncateToWidth, which calls it) checks "visible < width" BEFORE
// adding the next rune's width, so a wide (2-column) rune landing exactly on
// the boundary gets included even though doing so overshoots width by 1.
func TestWideRuneAtTruncationBoundary(t *testing.T) {
	s := "123456789012345678🎉XXXX" // 18 ascii cols, then a wide emoji, then filler
	got := truncateToWidth(s, 19, "")
	gotWidth := ansiVisibleLen(got)
	if gotWidth > 19 {
		t.Errorf("truncateToWidth(%q, 19) = %q with visible width %d, want <= 19", s, got, gotWidth)
	}
}
