package document_test

import (
	"testing"

	"github.com/client9/htmlterm/document"
)

// TestTableCollapseModeHitTesting is an end-to-end regression for
// border-collapse:collapse's position tracking (internal/render/
// table_collapse.go's composeCollapsedGrid): a clickable element nested
// inside a collapsed-grid table cell must still resolve to the correct
// Rect and be hit-testable via DispatchClick, the same way it would inside
// any other bordered block box.
func TestTableCollapseModeHitTesting(t *testing.T) {
	doc := mustParseDoc(t, `<style>
table { border-collapse: collapse; }
td { border: solid; }
</style>
<table><tr><td>A</td><td><button id="btn">Click</button></td></tr></table>`)

	btn := doc.GetElementByID("btn")
	rect, ok := btn.Rect()
	if !ok {
		t.Fatalf("no Rect recorded for the button nested in a collapse-mode cell")
	}

	clicked := 0
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { clicked++ })

	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("click at the button's own Rect (%+v) did not hit it", rect)
	}
	if clicked != 1 {
		t.Errorf("click fired %d times, want 1", clicked)
	}
}
