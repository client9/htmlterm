package render

import (
	"strings"
	"testing"
)

func TestOutOfFlowFixedAnchorsToViewportIgnoringPositionedAncestor(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:relative;top:5;left:5;width:10">` +
		`<div style="position:fixed;top:0;left:0;width:4">FX</div></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
		}
	}
	if rect.Row != 0 || rect.Col != 0 {
		t.Errorf("fixed element Rect = %+v, want Row=0 Col=0 (viewport-anchored, ignoring the relative ancestor)", rect)
	}
}

func TestOutOfFlowAbsoluteAnchorsToNearestPositionedAncestor(t *testing.T) {
	// applyRelativeOffsets (position: relative) clamps rather than grows the
	// canvas — see position.go's doc comment — so enough trailing sibling
	// lines are needed here to give the outer container's top:3 shift
	// somewhere to land without being clamped back to 0.
	e, err := New(Options{Width: 40, Height: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div id="outer" style="position:relative;top:3;left:2;width:20">CONTAINER` +
		`<div style="position:absolute;top:1;left:1;width:4">AB</div></div>` +
		`<div>t1</div><div>t2</div><div>t3</div><div>t4</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	found := false
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
			found = true
		}
	}
	if !found {
		t.Fatal("no absolute div found in Positions")
	}
	// outer's own Rect starts at (0,0), shifted by top:3/left:2 to (3,2);
	// the absolute child's containing block is that shifted Rect, so
	// top:1/left:1 resolves to (3+1, 2+1) = (4,3).
	if rect.Row != 4 || rect.Col != 3 {
		t.Errorf("absolute element Rect = %+v, want Row=4 Col=3 (relative to the shifted containing block)", rect)
	}
}

func TestOutOfFlowAbsoluteFallsBackToDocumentRoot(t *testing.T) {
	e, err := New(Options{Width: 40})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// No positioned ancestor anywhere — falls back to the whole document.
	src := `<div><div style="position:absolute;top:2;left:3;width:4">AB</div></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
		}
	}
	if rect.Row != 2 || rect.Col != 3 {
		t.Errorf("Rect = %+v, want Row=2 Col=3 (document-root-anchored)", rect)
	}
}

func TestOutOfFlowAbsoluteInsideAbsoluteDependencyOrdering(t *testing.T) {
	e, err := New(Options{Width: 40})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// The outer div is itself out-of-flow (absolute), so the inner
	// absolute's containing block only exists after Phase A resolves the
	// outer one first — exercises collectOutOfFlow's preorder guarantee.
	src := `<div style="position:absolute;top:2;left:2;width:10">` +
		`<div style="position:absolute;top:1;left:1;width:4">AB</div></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
		}
	}
	if rect.Row != 3 || rect.Col != 3 {
		t.Errorf("inner absolute Rect = %+v, want Row=3 Col=3 ((2,2)+(1,1))", rect)
	}
}

func TestOutOfFlowBottomRightResolution(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Containing block is the 20-wide viewport; bottom:1/right:2 with no
	// top/left set anchors the box's trailing edges instead.
	src := `<div style="position:absolute;bottom:1;right:2;width:4">AB</div>` +
		`<div>x</div><div>y</div><div>z</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
		}
	}
	// Document height is 3 (x/y/z); box height 1: row = 3-1-1 = 1.
	// Viewport width 20; box width 4: col = 20-4-2 = 14.
	if rect.Row != 1 || rect.Col != 14 {
		t.Errorf("Rect = %+v, want Row=1 Col=14", rect)
	}
}

func TestOutOfFlowZIndexPaintOrder(t *testing.T) {
	e, err := New(Options{Width: 40})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:absolute;top:0;left:0;width:6;z-index:1">back</div>` +
		`<div style="position:absolute;top:0;left:2;width:6;z-index:2">FRONT</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "FRONT") {
		t.Fatalf("row 0 = %q, want to contain FRONT (higher z-index paints last)", lines[0])
	}
	if strings.Contains(lines[0], "back") {
		t.Errorf("row 0 = %q, want back's overlapped cells overwritten by FRONT", lines[0])
	}
}

func TestOutOfFlowReservesNoSpaceInNormalFlow(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div>A</div><div style="position:absolute;top:0;left:0">X</div><div>B</div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	// Normal flow is just A then B (2 lines) — the absolute element
	// contributes no line of its own. It then paints over row 0 (no
	// positioned ancestor, top:0/left:0 -> document root).
	if len(lines) < 2 || strings.TrimRight(lines[1], " ") != "B" {
		t.Fatalf("lines = %q, want B on row 1 (unaffected by the out-of-flow element between A and B)", lines)
	}
	if !strings.HasPrefix(lines[0], "X") {
		t.Errorf("row 0 = %q, want to start with X (painted over A's slot)", lines[0])
	}
}

func TestOutOfFlowColumnClampedToViewport(t *testing.T) {
	e, err := New(Options{Width: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:absolute;top:0;left:8;width:5">ABCDE</div>`
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
	if rect.Col != 5 {
		t.Errorf("Rect.Col = %d, want 5 (clamped so a 5-wide box fits in a 10-wide viewport)", rect.Col)
	}
}

// TestOutOfFlowAppliesRelativeOffsetsToNestedDescendants regression-tests a
// bug where a position: relative element nested inside a position:
// absolute/fixed subtree never shifted at all: the outer element's whole
// subtree is skipped during normal layout (see outofflow.go's out-of-flow
// removal), so Engine.RenderNode's top-level applyRelativeOffsets call never
// sees the nested relative element — applyOutOfFlow's Phase A must apply it
// separately to each out-of-flow element's own freshly rendered box.
func TestOutOfFlowAppliesRelativeOffsetsToNestedDescendants(t *testing.T) {
	e, err := New(Options{Width: 30})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src := `<div style="position:absolute;top:0;left:0;width:20">outer` +
		`<div style="position:relative;top:0;left:5">SHIFTED</div></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "     SHIFTED") {
		t.Fatalf("lines = %q, want row 1 to start with 5 spaces then SHIFTED (left:5 applied)", lines)
	}
}

// TestOutOfFlowDescendantPositionsAgainstClampedAncestor regression-tests a
// bug where a nested absolute element's containing block used its
// ancestor's pre-clamp geometry: an ancestor whose own computed column
// needed clamping to fit the viewport (clampOutOfFlowRect) would still hand
// its unclamped column to a descendant's containing-block lookup, so the
// descendant landed nowhere near where the ancestor actually painted.
func TestOutOfFlowDescendantPositionsAgainstClampedAncestor(t *testing.T) {
	e, err := New(Options{Width: 12})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Viewport is 12 wide; the outer box (width 10, left:15) must clamp to
	// col 2. The inner box (left:0) should land at the outer's *clamped*
	// column, not its pre-clamp column (15).
	src := `<div style="position:absolute;top:0;left:15;width:10">` +
		`<div style="position:absolute;top:0;left:0;width:4">IN</div></div>`
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var rect Rect
	for n, r := range result.Positions {
		if n.Data == "div" && r.Width == 4 {
			rect = r
		}
	}
	if rect.Col != 2 {
		t.Errorf("inner Rect.Col = %d, want 2 (the outer's actual clamped column, not its pre-clamp column 15)", rect.Col)
	}
}
