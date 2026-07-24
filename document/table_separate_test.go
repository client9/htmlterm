package document_test

import (
	"testing"

	"github.com/client9/htmlterm/document"
)

// TestTableSeparateModeHitTesting is an end-to-end regression for
// border-collapse:separate's position tracking (internal/render/
// table_separate.go's composeSeparateGrid): a clickable element nested
// inside a per-cell-bordered table cell must still resolve to the correct
// Rect and be hit-testable via DispatchClick, the same way it would inside
// any other bordered block box.
func TestTableSeparateModeHitTesting(t *testing.T) {
	doc := mustParseDoc(t, `<style>
table { border-collapse: separate; border-spacing: 1; border: solid; padding: 1; }
td { border: solid; }
</style>
<table><tr><td>A</td><td><button id="btn">Click</button></td></tr></table>`)

	btn := doc.GetElementByID("btn")
	rect, ok := btn.Rect()
	if !ok {
		t.Fatalf("no Rect recorded for the button nested in a separate-mode cell")
	}

	clicked := 0
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { clicked++ })

	if !doc.DispatchClick(rect.Row, rect.Col) {
		t.Fatalf("click at the button's own Rect (%+v) did not hit it", rect)
	}
	if clicked != 1 {
		t.Errorf("click fired %d times, want 1", clicked)
	}
}
