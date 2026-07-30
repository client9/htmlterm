package textcell

import (
	"reflect"
	"testing"
)

func TestSplitANSITokens(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x07"
	oscEnd := "\x1b]8;;\x07"
	text := "pre " + osc + "link text" + oscEnd + " post\t\x1b[31mred\x1b[0m"
	want := []string{
		"pre",
		osc + "link",
		"text" + oscEnd,
		"post",
		"\x1b[31mred\x1b[0m",
	}
	if got := SplitTokens(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitTokens() = %#v, want %#v", got, want)
	}
}

func TestStripANSI(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x1b\\"
	oscEnd := "\x1b]8;;\x1b\\"
	text := "a\x1b[31mred\x1b[0m" + osc + "link" + oscEnd + "b"
	if got := Strip(text); got != "aredlinkb" {
		t.Fatalf("Strip() = %q, want %q", got, "aredlinkb")
	}
}

func TestTrimOneTrailingVisibleSpace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain trailing space", "hi ", "hi", true},
		{"no trailing space", "hi", "hi", false},
		{"styled trailing space removed, escapes preserved", "\x1b[31mhi \x1b[0m", "\x1b[31mhi\x1b[0m", true},
		{"two trailing spaces only removes one", "hi  ", "hi ", true},
		{"space before a trailing escape sequence still counts as trailing", "hi \x1b[0m", "hi\x1b[0m", true},
	}
	for _, c := range cases {
		got, ok := TrimTrailingSpace(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: TrimTrailingSpace(%q) = (%q, %v), want (%q, %v)", c.name, c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestANSIIntermediateByte verifies that two-byte escape sequences whose first
// byte is an intermediate (0x20–0x3F, e.g. ESC '(' for ISO 2022 G0 charset)
// are fully consumed and do not leak their final byte as visible content.
func TestANSIIntermediateByte(t *testing.T) {
	// ESC '(' 'B' — G0 charset designation to US-ASCII (ISO 2022).
	seq := "\x1b(B"

	// VisibleLen must not count the final byte 'B' as visible.
	if got := VisibleLen(seq); got != 0 {
		t.Errorf("VisibleLen(ESC ( B) = %d, want 0", got)
	}
	// Strip must consume the whole 3-byte sequence, producing no output.
	if got := Strip(seq); got != "" {
		t.Errorf("Strip(ESC ( B) = %q, want %q", got, "")
	}

	// Mixed: visible text around the sequence.
	mixed := "ab" + seq + "cd"
	if got := VisibleLen(mixed); got != 4 {
		t.Errorf("VisibleLen(mixed) = %d, want 4", got)
	}
	if got := Strip(mixed); got != "abcd" {
		t.Errorf("Strip(mixed) = %q, want %q", got, "abcd")
	}
	// SplitTokens must produce one token containing the word with the
	// sequence attached — the sequence must not cause an extra word split.
	if got := SplitTokens("hi" + seq + " there"); !reflect.DeepEqual(got, []string{"hi" + seq, "there"}) {
		t.Errorf("SplitTokens with intermediate escape = %#v", got)
	}
}

func TestSplitAtVisualWidthEdgeCases(t *testing.T) {
	// Empty string early return
	got := splitAtWidth("", 5)
	if !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("splitAtWidth(%q, 5) = %#v, want [%q]", "", got, "")
	}

	// OSC hyperlink sequences don't count toward visible width and are preserved
	osc := "\x1b]8;;https://example.com\x07"
	// "abc" (3 visible) + OSC + "def" (3 visible), split at width 3
	text := "abc" + osc + "def"
	lines := splitAtWidth(text, 3)
	if len(lines) != 2 {
		t.Fatalf("OSC split: want 2 lines, got %d: %#v", len(lines), lines)
	}
	if v := VisibleLen(lines[0]); v != 3 {
		t.Errorf("OSC split lines[0] visible len = %d, want 3", v)
	}
	if v := Strip(lines[0]); v != "abc" {
		t.Errorf("OSC split lines[0] text = %q, want %q", v, "abc")
	}
	// The hyperlink span is reopened on the continuation line so it stays
	// active across the break (see Carry).
	if lines[1] != osc+"def" {
		t.Errorf("OSC split lines[1] = %q, want %q", lines[1], osc+"def")
	}
}

func TestSpliceColumns(t *testing.T) {
	t.Run("plain text interior splice", func(t *testing.T) {
		got := SpliceColumns("0123456789", 3, 4, "XXXX")
		want := "012XXXX789"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("splice at column 0", func(t *testing.T) {
		got := SpliceColumns("0123456789", 0, 3, "XXX")
		want := "XXX3456789"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("splice reaching past line's end preserves nothing after", func(t *testing.T) {
		got := SpliceColumns("012345", 3, 10, "XXXXXXXXXX")
		want := "012XXXXXXXXXX"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("splice starting past line's end pads with spaces", func(t *testing.T) {
		got := SpliceColumns("01", 4, 2, "XX")
		want := "01  XX"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("width<=0 or negative col is a no-op", func(t *testing.T) {
		if got := SpliceColumns("hello", 2, 0, "X"); got != "hello" {
			t.Errorf("width=0: SpliceColumns = %q, want unchanged %q", got, "hello")
		}
		if got := SpliceColumns("hello", -1, 2, "X"); got != "hello" {
			t.Errorf("col<0: SpliceColumns = %q, want unchanged %q", got, "hello")
		}
	})

	t.Run("a span that closes before the cut region needs no reopening", func(t *testing.T) {
		bold := "\x1b[1m"
		reset := "\x1b[m"
		// The bold span closes right after "012", well before the cut
		// region [3,7) even starts, so the suffix "789" was never bold in
		// the original and shouldn't be reopened as bold either.
		line := bold + "012" + reset + "3456789"
		got := SpliceColumns(line, 3, 4, "XXXX")
		want := bold + "012" + reset + "XXXX" + "789"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("a span spanning the whole cut region is reopened for the resuming suffix", func(t *testing.T) {
		bold := "\x1b[1m"
		reset := "\x1b[m"
		// The bold span opens before col 3 and stays open across the whole
		// cut region [3,7) and into the resuming suffix, only closing at
		// the very end — without carry-through, "789" would resume
		// unstyled even though the original span was still active there.
		line := "012" + bold + "3456789" + reset
		got := SpliceColumns(line, 3, 4, "XXXX")
		want := "012" + "XXXX" + bold + "789" + reset
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})

	t.Run("replacement content is inserted verbatim, including its own styling", func(t *testing.T) {
		styled := "\x1b[31mpopup\x1b[m"
		got := SpliceColumns("0123456789", 3, 5, styled)
		want := "012" + styled + "89"
		if got != want {
			t.Errorf("SpliceColumns = %q, want %q", got, want)
		}
	})
}

// TestColumnRuneIndexRoundTrip pins ColumnForRuneIndex/RuneIndexForColumn as
// exact inverses at every cluster boundary, including double-width CJK and a
// multi-rune ZWJ emoji cluster. Callers on both sides of the caret (tui's
// focusCursorPos placing the hardware cursor, document's caretIndexFromClick
// turning a click back into an offset) depend on that: a click must produce a
// caret whose cursor lands back on the cell that was clicked.
func TestColumnRuneIndexRoundTrip(t *testing.T) {
	const family = "\U0001F468‍\U0001F469‍\U0001F467" // 👨‍👩‍👧, one cluster
	for _, s := range []string{"", "abc", "日本語", "a日b", "x" + family + "y"} {
		runes := []rune(s)
		for idx := 0; idx <= len(runes); idx++ {
			col := ColumnForRuneIndex(s, idx)
			back := RuneIndexForColumn(s, col)
			// back is idx whenever idx is a cluster boundary; when it isn't,
			// ColumnForRuneIndex already rounded down to the cluster start,
			// so the round trip must land at or before idx and re-produce the
			// same column.
			if back > idx {
				t.Errorf("%q idx %d: col %d round-tripped to %d, past the original", s, idx, col, back)
			}
			if got := ColumnForRuneIndex(s, back); got != col {
				t.Errorf("%q idx %d: round trip changed column %d -> %d", s, idx, col, got)
			}
		}
	}
}

// TestColumnForRuneIndexWideRunes pins the concrete column arithmetic the
// round-trip test above can't (it only checks self-consistency).
func TestColumnForRuneIndexWideRunes(t *testing.T) {
	const s = "日本語ab"
	for _, tc := range []struct{ idx, want int }{{0, 0}, {1, 2}, {2, 4}, {3, 6}, {4, 7}, {5, 8}, {99, 8}} {
		if got := ColumnForRuneIndex(s, tc.idx); got != tc.want {
			t.Errorf("ColumnForRuneIndex(%q, %d) = %d, want %d", s, tc.idx, got, tc.want)
		}
	}
	// Either cell of a double-width character maps back to the rune before it.
	for _, tc := range []struct{ col, want int }{{0, 0}, {1, 0}, {2, 1}, {3, 1}, {6, 3}, {7, 4}, {8, 5}, {99, 5}} {
		if got := RuneIndexForColumn(s, tc.col); got != tc.want {
			t.Errorf("RuneIndexForColumn(%q, %d) = %d, want %d", s, tc.col, got, tc.want)
		}
	}
}

// splitAtWidth hard-splits s into chunks of at most width visible columns,
// starting from an empty ANSI carry — the no-carry shape these tests use.
// It used to exist in internal/render as splitAtVisualWidth, kept alive solely
// by tests after its last production caller went away; it lives here now so
// the production package carries no test-only wrapper.
func splitAtWidth(s string, width int) []string {
	lines, _ := SplitAtWidth(s, width, Carry{})
	return lines
}

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
// this to keep its own escape recognition (ConsumeANSI) fully separate
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
// is measured as one 2-column unit by VisibleLen, and that
// SplitAtWidth correctly keeps the SGR span open across it
// rather than splitting the cluster apart or miscounting its width as 4
// (one rune-summing bug this migration specifically fixes).
func TestANSIStyledMultiRuneClusterWidth(t *testing.T) {
	s := "\x1b[1mhi \U0001F1FA\U0001F1F8 bye\x1b[0m"
	if got, want := VisibleLen(s), len("hi  bye")+2; got != want {
		t.Errorf("VisibleLen(%q) = %d, want %d", s, got, want)
	}
	lines := splitAtWidth(s, 5)
	if len(lines) == 0 {
		t.Fatal("SplitAtWidth produced no lines")
	}
	for _, line := range lines {
		if w := VisibleLen(line); w > 5 {
			t.Errorf("line %q has visible width %d, want <= 5", line, w)
		}
	}
}

// Reproduces the truncation-boundary bug: VisiblePrefix
// (and TruncateToWidth, which calls it) checks "visible < width" BEFORE
// adding the next rune's width, so a wide (2-column) rune landing exactly on
// the boundary gets included even though doing so overshoots width by 1.
func TestWideRuneAtTruncationBoundary(t *testing.T) {
	s := "123456789012345678🎉XXXX" // 18 ascii cols, then a wide emoji, then filler
	got := TruncateToWidth(s, 19, "")
	gotWidth := VisibleLen(got)
	if gotWidth > 19 {
		t.Errorf("TruncateToWidth(%q, 19) = %q with visible width %d, want <= 19", s, got, gotWidth)
	}
}
