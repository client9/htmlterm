package document_test

import (
	"testing"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
)

func TestStyleGetPropertyValue(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d" style="color: red; background-color: blue">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	if el == nil {
		t.Fatal("GetElementByID(d) = nil")
	}
	if got, want := el.Style().GetPropertyValue("color"), "red"; got != want {
		t.Errorf("GetPropertyValue(color) = %q, want %q", got, want)
	}
	if got, want := el.Style().GetPropertyValue("COLOR"), "red"; got != want {
		t.Errorf("GetPropertyValue(COLOR) = %q, want %q (case-insensitive)", got, want)
	}
	if got, want := el.Style().GetPropertyValue("background-color"), "blue"; got != want {
		t.Errorf("GetPropertyValue(background-color) = %q, want %q", got, want)
	}
	if got := el.Style().GetPropertyValue("font-weight"); got != "" {
		t.Errorf("GetPropertyValue(font-weight) = %q, want \"\"", got)
	}
}

func TestStyleSetProperty(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	style := el.Style()

	style.SetProperty("color", "red")
	if got, want := style.GetPropertyValue("color"), "red"; got != want {
		t.Errorf("after SetProperty(color, red): GetPropertyValue(color) = %q, want %q", got, want)
	}

	style.SetProperty("background-color", "blue")
	if got, want := style.GetPropertyValue("color"), "red"; got != want {
		t.Errorf("existing property clobbered: GetPropertyValue(color) = %q, want %q", got, want)
	}
	if got, want := style.GetPropertyValue("background-color"), "blue"; got != want {
		t.Errorf("GetPropertyValue(background-color) = %q, want %q", got, want)
	}

	// Overwriting an existing property replaces its value.
	style.SetProperty("color", "green")
	if got, want := style.GetPropertyValue("color"), "green"; got != want {
		t.Errorf("after overwrite: GetPropertyValue(color) = %q, want %q", got, want)
	}

	// SetProperty with an empty value removes it, per spec.
	style.SetProperty("color", "")
	if got := style.GetPropertyValue("color"); got != "" {
		t.Errorf("after SetProperty(color, \"\"): GetPropertyValue(color) = %q, want \"\"", got)
	}
	if got, want := style.GetPropertyValue("background-color"), "blue"; got != want {
		t.Errorf("unrelated property disturbed: GetPropertyValue(background-color) = %q, want %q", got, want)
	}
}

func TestStyleSetPropertyExpandsShorthand(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	style := el.Style()

	style.SetProperty("margin", "1")
	if got, want := style.GetPropertyValue("margin-top"), "1"; got != want {
		t.Errorf("GetPropertyValue(margin-top) = %q, want %q", got, want)
	}
	if got, want := style.GetPropertyValue("margin-left"), "1"; got != want {
		t.Errorf("GetPropertyValue(margin-left) = %q, want %q", got, want)
	}
	// Shorthand names don't round-trip — only longhands are stored.
	if got := style.GetPropertyValue("margin"); got != "" {
		t.Errorf("GetPropertyValue(margin) = %q, want \"\" (shorthand not stored)", got)
	}
}

func TestStyleRemoveProperty(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d" style="color: red; background-color: blue">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	style := el.Style()

	old := style.RemoveProperty("color")
	if old != "red" {
		t.Errorf("RemoveProperty(color) returned %q, want %q", old, "red")
	}
	if got := style.GetPropertyValue("color"); got != "" {
		t.Errorf("after RemoveProperty(color): GetPropertyValue(color) = %q, want \"\"", got)
	}
	if got, want := style.GetPropertyValue("background-color"), "blue"; got != want {
		t.Errorf("unrelated property disturbed: GetPropertyValue(background-color) = %q, want %q", got, want)
	}

	// Removing an absent property is a no-op returning "".
	if got := style.RemoveProperty("font-weight"); got != "" {
		t.Errorf("RemoveProperty(font-weight) = %q, want \"\"", got)
	}
}

func TestStyleCSSText(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d" style="color: red; background-color: blue">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	style := el.Style()

	// Sorted by property name, not source order.
	if got, want := style.CSSText(), "background-color: blue; color: red;"; got != want {
		t.Errorf("CSSText() = %q, want %q", got, want)
	}

	style.SetCSSText("font-weight: bold")
	if got, want := style.GetPropertyValue("font-weight"), "bold"; got != want {
		t.Errorf("after SetCSSText: GetPropertyValue(font-weight) = %q, want %q", got, want)
	}
	if got := style.GetPropertyValue("color"); got != "" {
		t.Errorf("SetCSSText should replace, not merge: GetPropertyValue(color) = %q, want \"\"", got)
	}
}

func TestStyleCustomProperty(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d">x</div>`, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("d")
	style := el.Style()

	style.SetProperty("--MyVar", "5")
	if got, want := style.GetPropertyValue("--MyVar"), "5"; got != want {
		t.Errorf("GetPropertyValue(--MyVar) = %q, want %q (case-preserving)", got, want)
	}
	if got := style.GetPropertyValue("--myvar"); got != "" {
		t.Errorf("GetPropertyValue(--myvar) = %q, want \"\" (custom props are case-sensitive)", got)
	}
}
