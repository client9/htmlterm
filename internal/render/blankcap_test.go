package render

import (
	"reflect"
	"testing"
)

func TestCapBlankRunsDisabledPassesThrough(t *testing.T) {
	lines := []string{"A", "", "", "", "B"}
	got, _ := capBlankRuns(lines, nil, 0)
	if !reflect.DeepEqual(got, lines) {
		t.Errorf("got %#v, want unchanged %#v", got, lines)
	}
}

func TestCapBlankRunsCapsToMaxBlanks1(t *testing.T) {
	lines := []string{"A", "", "", "", "", "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsCapsToMaxBlanks2(t *testing.T) {
	lines := []string{"A", "", "", "", "B"}
	got, _ := capBlankRuns(lines, nil, 2)
	want := []string{"A", "", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsRunNotExceedingCapStillBlanksContent(t *testing.T) {
	// Even when a run doesn't exceed the cap, its content is replaced with
	// "" — matching cappedWriter's flushNLBuf, which only ever tracked a
	// pending newline count, never a blank line's original bytes.
	lines := []string{"A", "   ", "B"}
	got, _ := capBlankRuns(lines, nil, 5)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsSpaceLineCountsAsBlank(t *testing.T) {
	lines := []string{"A", " ", "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsNBSPCountsAsBlank(t *testing.T) {
	lines := []string{"A", " ", " ", " ", "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsNBSPPreservedWhenDisabled(t *testing.T) {
	lines := []string{"A", " ", "B"}
	got, _ := capBlankRuns(lines, nil, 0)
	want := []string{"A", " ", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsMixedUnicodeSpacesBlank(t *testing.T) {
	lines := []string{"A", "   ", "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// TestCapBlankRunsPreLinesExemptAndBreakRuns verifies pre-tagged lines are
// never collapsed or counted as part of a blank run (regardless of how many
// consecutive pre-tagged lines are themselves blank), and that they split
// the blank runs on either side of them into separate runs, each capped
// independently — the box.pre-based replacement for cappedWriter's preDepth
// exemption.
func TestCapBlankRunsPreLinesExemptAndBreakRuns(t *testing.T) {
	lines := []string{"A", "", "", "", "pre1", "", "", "pre2", "", "", "", "B"}
	pre := []bool{false, false, false, false, true, true, true, true, false, false, false, false}
	got, _ := capBlankRuns(lines, pre, 1)
	want := []string{"A", "", "pre1", "", "", "pre2", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCapBlankRunsNilPreTreatsAllLinesAsCappable(t *testing.T) {
	lines := []string{"A", "", "", "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestIsBlankLineIgnoresOSC8Hyperlink(t *testing.T) {
	// wrapHyperlinkBox wraps an otherwise-empty line in a zero-width OSC8
	// open/reset pair (e.g. a link with no visible text); it must still
	// count as blank, or -no-osc8-links and default output disagree on how
	// many blank lines a run collapses to.
	line := "\x1b]8;;https://example.com\x1b\\\x1b]8;;\x1b\\"
	if !isBlankLine(line) {
		t.Errorf("isBlankLine(%q) = false, want true", line)
	}
}

func TestIsBlankLineIgnoresSGROnly(t *testing.T) {
	line := "\x1b[4m\x1b[m"
	if !isBlankLine(line) {
		t.Errorf("isBlankLine(%q) = false, want true", line)
	}
}

func TestIsBlankLineNotBlankWithVisibleTextInsideEscapes(t *testing.T) {
	line := "\x1b[4mhi\x1b[m"
	if isBlankLine(line) {
		t.Errorf("isBlankLine(%q) = true, want false", line)
	}
}

func TestCapBlankRunsOSC8OnlyLinesCollapse(t *testing.T) {
	// Regression test: a run of lines that are visually blank but each
	// wrapped in a zero-width OSC8 hyperlink sequence must collapse exactly
	// like the same run without any hyperlinks does.
	osc8Blank := "\x1b]8;;https://example.com\x1b\\\x1b]8;;\x1b\\"
	lines := []string{"A", osc8Blank, osc8Blank, osc8Blank, "B"}
	got, _ := capBlankRuns(lines, nil, 1)
	want := []string{"A", "", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
