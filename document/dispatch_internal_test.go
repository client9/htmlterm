package document

import "testing"

func TestDocumentElementResizeDispatch(t *testing.T) {
	doc, err := ParseDocument(`<p>hi</p>`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	fired := false
	doc.AddEventListener(doc.DocumentElement(), "resize", false, func(e *Event) {
		fired = true
		if e.Type != "resize" {
			t.Errorf("Event.Type = %q, want %q", e.Type, "resize")
		}
	})
	doc.dispatch(doc.doc, "resize", "", Modifiers{})
	if !fired {
		t.Error("resize listener did not fire")
	}
}
