package render

import (
	"strings"
	"testing"
)

func TestApplyRelativeOffsetsShiftsContentAndBlanksOriginal(t *testing.T) {
	// Height:10 pads the document with blank rows below the two divs, giving
	// the top:1 shift somewhere to land without being clamped back to 0 —
	// applyRelativeOffsets deliberately never grows the canvas (see its doc
	// comment), so a 2-line document alone would clamp the shift away.
	e, err := New(Options{Width: 20, Height: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:relative;top:1;left:3">AB</div><div>XY</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) < 3 {
		t.Fatalf("got %d lines, want >= 3:\n%q", len(lines), result.Output)
	}
	// Original slot (row 0) must be blanked, not "AB".
	if strings.TrimRight(lines[0], " ") != "" {
		t.Errorf("row 0 (original slot) = %q, want blank", lines[0])
	}
	// Sibling <div>XY</div> stays on row 1, unshifted.
	if !strings.HasPrefix(lines[1], "XY") {
		t.Errorf("row 1 = %q, want to start with XY", lines[1])
	}
	// Shifted content lands one row down from its original slot (row 0+1=1
	// is already XY's row, so AB ends up spliced onto that same row), three
	// columns right of its original column.
	if !strings.Contains(lines[1], "AB") {
		t.Errorf("row 1 = %q, want to contain shifted AB", lines[1])
	}
}

func TestApplyRelativeOffsetsUpdatesPositionMap(t *testing.T) {
	e, err := New(Options{Width: 20, Height: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div id="a" style="position:relative;top:2;left:4">Hi</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var found bool
	for n, rect := range result.Positions {
		if n.Data == "div" {
			found = true
			if rect.Row != 2 || rect.Col != 4 {
				t.Errorf("div Rect = %+v, want Row=2 Col=4", rect)
			}
		}
	}
	if !found {
		t.Fatal("no div node found in Positions")
	}
}

func TestApplyRelativeOffsetsNoOffsetIsNoop(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	withRelative, err := e.RenderHTML(`<div style="position:relative">Hi</div>`)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	plain, err := e.RenderHTML(`<div>Hi</div>`)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if withRelative.Output != plain.Output {
		t.Errorf("position:relative with no offset changed output:\ngot:  %q\nwant: %q", withRelative.Output, plain.Output)
	}
}

func TestApplyRelativeOffsetsBottomAndRightPrecedence(t *testing.T) {
	e, err := New(Options{Width: 20, Height: 5})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// bottom:1 shifts up by 1 (negative dRow); right:2 shifts left by 2.
	src := `<div style="position:relative;bottom:1;right:2">Hi</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" {
			rect = r
		}
	}
	if rect.Row != 0 || rect.Col != 0 {
		// Started at Row=0,Col=0 (first element); bottom:1 -> -1 clamped to 0,
		// right:2 -> -2 clamped to 0.
		t.Errorf("Rect = %+v, want clamped to Row=0 Col=0", rect)
	}
}

func TestApplyRelativeOffsetsTopWinsOverBottom(t *testing.T) {
	e, err := New(Options{Width: 20, Height: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:relative;top:2;bottom:5">Hi</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" {
			rect = r
		}
	}
	if rect.Row != 2 {
		t.Errorf("Rect.Row = %d, want 2 (top wins over bottom)", rect.Row)
	}
}

func TestApplyRelativeOffsetsShiftsDescendants(t *testing.T) {
	e, err := New(Options{Width: 20, Height: 10, FocusAttr: "data-focus"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:relative;top:1;left:2"><input data-focus size="1"></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var inputRect Rect
	found := false
	for n, r := range result.Positions {
		if n.Data == "input" {
			inputRect = r
			found = true
		}
	}
	if !found {
		t.Fatal("no input node found in Positions")
	}
	if inputRect.Row != 1 || inputRect.Col != 2 {
		t.Errorf("input Rect = %+v, want shifted by (1,2)", inputRect)
	}
}

func TestApplyRelativeOffsetsPercentage(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:relative;left:50%">Hi</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" {
			rect = r
		}
	}
	if rect.Col != 10 {
		t.Errorf("Rect.Col = %d, want 10 (50%% of width 20)", rect.Col)
	}
}
