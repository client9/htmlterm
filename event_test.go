package htmlterm_test

import (
	"testing"

	"github.com/client9/htmlterm"
)

func mustParseDoc(t *testing.T, htmlStr string) *htmlterm.Document {
	t.Helper()
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return doc
}

func TestEventDispatchCaptureTargetBubbleOrder(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><div id="mid"><button id="inner">Go</button></div></div>`)
	outer := doc.GetElementByID("outer")
	mid := doc.GetElementByID("mid")
	inner := doc.GetElementByID("inner")

	var order []string
	doc.AddEventListener(outer, "click", true, func(e *htmlterm.Event) { order = append(order, "outer-capture") })
	doc.AddEventListener(mid, "click", true, func(e *htmlterm.Event) { order = append(order, "mid-capture") })
	doc.AddEventListener(inner, "click", false, func(e *htmlterm.Event) { order = append(order, "inner-target") })
	doc.AddEventListener(mid, "click", false, func(e *htmlterm.Event) { order = append(order, "mid-bubble") })
	doc.AddEventListener(outer, "click", false, func(e *htmlterm.Event) { order = append(order, "outer-bubble") })

	rect, ok := doc.Rect(inner)
	if !ok {
		t.Fatalf("Rect(inner) not found")
	}
	if !doc.DispatchClick(rect.Row, rect.Col) {
		t.Fatalf("DispatchClick did not hit the button")
	}

	want := []string{"outer-capture", "mid-capture", "inner-target", "mid-bubble", "outer-bubble"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}

func TestEventStopPropagationStopsBubble(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><div id="mid"><button id="inner">Go</button></div></div>`)
	outer := doc.GetElementByID("outer")
	mid := doc.GetElementByID("mid")
	inner := doc.GetElementByID("inner")

	outerCalled := false
	doc.AddEventListener(mid, "click", false, func(e *htmlterm.Event) { e.StopPropagation() })
	doc.AddEventListener(outer, "click", false, func(e *htmlterm.Event) { outerCalled = true })

	rect, _ := doc.Rect(inner)
	doc.DispatchClick(rect.Row, rect.Col)

	if outerCalled {
		t.Error("outer listener ran after mid called StopPropagation, want it suppressed")
	}
}

func TestEventStopImmediatePropagationSkipsSiblingListeners(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	secondCalled := false
	doc.AddEventListener(btn, "click", false, func(e *htmlterm.Event) { e.StopImmediatePropagation() })
	doc.AddEventListener(btn, "click", false, func(e *htmlterm.Event) { secondCalled = true })

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)

	if secondCalled {
		t.Error("second listener on the same node ran after StopImmediatePropagation, want it suppressed")
	}
}

func TestDispatchClickHitTestsInnermostElement(t *testing.T) {
	doc := mustParseDoc(t, `<label id="lbl">Name: <input type="checkbox" id="cb"></label>`)
	lbl := doc.GetElementByID("lbl")
	cb := doc.GetElementByID("cb")

	var target string
	doc.AddEventListener(lbl, "click", false, func(e *htmlterm.Event) { target = e.Target.TagName() })

	rect, ok := doc.Rect(cb)
	if !ok {
		t.Fatalf("Rect(cb) not found")
	}
	if !doc.DispatchClick(rect.Row, rect.Col) {
		t.Fatalf("DispatchClick did not hit anything")
	}
	if target != "input" {
		t.Errorf("bubbled event's Target.TagName() = %q, want %q (innermost element)", target, "input")
	}
}

func TestDispatchClickReturnsFalseWhenNothingHit(t *testing.T) {
	doc := mustParseDoc(t, `<p>hello</p>`)
	if doc.DispatchClick(999, 999) {
		t.Error("DispatchClick at an empty point returned true, want false")
	}
}

func TestDispatchClickTogglesCheckbox(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	rect, _ := doc.Rect(cb)

	doc.DispatchClick(rect.Row, rect.Col)
	if !cb.Checked() {
		t.Fatal("checkbox not checked after first click")
	}
	doc.DispatchClick(rect.Row, rect.Col)
	if cb.Checked() {
		t.Fatal("checkbox still checked after second click")
	}
}

func TestDispatchClickPreventDefaultSuppressesToggle(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	doc.AddEventListener(cb, "click", false, func(e *htmlterm.Event) { e.PreventDefault() })

	rect, _ := doc.Rect(cb)
	doc.DispatchClick(rect.Row, rect.Col)
	if cb.Checked() {
		t.Error("checkbox checked despite PreventDefault, want unchanged")
	}
}

func TestDispatchClickRadioGroupScopedToForm(t *testing.T) {
	doc := mustParseDoc(t, `<form><input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2"></form><input type="radio" name="r" id="r3" checked>`)
	r1 := doc.GetElementByID("r1")
	r2 := doc.GetElementByID("r2")
	r3 := doc.GetElementByID("r3")

	rect, _ := doc.Rect(r2)
	doc.DispatchClick(rect.Row, rect.Col)

	if r1.Checked() {
		t.Error("r1 still checked, want cleared by clicking sibling r2 in the same form")
	}
	if !r2.Checked() {
		t.Error("r2 not checked after click")
	}
	if !r3.Checked() {
		t.Error("r3 (outside the form) unexpectedly cleared, want radio clearing scoped to the form")
	}
}

func TestDispatchClickSubmitButtonFiresSubmitOnForm(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name"><button id="go">Go</button></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)

	if !submitted {
		t.Error("clicking a bare <button> in a <form> did not fire submit")
	}
}

func TestDispatchClickButtonTypeButtonDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><button type="button" id="go">Go</button></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)

	if submitted {
		t.Error("clicking <button type=button> fired submit, want no-op")
	}
}

