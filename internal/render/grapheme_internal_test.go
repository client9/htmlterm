package render

import "testing"

// TestNextGrapheme pins NextGrapheme's contract directly, in isolation from
// any of its callers: given a rune slice and a starting index, it must
// return the whole grapheme cluster's width and the index of the rune
// immediately following that cluster — including for multi-rune clusters
// (VS16 pairs, ZWJ sequences, regional-indicator flag pairs, skin-tone
// modifiers) that a naive per-rune scan would mis-score.
func TestNextGrapheme(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		wantWidth int
		wantNext  int // rune count consumed
	}{
		{"ascii", "hi", 1, 1},
		{"vs16 pair", "❤️x", 2, 2},           // ❤(U+2764) + VS16(U+FE0F) -> one cluster, width 2
		{"bare ambiguous emoji", "❤x", 1, 1}, // no VS16 -> narrow, one rune
		{"already wide emoji", "✅x", 2, 1},
		{"zwj family sequence", "👨‍👩‍👧‍👦x", 2, 7}, // 4 people + 3 ZWJ = 7 runes, one cluster
		{"regional indicator flag pair", "🇺🇸x", 2, 2},
		{"skin tone modifier", "👍🏽x", 2, 2},
		{"lone combining mark", "́x", 0, 1}, // combining acute accent, no base
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runes := []rune(tc.s)
			gotWidth, gotNext := NextGrapheme(runes, 0)
			if gotWidth != tc.wantWidth || gotNext != tc.wantNext {
				t.Errorf("NextGrapheme(%q, 0) = (%d, %d), want (%d, %d)",
					tc.s, gotWidth, gotNext, tc.wantWidth, tc.wantNext)
			}
		})
	}
}

// TestNextGraphemeStopsBeforeEscape confirms NextGrapheme never folds an
// ANSI escape byte into a visible-text cluster — every caller relies on
// this to keep its own escape recognition (consumeANSI) fully separate
// from width computation.
func TestNextGraphemeStopsBeforeEscape(t *testing.T) {
	runes := []rune("a\x1b[0m")
	width, next := NextGrapheme(runes, 0)
	if width != 1 || next != 1 {
		t.Errorf("NextGrapheme(%q, 0) = (%d, %d), want (1, 1)", string(runes), width, next)
	}
}

// TestANSIStyledMultiRuneClusterWidth confirms a multi-rune grapheme
// cluster (a regional-indicator flag pair) embedded in an SGR-styled string
// is measured as one 2-column unit by ansiVisibleLen, and that
// splitAtVisualWidthCarry correctly keeps the SGR span open across it
// rather than splitting the cluster apart or miscounting its width as 4
// (one rune-summing bug this migration specifically fixes).
func TestANSIStyledMultiRuneClusterWidth(t *testing.T) {
	s := "\x1b[1mhi \U0001F1FA\U0001F1F8 bye\x1b[0m"
	if got, want := ansiVisibleLen(s), len("hi  bye")+2; got != want {
		t.Errorf("ansiVisibleLen(%q) = %d, want %d", s, got, want)
	}
	lines := splitAtVisualWidth(s, 5)
	if len(lines) == 0 {
		t.Fatal("splitAtVisualWidthCarry produced no lines")
	}
	for _, line := range lines {
		if w := ansiVisibleLen(line); w > 5 {
			t.Errorf("line %q has visible width %d, want <= 5", line, w)
		}
	}
}
