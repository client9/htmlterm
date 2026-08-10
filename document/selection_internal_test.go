package document

import "testing"

func TestSelectionDefaultsToCollapsedAtEnd(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")
	if el == nil {
		t.Fatal("input not found")
	}
	if got := el.SelectionStart(); got != 5 {
		t.Errorf("SelectionStart() = %d, want 5 (end of \"hello\")", got)
	}
	if got := el.SelectionEnd(); got != 5 {
		t.Errorf("SelectionEnd() = %d, want 5", got)
	}
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() = %q, want \"none\"", got)
	}
}

func TestSetSelectionRangeClampsAndSwapsAndDirection(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")

	// Inverted range gets swapped, matching real setSelectionRange.
	el.SetSelectionRange(4, 1, "backward")
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 1 || end != 4 {
		t.Errorf("after SetSelectionRange(4, 1): start=%d end=%d, want 1,4", start, end)
	}
	if got := el.SelectionDirection(); got != "backward" {
		t.Errorf("SelectionDirection() = %q, want \"backward\"", got)
	}

	// Out-of-range offsets clamp to [0, len(value)] (5 runes).
	el.SetSelectionRange(-3, 99)
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 0 || end != 5 {
		t.Errorf("after SetSelectionRange(-3, 99): start=%d end=%d, want 0,5", start, end)
	}
	// Direction omitted normalizes to "none".
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() = %q, want \"none\"", got)
	}

	// Unrecognized direction also normalizes to "none".
	el.SetSelectionRange(0, 2, "sideways")
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() with bogus direction = %q, want \"none\"", got)
	}
}

func TestSetValueClearsSelection(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")
	el.SetSelectionRange(1, 2)
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 1 || end != 2 {
		t.Fatalf("precondition failed: start=%d end=%d, want 1,2", start, end)
	}

	el.SetValue("hi")
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 2 || end != 2 {
		t.Errorf("after SetValue: start=%d end=%d, want collapsed at 2 (end of \"hi\")", start, end)
	}
}
