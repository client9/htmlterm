package document

import "testing"

// TestPruneDetachedStateDropsSelectionsAndValueBaselines pins that per-node
// state for nodes cut out of the tree is swept, not leaked. d.selections and
// d.valueAtFocus are the two maps Render does *not* rebuild each frame (see
// pruneDetachedState), so before this a long-running Loop that repeatedly
// refreshed a container via SetInnerHTML accumulated one entry per text entry
// it ever rendered.
func TestPruneDetachedStateDropsSelectionsAndValueBaselines(t *testing.T) {
	doc, err := ParseDocument(`<div id="pane"><input id="a" value="hello"></div><input id="keep" value="x">`,
		Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Give both inputs recorded selection state, and both focus baselines.
	a := doc.GetElementByID("a")
	keep := doc.GetElementByID("keep")
	a.Focus()
	a.SetSelectionRange(1, 3)
	keep.Focus() // focusing keep snapshots its value and blurs a
	keep.SetSelectionRange(0, 1)

	if len(doc.selections) != 2 {
		t.Fatalf("selections = %d entries, want 2 before the removal", len(doc.selections))
	}

	if err := doc.GetElementByID("pane").SetInnerHTML(`<p>replaced</p>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}

	if len(doc.selections) != 1 {
		t.Errorf("selections = %d entries after detaching one input, want 1", len(doc.selections))
	}
	if _, ok := doc.selections[keep.node]; !ok {
		t.Error("the still-attached input's selection was swept too, want it kept")
	}
	for n := range doc.valueAtFocus {
		if !isDescendant(doc.doc, n) {
			t.Error("valueAtFocus still holds an entry for a detached node")
		}
	}
	// The surviving input keeps working — the sweep didn't disturb live state.
	if start, end := keep.SelectionStart(), keep.SelectionEnd(); start != 0 || end != 1 {
		t.Errorf("surviving input's selection = [%d,%d), want [0,1)", start, end)
	}
}

// TestPruneDetachedStateViaRemoveChild pins that the sweep runs on the
// element-level tree mutations too, not just SetInnerHTML.
func TestPruneDetachedStateViaRemoveChild(t *testing.T) {
	doc, err := ParseDocument(`<div id="pane"><input id="a" value="hello"></div>`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 3)

	doc.GetElementByID("pane").RemoveChild(a)

	if len(doc.selections) != 0 {
		t.Errorf("selections = %d entries after RemoveChild, want 0", len(doc.selections))
	}
	if doc.focused != nil {
		t.Error("focus still points at the removed node")
	}
}
