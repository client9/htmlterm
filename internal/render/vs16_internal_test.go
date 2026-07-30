package render

import (
	"testing"

	"github.com/client9/htmlterm/internal/textcell"
)

// Regression test for the scrollbar-gutter-off-by-one bug: ambiguous-width
// emoji (heart, warning triangle, play button, ...) paired with VARIATION
// SELECTOR-16 (U+FE0F) render as width 2 in virtually every modern terminal.
// This used to require a hand-rolled correction on top of go-runewidth's
// rune-by-rune scoring (width 1 for the base rune alone, 0 for VS16 itself);
// displaywidth's grapheme-cluster width already folds the pair into one
// width-2 unit, so these cases now need no special-casing at all.
func TestVS16WidthCorrection(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"heart with VS16", "❤️", 2},              // ❤️
		{"warning with VS16", "⚠️", 2},            // ⚠️
		{"play button with VS16", "▶️", 2},        // ▶️
		{"heart alone, no VS16", "❤", 1},          // ❤ (ambiguous, defaults narrow)
		{"already-wide emoji unaffected", "✅", 2}, // ✅ (unambiguous, VS16 not needed)
		{"plain ascii unaffected", "hi", 2},
		{"heart with VS16 amid text", "a❤️b", 4}, // a + ❤️(2) + b
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := textcell.Width(tc.s); got != tc.want {
				t.Errorf("textcell.Width(%q) = %d, want %d", tc.s, got, tc.want)
			}
			if got := textcell.VisibleLen(tc.s); got != tc.want {
				t.Errorf("textcell.VisibleLen(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}
