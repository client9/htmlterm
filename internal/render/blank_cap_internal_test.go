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