func TestDispatchClickSubmitInputFiresSubmitOnForm(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="submit" id="go" value="Go"></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)

	if !submitted {
		t.Error("clicking input[type=submit] did not fire submit")
	}
}

func TestDispatchClickDisabledCheckboxDoesNotToggle(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb" disabled>`)
	cb := doc.GetElementByID("cb")
	rect, _ := doc.Rect(cb)

	doc.DispatchClick(rect.Row, rect.Col)
	if cb.Checked() {
		t.Error("disabled checkbox toggled on click, want no-op")
	}
}

func TestDispatchClickDisabledSubmitButtonDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><button id="go" disabled>Go</button></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)

	if submitted {
		t.Error("clicking a disabled submit button fired submit, want no-op")
	}
}

func TestDispatchKeyEnterOnTextEntrySubmitsForm(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name"></form>`)
	form := doc.GetElementByID("f")
	name := doc.GetElementByID("name")
	doc.Focus(name)

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	doc.DispatchKey("Enter")

	if !submitted {
		t.Error("Enter in a text input inside a form did not fire submit")
	}
}

func TestDispatchKeyEnterOutsideFormDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="name">`)
	name := doc.GetElementByID("name")
	doc.Focus(name)

	// No form ancestor at all — DispatchKey should just not panic or fire
	// anything; nothing to assert beyond "doesn't blow up".
	if !doc.DispatchKey("Enter") {
		t.Error("DispatchKey(Enter) returned false with a focused element")
	}
}

func TestDispatchKeyEnterOnTextareaInsertsNewlineInsteadOfSubmitting(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><textarea id="ta"></textarea></form>`)
	form := doc.GetElementByID("f")
	ta := doc.GetElementByID("ta")
	doc.Focus(ta)

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *htmlterm.Event) { submitted = true })

	doc.DispatchKey("Enter")
	doc.DispatchKey("a")
	doc.DispatchKey("Enter")
	doc.DispatchKey("b")

	if submitted {
		t.Error("Enter in a <textarea> fired submit, want a newline inserted instead")
	}
	if want := "\na\nb"; ta.Value() != want {
		t.Errorf("textarea value = %q, want %q", ta.Value(), want)
	}
}

func TestFocusAndBlur(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b" disabled><button id="c">Go</button>`)
	a := doc.GetElementByID("a")

	if doc.FocusedElement() != nil {
		t.Fatal("FocusedElement should start nil")
	}
	if !doc.Focus(a) {
		t.Fatal("Focus(a) returned false, want true")
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "a" {
		t.Errorf("FocusedElement() = %v, want element a", got)
	}
	if m := doc.QuerySelector("input:focus"); m == nil || m.ID() != "a" {
		t.Errorf("QuerySelector(input:focus) = %v, want element a", m)
	}

	b := doc.GetElementByID("b")
	if doc.Focus(b) {
		t.Error("Focus on a disabled element returned true, want false")
	}

	doc.Blur()
	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() after Blur should be nil")
	}
	if m := doc.QuerySelector("input:focus"); m != nil {
		t.Errorf("QuerySelector(input:focus) after Blur = %v, want nil", m)
	}
}

func TestFocusNextSkipsDisabledAndWraps(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b" disabled><button id="c">Go</button>`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "a" {
		t.Fatalf("first FocusNext() = %v, want a", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "c" {
		t.Fatalf("second FocusNext() (should skip disabled b) = %v, want c", second)
	}
	third := doc.FocusNext()
	if third == nil || third.ID() != "a" {
		t.Fatalf("third FocusNext() (should wrap) = %v, want a", third)
	}

	prev := doc.FocusPrev()
	if prev == nil || prev.ID() != "c" {
		t.Fatalf("FocusPrev() = %v, want c", prev)
	}
}

func TestDispatchKeyTypesAndBackspace(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	doc.Focus(a)

	doc.DispatchKey("a")
	doc.DispatchKey("b")
	if got := a.Value(); got != "ab" {
		t.Fatalf("value after typing = %q, want %q", got, "ab")
	}
	doc.DispatchKey("Backspace")
	if got := a.Value(); got != "a" {
		t.Fatalf("value after Backspace = %q, want %q", got, "a")
	}
}

func TestDispatchKeySpaceTogglesFocusedCheckbox(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	doc.Focus(cb)

	doc.DispatchKey(" ")
	if !cb.Checked() {
		t.Fatal("checkbox not checked after space key")
	}
}

func TestDispatchKeyTabMovesFocus(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b">`)
	a := doc.GetElementByID("a")
	doc.Focus(a)

	doc.DispatchKey("Tab")
	if got := doc.FocusedElement(); got == nil || got.ID() != "b" {
		t.Errorf("FocusedElement() after Tab = %v, want b", got)
	}
}

func TestDispatchKeyReturnsFalseWhenNothingFocused(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	if doc.DispatchKey("a") {
		t.Error("DispatchKey with nothing focused returned true, want false")
	}
}

func TestDispatchKeyPreventDefaultSuppressesTyping(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	doc.Focus(a)
	doc.AddEventListener(a, "keydown", false, func(e *htmlterm.Event) { e.PreventDefault() })

	doc.DispatchKey("x")
	if got := a.Value(); got != "" {
		t.Errorf("value after PreventDefault-ed keydown = %q, want empty", got)
	}
}

func TestRemoveEventListener(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	called := false
	h := doc.AddEventListener(btn, "click", false, func(e *htmlterm.Event) { called = true })
	doc.RemoveEventListener(h)

	rect, _ := doc.Rect(btn)
	doc.DispatchClick(rect.Row, rect.Col)
	if called {
		t.Error("listener ran after RemoveEventListener, want it gone")
	}
}
