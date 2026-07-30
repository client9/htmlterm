package document_test

import (
	"strings"
	"testing"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
)

func mustParseDoc(t *testing.T, htmlStr string) *document.Document {
	t.Helper()
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc.AddEventListener(outer, "click", true, func(e *document.Event) { order = append(order, "outer-capture") })
	doc.AddEventListener(mid, "click", true, func(e *document.Event) { order = append(order, "mid-capture") })
	doc.AddEventListener(inner, "click", false, func(e *document.Event) { order = append(order, "inner-target") })
	doc.AddEventListener(mid, "click", false, func(e *document.Event) { order = append(order, "mid-bubble") })
	doc.AddEventListener(outer, "click", false, func(e *document.Event) { order = append(order, "outer-bubble") })

	rect, ok := inner.Rect()
	if !ok {
		t.Fatalf("Rect(inner) not found")
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
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
	doc.AddEventListener(mid, "click", false, func(e *document.Event) { e.StopPropagation() })
	doc.AddEventListener(outer, "click", false, func(e *document.Event) { outerCalled = true })

	rect, _ := inner.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if outerCalled {
		t.Error("outer listener ran after mid called StopPropagation, want it suppressed")
	}
}

func TestEventStopImmediatePropagationSkipsSiblingListeners(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	secondCalled := false
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { e.StopImmediatePropagation() })
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { secondCalled = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if secondCalled {
		t.Error("second listener on the same node ran after StopImmediatePropagation, want it suppressed")
	}
}

func TestDispatchClickHitTestsInnermostElement(t *testing.T) {
	doc := mustParseDoc(t, `<label id="lbl">Name: <input type="checkbox" id="cb"></label>`)
	lbl := doc.GetElementByID("lbl")
	cb := doc.GetElementByID("cb")

	var target string
	doc.AddEventListener(lbl, "click", false, func(e *document.Event) { target = e.Target.TagName() })

	rect, ok := cb.Rect()
	if !ok {
		t.Fatalf("Rect(cb) not found")
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("DispatchClick did not hit anything")
	}
	if target != "input" {
		t.Errorf("bubbled event's Target.TagName() = %q, want %q (innermost element)", target, "input")
	}
}

// TestDispatchClickFocusesTextEntryAndPositionsCaret pins that clicking a
// text <input> both focuses it (previously nothing did — see
// docs/proposals/CARET_SELECTION.md) and places the caret at the clicked
// column, not just at the end of the value.
func TestDispatchClickFocusesTextEntryAndPositionsCaret(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="a" value="hello">`)
	a := doc.GetElementByID("a")
	rect, ok := a.Rect()
	if !ok {
		t.Fatalf("Rect(a) not found")
	}
	// Rendered content is "[hello]": rect.Col+0 is "[", rect.Col+1 is "h".
	if !doc.DispatchClick(rect.Row, rect.Col+3, document.Modifiers{}) {
		t.Fatal("DispatchClick did not hit the input")
	}
	if doc.FocusedElement() == nil || !doc.FocusedElement().IsSameNode(a) {
		t.Fatal("click did not focus the text entry")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 3 || end != 3 {
		t.Errorf("selection after click = [%d,%d), want collapsed at 3", start, end)
	}
}

// TestDispatchClickShiftExtendsSelection pins that Shift+Click extends the
// existing selection to the click point rather than collapsing it there —
// real Shift+Click semantics, reachable without any mousemove/drag support.
func TestDispatchClickShiftExtendsSelection(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 1)
	rect, ok := a.Rect()
	if !ok {
		t.Fatalf("Rect(a) not found")
	}

	if !doc.DispatchClick(rect.Row, rect.Col+4, document.Modifiers{Shift: true}) {
		t.Fatal("DispatchClick did not hit the input")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 1 || end != 4 {
		t.Errorf("selection after Shift+Click = [%d,%d), want [1,4)", start, end)
	}
}

func TestDispatchClickReturnsFalseWhenNothingHit(t *testing.T) {
	doc := mustParseDoc(t, `<p>hello</p>`)
	if doc.DispatchClick(999, 999, document.Modifiers{}) {
		t.Error("DispatchClick at an empty point returned true, want false")
	}
}

func TestDispatchClickTogglesCheckbox(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	rect, _ := cb.Rect()

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !cb.Checked() {
		t.Fatal("checkbox not checked after first click")
	}
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if cb.Checked() {
		t.Fatal("checkbox still checked after second click")
	}
}

func TestDispatchClickPreventDefaultSuppressesToggle(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	doc.AddEventListener(cb, "click", false, func(e *document.Event) { e.PreventDefault() })

	rect, _ := cb.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if cb.Checked() {
		t.Error("checkbox checked despite PreventDefault, want unchanged")
	}
}

func TestDispatchClickRadioGroupScopedToForm(t *testing.T) {
	doc := mustParseDoc(t, `<form><input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2"></form><input type="radio" name="r" id="r3" checked>`)
	r1 := doc.GetElementByID("r1")
	r2 := doc.GetElementByID("r2")
	r3 := doc.GetElementByID("r3")

	rect, _ := r2.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

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
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !submitted {
		t.Error("clicking a bare <button> in a <form> did not fire submit")
	}
}

func TestDispatchClickButtonTypeButtonDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><button type="button" id="go">Go</button></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if submitted {
		t.Error("clicking <button type=button> fired submit, want no-op")
	}
}

func TestDispatchClickSubmitInputFiresSubmitOnForm(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="submit" id="go" value="Go"></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !submitted {
		t.Error("clicking input[type=submit] did not fire submit")
	}
}

func TestDispatchClickDisabledCheckboxDoesNotToggle(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb" disabled>`)
	cb := doc.GetElementByID("cb")
	rect, _ := cb.Rect()

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if cb.Checked() {
		t.Error("disabled checkbox toggled on click, want no-op")
	}
}

func TestDispatchClickDisabledSubmitButtonDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><button id="go" disabled>Go</button></form>`)
	form := doc.GetElementByID("f")
	btn := doc.GetElementByID("go")

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if submitted {
		t.Error("clicking a disabled submit button fired submit, want no-op")
	}
}

// TestDispatchClickHitsPositionRelativeShiftedLocation confirms
// position: relative moves both what's painted and what's hit-tested: a
// click at the button's original, unshifted layout slot must miss (that
// space is blanked, not the button), and a click at its Rect() (which
// reports the shifted position, matching a real getBoundingClientRect())
// must hit it.
func TestDispatchClickHitsPositionRelativeShiftedLocation(t *testing.T) {
	doc, err := document.ParseDocument(
		`<button id="go" style="position:relative;top:2;left:5">Go</button>`,
		htmlterm.Options{Width: 40, Height: 10},
	)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	btn := doc.GetElementByID("go")

	clicked := false
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { clicked = true })

	// The original (unshifted) slot is row 0, col 0 — now blank.
	if doc.DispatchClick(0, 0, document.Modifiers{}) {
		t.Error("DispatchClick at the original layout slot hit something, want a miss")
	}
	if clicked {
		t.Error("click fired at the original layout slot, want no-op")
	}

	rect, ok := btn.Rect()
	if !ok {
		t.Fatalf("Rect(go) not found")
	}
	if rect.Row != 2 || rect.Col != 5 {
		t.Fatalf("Rect() = %+v, want Row=2 Col=5 (shifted)", rect)
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("DispatchClick at the shifted Rect() did not hit the button")
	}
	if !clicked {
		t.Error("click did not fire at the shifted position")
	}
}

// TestDispatchClickHitsPositionAbsoluteComputedLocation confirms
// position: absolute is hit-tested at its computed position, not wherever
// it would have flowed: the button reserves no space in normal flow (a
// click at row 0, where it would have flowed, misses — that row belongs to
// the sibling paragraph instead), and a click at its Rect() (computed
// against the document root, no positioned ancestor here) hits it.
func TestDispatchClickHitsPositionAbsoluteComputedLocation(t *testing.T) {
	doc, err := document.ParseDocument(
		`<button id="go" style="position:absolute;top:2;left:5">Go</button><p>hello</p>`,
		htmlterm.Options{Width: 40, Height: 10},
	)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	btn := doc.GetElementByID("go")

	clicked := false
	doc.AddEventListener(btn, "click", false, func(e *document.Event) { clicked = true })

	// Row 0 is where the button would have flowed (it's the first element
	// in the document) — it's out-of-flow, so that row belongs to <p>
	// instead: the click still hits something (the paragraph), just not
	// the button.
	doc.DispatchClick(0, 0, document.Modifiers{})
	if clicked {
		t.Error("click fired at the button's would-be flow position, want no-op")
	}

	rect, ok := btn.Rect()
	if !ok {
		t.Fatalf("Rect(go) not found")
	}
	if rect.Row != 2 || rect.Col != 5 {
		t.Fatalf("Rect() = %+v, want Row=2 Col=5 (computed against the document root)", rect)
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("DispatchClick at the computed Rect() did not hit the button")
	}
	if !clicked {
		t.Error("click did not fire at the computed position")
	}
}

func TestDispatchKeyEnterOnTextEntrySubmitsForm(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name"></form>`)
	form := doc.GetElementByID("f")
	name := doc.GetElementByID("name")
	name.Focus()

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	doc.DispatchKey("Enter", document.Modifiers{})

	if !submitted {
		t.Error("Enter in a text input inside a form did not fire submit")
	}
}

func TestDispatchKeyEnterOutsideFormDoesNotSubmit(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="name">`)
	name := doc.GetElementByID("name")
	name.Focus()

	// No form ancestor at all — DispatchKey should just not panic or fire
	// anything; nothing to assert beyond "doesn't blow up".
	if !doc.DispatchKey("Enter", document.Modifiers{}) {
		t.Error("DispatchKey(Enter) returned false with a focused element")
	}
}

func TestDispatchKeyEnterOnTextareaInsertsNewlineInsteadOfSubmitting(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><textarea id="ta"></textarea></form>`)
	form := doc.GetElementByID("f")
	ta := doc.GetElementByID("ta")
	ta.Focus()

	submitted := false
	doc.AddEventListener(form, "submit", false, func(e *document.Event) { submitted = true })

	doc.DispatchKey("Enter", document.Modifiers{})
	doc.DispatchKey("a", document.Modifiers{})
	doc.DispatchKey("Enter", document.Modifiers{})
	doc.DispatchKey("b", document.Modifiers{})

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
	if !a.Focus() {
		t.Fatal("Focus(a) returned false, want true")
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "a" {
		t.Errorf("FocusedElement() = %v, want element a", got)
	}
	if m := doc.QuerySelector("input:focus"); m == nil || m.ID() != "a" {
		t.Errorf("QuerySelector(input:focus) = %v, want element a", m)
	}

	b := doc.GetElementByID("b")
	if b.Focus() {
		t.Error("Focus on a disabled element returned true, want false")
	}

	a.Blur()
	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() after Blur should be nil")
	}
	if m := doc.QuerySelector("input:focus"); m != nil {
		t.Errorf("QuerySelector(input:focus) after Blur = %v, want nil", m)
	}
}

// TestElementBlurNoopIfNotFocused: calling Blur on an element that isn't the
// currently focused one must do nothing — mirroring a real browser, where
// HTMLElement.blur() only has an effect on the actually-focused element.
func TestElementBlurNoopIfNotFocused(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b">`)
	a := doc.GetElementByID("a")
	b := doc.GetElementByID("b")

	if !a.Focus() {
		t.Fatal("Focus(a) returned false, want true")
	}
	b.Blur()
	if got := doc.FocusedElement(); got == nil || got.ID() != "a" {
		t.Errorf("FocusedElement() after unfocused b.Blur() = %v, want element a (unchanged)", got)
	}
}

// TestElementFocusBlurRectNilReceiverSafe pins that Focus/Blur/Rect on a nil
// *Element (or one never attached to a Document) degrade to a safe
// false/no-op/zero-value rather than panicking — the same defensive shape
// every other pointer-receiver method in this package that takes an *Element
// argument already has (e.g. Document.Rect used to check el == nil before
// this refactor moved Rect onto Element itself).
func TestElementFocusBlurRectNilReceiverSafe(t *testing.T) {
	var el *document.Element
	if el.Focus() {
		t.Error("nil Element.Focus() = true, want false")
	}
	el.Blur() // must not panic
	if _, ok := el.Rect(); ok {
		t.Error("nil Element.Rect() ok = true, want false")
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

// TestFocusNextOrdersByPositiveTabindex: elements with a positive tabindex
// come first, ascending by value (ties broken by document order), followed
// by every other focusable element in document order — real DOM's
// tabindex-ordering algorithm.
func TestFocusNextOrdersByPositiveTabindex(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b" tabindex="2"><input id="c" tabindex="1">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "c" {
		t.Fatalf("first FocusNext() = %v, want c (tabindex=1)", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "b" {
		t.Fatalf("second FocusNext() = %v, want b (tabindex=2)", second)
	}
	third := doc.FocusNext()
	if third == nil || third.ID() != "a" {
		t.Fatalf("third FocusNext() = %v, want a (no tabindex, sorts last)", third)
	}
}

// TestTabindexZeroMakesDivFocusable: tabindex="0" makes an otherwise
// non-focusable element (like <div>) a real tab stop, matching real DOM.
func TestTabindexZeroMakesDivFocusable(t *testing.T) {
	doc := mustParseDoc(t, `<div id="a">plain</div><div id="b" tabindex="0">focusable</div>`)

	b := doc.GetElementByID("b")
	if !b.Focus() {
		t.Fatal("Focus() on div[tabindex=0] returned false, want true")
	}
	if m := doc.QuerySelector("div:focus"); m == nil || m.ID() != "b" {
		t.Errorf("QuerySelector(div:focus) = %v, want element b", m)
	}

	first := doc.FocusNext()
	if first == nil || first.ID() != "b" {
		t.Fatalf("FocusNext() = %v, want b (only tabindex-bearing div is a tab stop)", first)
	}
}

// TestTabindexOnAnchor: plain <a href> is never a tab stop (an existing,
// documented deviation from real browsers), but explicit tabindex opts it
// in, same as any other element.
func TestTabindexOnAnchor(t *testing.T) {
	doc := mustParseDoc(t, `<a id="a" href="/x">plain link</a><a id="b" href="/y" tabindex="0">tabbable link</a>`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "b" {
		t.Fatalf("FocusNext() = %v, want b (only the tabindex anchor is a tab stop)", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "b" {
		t.Fatalf("second FocusNext() = %v, want b again (wraps, only one tab stop)", second)
	}
}

// TestTabindexNegativeExcludedFromTabOrderButFocusable: tabindex="-1" is
// reachable via Focus()/click but skipped by Tab navigation, matching real
// DOM's tabindex="-1" semantics.
func TestTabindexNegativeExcludedFromTabOrderButFocusable(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><div id="b" tabindex="-1">panel</div><input id="c">`)

	b := doc.GetElementByID("b")
	if !b.Focus() {
		t.Fatal("Focus() on div[tabindex=-1] returned false, want true (focusable, just not in tab order)")
	}

	doc.GetElementByID("a").Focus()
	next := doc.FocusNext()
	if next == nil || next.ID() != "c" {
		t.Fatalf("FocusNext() from a = %v, want c (tabindex=-1 element b must be skipped)", next)
	}
}

// TestTabindexInvalidValueTreatedAsAbsent: a non-integer tabindex value is
// treated as if the attribute were absent, matching real DOM behavior.
func TestTabindexInvalidValueTreatedAsAbsent(t *testing.T) {
	doc := mustParseDoc(t, `<div id="a" tabindex="not-a-number">plain</div>`)

	a := doc.GetElementByID("a")
	if a.Focus() {
		t.Error("Focus() on div with invalid tabindex returned true, want false")
	}
}

// TestTabindexDisabledStillExcluded: a disabled form control with a
// tabindex attribute stays excluded from focus, same as without tabindex.
func TestTabindexDisabledStillExcluded(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" disabled tabindex="0">`)

	a := doc.GetElementByID("a")
	if a.Focus() {
		t.Error("Focus() on disabled input with tabindex=0 returned true, want false")
	}
}

func TestDispatchKeyTypesAndBackspace(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.DispatchKey("a", document.Modifiers{})
	doc.DispatchKey("b", document.Modifiers{})
	if got := a.Value(); got != "ab" {
		t.Fatalf("value after typing = %q, want %q", got, "ab")
	}
	doc.DispatchKey("Backspace", document.Modifiers{})
	if got := a.Value(); got != "a" {
		t.Fatalf("value after Backspace = %q, want %q", got, "a")
	}
}

// TestSelectionRendersHighlightEndToEnd exercises the full path from
// Document.DispatchKey through Document.Render: a Shift+Arrow-extended
// selection on a focused <input> renders under a reverse-video highlight —
// see docs/proposals/CARET_SELECTION.md's Document->internal/render marker
// attribute plumbing (Document.setSelection/syncSelectionAttrs).
func TestSelectionRendersHighlightEndToEnd(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(5, 5)

	doc.DispatchKey("ArrowLeft", document.Modifiers{Shift: true})
	doc.DispatchKey("ArrowLeft", document.Modifiers{Shift: true})

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "\x1b[7mlo\x1b[m") {
		t.Errorf("expected \"lo\" wrapped in reverse video, got %q", out)
	}
}

func TestDispatchKeyArrowMovesCaretNotJustScroll(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="abc">`)
	a := doc.GetElementByID("a")
	a.Focus()
	// SelectionStart/End default to the end of the value (3).
	if got := a.SelectionEnd(); got != 3 {
		t.Fatalf("initial SelectionEnd() = %d, want 3", got)
	}

	doc.DispatchKey("ArrowLeft", document.Modifiers{})
	if got := a.SelectionEnd(); got != 2 {
		t.Errorf("after ArrowLeft, SelectionEnd() = %d, want 2", got)
	}

	// Typing at the caret inserts there, not at the end of the value.
	doc.DispatchKey("X", document.Modifiers{})
	if got := a.Value(); got != "abXc" {
		t.Fatalf("value after typing mid-string = %q, want %q", got, "abXc")
	}
	if got := a.SelectionEnd(); got != 3 {
		t.Errorf("SelectionEnd() after insert = %d, want 3 (just after inserted char)", got)
	}
}

func TestDispatchKeyShiftArrowExtendsSelectionThenTypeReplaces(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(5, 5) // collapsed at end

	// Shift+ArrowLeft twice: select the last two characters ("lo").
	doc.DispatchKey("ArrowLeft", document.Modifiers{Shift: true})
	doc.DispatchKey("ArrowLeft", document.Modifiers{Shift: true})
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 3 || end != 5 {
		t.Fatalf("selection after Shift+ArrowLeft x2 = [%d,%d), want [3,5)", start, end)
	}
	if dir := a.SelectionDirection(); dir != "backward" {
		t.Errorf("SelectionDirection() = %q, want \"backward\"", dir)
	}

	// Typing replaces the selection.
	doc.DispatchKey("!", document.Modifiers{})
	if got := a.Value(); got != "hel!" {
		t.Fatalf("value after typing over selection = %q, want %q", got, "hel!")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 4 || end != 4 {
		t.Errorf("selection after replace = [%d,%d), want collapsed at 4", start, end)
	}
}

func TestDispatchKeyHomeEndOnTextarea(t *testing.T) {
	doc := mustParseDoc(t, `<textarea id="a" value="foo&#10;bar"></textarea>`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(5, 5) // just after "foo\nb" -> caret between "b" and "ar"

	doc.DispatchKey("Home", document.Modifiers{})
	if got := a.SelectionEnd(); got != 4 { // start of second line ("foo\n" is 4 runes)
		t.Errorf("after Home, SelectionEnd() = %d, want 4", got)
	}
	doc.DispatchKey("End", document.Modifiers{})
	if got := a.SelectionEnd(); got != 7 { // end of "foo\nbar"
		t.Errorf("after End, SelectionEnd() = %d, want 7", got)
	}
}

func TestDispatchKeyDeleteAndBackspaceOnSelection(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 4) // "ell" selected

	doc.DispatchKey("Delete", document.Modifiers{})
	if got := a.Value(); got != "ho" {
		t.Fatalf("value after Delete on selection = %q, want %q", got, "ho")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 1 || end != 1 {
		t.Errorf("selection after Delete = [%d,%d), want collapsed at 1", start, end)
	}

	doc.DispatchKey("Backspace", document.Modifiers{})
	if got := a.Value(); got != "o" {
		t.Fatalf("value after Backspace = %q, want %q", got, "o")
	}
}

func TestDispatchKeyCtrlASelectsAll(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(2, 2)

	doc.DispatchKey("a", document.Modifiers{Ctrl: true})
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 0 || end != 5 {
		t.Errorf("selection after Ctrl+A = [%d,%d), want [0,5)", start, end)
	}
	if dir := a.SelectionDirection(); dir != "forward" {
		t.Errorf("SelectionDirection() = %q, want \"forward\"", dir)
	}
}

func TestDispatchKeyArrowLeftCollapsesSelectionToNearEdge(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 4)

	doc.DispatchKey("ArrowLeft", document.Modifiers{})
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 1 || end != 1 {
		t.Errorf("selection after unmodified ArrowLeft = [%d,%d), want collapsed at 1 (near edge)", start, end)
	}
}

func TestDispatchKeyArrowMovementFiresNoInputEvent(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()

	var fired bool
	doc.AddEventListener(a, "input", false, func(e *document.Event) { fired = true })

	doc.DispatchKey("ArrowLeft", document.Modifiers{})
	doc.DispatchKey("Home", document.Modifiers{})
	doc.DispatchKey("End", document.Modifiers{Shift: true})
	if fired {
		t.Error("pure caret movement fired \"input\", want none (no value mutation)")
	}
}

func TestDispatchKeyFiresInputOnEveryMutatingKeystroke(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	a.Focus()

	var events []string
	doc.AddEventListener(a, "input", false, func(e *document.Event) { events = append(events, e.Key) })

	doc.DispatchKey("a", document.Modifiers{})
	doc.DispatchKey("b", document.Modifiers{})
	doc.DispatchKey("Backspace", document.Modifiers{})
	doc.DispatchKey("Backspace", document.Modifiers{}) // value now empty
	doc.DispatchKey("Backspace", document.Modifiers{}) // no-op: nothing to delete, no "input" expected

	want := []string{"a", "b", "Backspace", "Backspace"}
	if len(events) != len(want) {
		t.Fatalf("input events = %v, want %v", events, want)
	}
	for i, k := range want {
		if events[i] != k {
			t.Errorf("input event %d key = %q, want %q", i, events[i], k)
		}
	}
}

func TestDispatchKeyChangeFiresOnCommitNotEveryKeystroke(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b">`)
	a := doc.GetElementByID("a")
	b := doc.GetElementByID("b")
	a.Focus()

	changed := 0
	doc.AddEventListener(a, "change", false, func(e *document.Event) { changed++ })

	doc.DispatchKey("x", document.Modifiers{})
	doc.DispatchKey("y", document.Modifiers{})
	if changed != 0 {
		t.Fatalf("change fired %d times during typing, want 0", changed)
	}

	// Enter commits without losing focus; a second Enter with no further
	// typing shouldn't re-fire since the value hasn't changed since.
	doc.DispatchKey("Enter", document.Modifiers{})
	if changed != 1 {
		t.Fatalf("change count after Enter = %d, want 1", changed)
	}
	doc.DispatchKey("Enter", document.Modifiers{})
	if changed != 1 {
		t.Fatalf("change count after second no-op Enter = %d, want 1", changed)
	}

	// Typing again and then moving focus away (blur) should fire once more.
	doc.DispatchKey("z", document.Modifiers{})
	b.Focus()
	if changed != 2 {
		t.Fatalf("change count after typing + blur = %d, want 2", changed)
	}

	// Re-focusing a and blurring with no edits shouldn't fire again.
	a.Focus()
	b.Focus()
	if changed != 2 {
		t.Fatalf("change count after no-op focus/blur = %d, want 2", changed)
	}
}

func TestDispatchPasteAppendsToFocusedTextEntry(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="ab">`)
	a := doc.GetElementByID("a")
	a.Focus()

	var inputs []string
	doc.AddEventListener(a, "input", false, func(e *document.Event) { inputs = append(inputs, e.Key) })

	if !doc.DispatchPaste("cd") {
		t.Fatal("DispatchPaste returned false with a focused element")
	}
	if got := a.Value(); got != "abcd" {
		t.Fatalf("value after paste = %q, want %q", got, "abcd")
	}
	if len(inputs) != 1 || inputs[0] != "" {
		t.Fatalf("input events after paste = %v, want one event with empty Key", inputs)
	}
}

func TestDispatchPasteListenerCanRewriteClipboardData(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.AddEventListener(a, "paste", false, func(e *document.Event) {
		e.ClipboardData = strings.ToUpper(e.ClipboardData)
	})

	doc.DispatchPaste("abc")
	if got := a.Value(); got != "ABC" {
		t.Fatalf("value after rewritten paste = %q, want %q", got, "ABC")
	}
}

func TestDispatchPastePreventDefaultSuppressesInsertion(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="x">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.AddEventListener(a, "paste", false, func(e *document.Event) { e.PreventDefault() })

	doc.DispatchPaste("y")
	if got := a.Value(); got != "x" {
		t.Fatalf("value after prevented paste = %q, want unchanged %q", got, "x")
	}
}

func TestDispatchPasteReturnsFalseWhenNothingFocused(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	if doc.DispatchPaste("x") {
		t.Fatal("DispatchPaste returned true with nothing focused")
	}
}

func TestDispatchPasteReplacesSelection(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 4) // "ell" selected

	if !doc.DispatchPaste("XY") {
		t.Fatal("DispatchPaste returned false with a focused element")
	}
	if got := a.Value(); got != "hXYo" {
		t.Fatalf("value after paste over selection = %q, want %q", got, "hXYo")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 3 || end != 3 {
		t.Errorf("selection after paste = [%d,%d), want collapsed at 3 (just after pasted text)", start, end)
	}
}

func TestDispatchCutRemovesOnlySelectedRange(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 4) // "ell" selected

	text, ok := doc.DispatchCut()
	if !ok {
		t.Fatal("DispatchCut returned ok=false with a focused element")
	}
	if text != "ell" {
		t.Fatalf("DispatchCut text = %q, want %q (just the selected range)", text, "ell")
	}
	if got := a.Value(); got != "ho" {
		t.Fatalf("value after cutting a selection = %q, want %q", got, "ho")
	}
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 1 || end != 1 {
		t.Errorf("selection after cut = [%d,%d), want collapsed at 1 (start of the removed range)", start, end)
	}
}

// TestDispatchCutListenerShrinkingValueDoesNotPanic pins that a "cut"
// listener mutating (in particular, shortening) the target's value during
// dispatch doesn't crash the post-dispatch deletion step — it must re-read
// the selection instead of reusing the pre-dispatch snapshot, since that
// snapshot can point past the end of a since-shortened value.
func TestDispatchCutListenerShrinkingValueDoesNotPanic(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 4) // "ell" selected, out of a 5-rune value

	doc.AddEventListener(a, "cut", false, func(e *document.Event) {
		// Shrinks the value below the pre-dispatch selection's end (4).
		e.Target.SetValue("hi")
	})

	text, ok := doc.DispatchCut()
	if !ok {
		t.Fatal("DispatchCut returned ok=false with a focused element")
	}
	if text != "ell" {
		t.Fatalf("DispatchCut text (populated pre-dispatch) = %q, want %q", text, "ell")
	}
	// The listener's SetValue collapsed the selection (SetValue always
	// does), so the post-dispatch default action falls back to clearing
	// the (now-current) whole value, rather than operating on the
	// pre-dispatch range at all.
	if got := a.Value(); got != "" {
		t.Errorf("value after cut = %q, want empty", got)
	}
}

func TestDispatchCutClearsFocusedTextEntryAndReturnsValue(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="secret">`)
	a := doc.GetElementByID("a")
	a.Focus()

	var gotClipboard string
	doc.AddEventListener(a, "cut", false, func(e *document.Event) { gotClipboard = e.ClipboardData })

	text, ok := doc.DispatchCut()
	if !ok {
		t.Fatal("DispatchCut returned ok=false with a focused element")
	}
	if text != "secret" {
		t.Fatalf("DispatchCut text = %q, want %q", text, "secret")
	}
	if gotClipboard != "secret" {
		t.Fatalf("cut event ClipboardData = %q, want %q (pre-populated before dispatch)", gotClipboard, "secret")
	}
	if got := a.Value(); got != "" {
		t.Fatalf("value after cut = %q, want empty", got)
	}
}

func TestDispatchCutPreventDefaultKeepsValue(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="keep">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.AddEventListener(a, "cut", false, func(e *document.Event) { e.PreventDefault() })

	text, ok := doc.DispatchCut()
	if !ok || text != "keep" {
		t.Fatalf("DispatchCut() = (%q, %v), want (%q, true)", text, ok, "keep")
	}
	if got := a.Value(); got != "keep" {
		t.Fatalf("value after prevented cut = %q, want unchanged %q", got, "keep")
	}
}

func TestDispatchCutReturnsNotOkWhenNothingFocused(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="x">`)
	if _, ok := doc.DispatchCut(); ok {
		t.Fatal("DispatchCut returned ok=true with nothing focused")
	}
}

func TestDispatchKeySpaceTogglesFocusedCheckbox(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	cb.Focus()

	doc.DispatchKey(" ", document.Modifiers{})
	if !cb.Checked() {
		t.Fatal("checkbox not checked after space key")
	}
}

func TestDispatchKeyTabMovesFocus(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.DispatchKey("Tab", document.Modifiers{})
	if got := doc.FocusedElement(); got == nil || got.ID() != "b" {
		t.Errorf("FocusedElement() after Tab = %v, want b", got)
	}
}

func TestDispatchKeyReturnsFalseWhenNothingFocused(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	if doc.DispatchKey("a", document.Modifiers{}) {
		t.Error("DispatchKey with nothing focused returned true, want false")
	}
}

func TestDispatchKeyPreventDefaultSuppressesTyping(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	a := doc.GetElementByID("a")
	a.Focus()
	doc.AddEventListener(a, "keydown", false, func(e *document.Event) { e.PreventDefault() })

	doc.DispatchKey("x", document.Modifiers{})
	if got := a.Value(); got != "" {
		t.Errorf("value after PreventDefault-ed keydown = %q, want empty", got)
	}
}

func TestRemoveEventListener(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	called := false
	h := doc.AddEventListener(btn, "click", false, func(e *document.Event) { called = true })
	doc.RemoveEventListener(h)

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if called {
		t.Error("listener ran after RemoveEventListener, want it gone")
	}
}

func TestDispatchEventCaptureTargetBubbleOrder(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><div id="mid"><span id="inner">x</span></div></div>`)
	outer := doc.GetElementByID("outer")
	mid := doc.GetElementByID("mid")
	inner := doc.GetElementByID("inner")

	var order []string
	doc.AddEventListener(outer, "tabchange", true, func(e *document.Event) { order = append(order, "outer-capture") })
	doc.AddEventListener(mid, "tabchange", true, func(e *document.Event) { order = append(order, "mid-capture") })
	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) { order = append(order, "inner-target") })
	doc.AddEventListener(mid, "tabchange", false, func(e *document.Event) { order = append(order, "mid-bubble") })
	doc.AddEventListener(outer, "tabchange", false, func(e *document.Event) { order = append(order, "outer-bubble") })

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Bubbles: true})
	if !inner.DispatchEvent(ev) {
		t.Fatal("DispatchEvent = false, want true")
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

func TestDispatchEventNonBubblingSkipsBubblePhaseButRunsCapture(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><span id="inner">x</span></div>`)
	outer := doc.GetElementByID("outer")
	inner := doc.GetElementByID("inner")

	captureCalled, bubbleCalled := false, false
	doc.AddEventListener(outer, "tabchange", true, func(e *document.Event) { captureCalled = true })
	doc.AddEventListener(outer, "tabchange", false, func(e *document.Event) { bubbleCalled = true })

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Bubbles: false})
	inner.DispatchEvent(ev)

	if !captureCalled {
		t.Error("capture-phase listener did not run for non-bubbling event")
	}
	if bubbleCalled {
		t.Error("bubble-phase listener ran for a Bubbles:false event, want suppressed")
	}
}

func TestDispatchEventCancelablePreventDefault(t *testing.T) {
	doc := mustParseDoc(t, `<span id="inner">x</span>`)
	inner := doc.GetElementByID("inner")

	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) { e.PreventDefault() })

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Cancelable: true})
	if inner.DispatchEvent(ev) {
		t.Error("DispatchEvent = true after PreventDefault on a cancelable event, want false")
	}
	if !ev.DefaultPrevented() {
		t.Error("DefaultPrevented() = false after PreventDefault on a cancelable event, want true")
	}
}

func TestDispatchEventNonCancelablePreventDefaultIsNoop(t *testing.T) {
	doc := mustParseDoc(t, `<span id="inner">x</span>`)
	inner := doc.GetElementByID("inner")

	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) { e.PreventDefault() })

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Cancelable: false})
	if !inner.DispatchEvent(ev) {
		t.Error("DispatchEvent = false for a non-cancelable event, want true (PreventDefault should be a no-op)")
	}
	if ev.DefaultPrevented() {
		t.Error("DefaultPrevented() = true on a non-cancelable event, want false")
	}
}

func TestDispatchEventDetailRoundTrips(t *testing.T) {
	doc := mustParseDoc(t, `<span id="inner">x</span>`)
	inner := doc.GetElementByID("inner")

	type payload struct{ Tab string }
	want := payload{Tab: "settings"}

	var got any
	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) { got = e.Detail })

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Detail: want})
	inner.DispatchEvent(ev)

	if got != want {
		t.Errorf("Detail seen by listener = %#v, want %#v", got, want)
	}
}

func TestDispatchEventReentrancyBlockedAndDoesNotCorruptOuterDispatch(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><span id="inner">x</span></div>`)
	outer := doc.GetElementByID("outer")
	inner := doc.GetElementByID("inner")

	var nestedResult bool
	var outerRanAfterNested bool
	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Bubbles: true})

	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) {
		nestedResult = inner.DispatchEvent(ev) // reentrant: same *Event, still mid-dispatch
	})
	doc.AddEventListener(outer, "tabchange", false, func(e *document.Event) {
		outerRanAfterNested = true
		if e.Target.ID() != "inner" {
			t.Errorf("outer bubble listener saw Target = %q, want inner (nested call must not have corrupted it)", e.Target.ID())
		}
	})

	if !inner.DispatchEvent(ev) {
		t.Error("outer DispatchEvent = false, want true")
	}
	if nestedResult {
		t.Error("nested (reentrant) DispatchEvent = true, want false")
	}
	if !outerRanAfterNested {
		t.Error("outer bubble listener did not run after the blocked nested dispatch")
	}
}

func TestDispatchEventSequentialRedispatchResetsStateButKeepsDefaultPrevented(t *testing.T) {
	doc := mustParseDoc(t, `<span id="inner">x</span>`)
	inner := doc.GetElementByID("inner")

	calls := 0
	doc.AddEventListener(inner, "tabchange", false, func(e *document.Event) {
		calls++
		e.PreventDefault()
	})

	ev := document.NewCustomEvent("tabchange", document.CustomEventInit{Cancelable: true})
	inner.DispatchEvent(ev)
	inner.DispatchEvent(ev)

	if calls != 2 {
		t.Errorf("listener ran %d times across two sequential dispatches, want 2", calls)
	}
	if !ev.DefaultPrevented() {
		t.Error("DefaultPrevented() = false after a second dispatch, want true (canceled flag persists across redispatch of the same Event)")
	}
}

func TestBuiltinEventsPopulateBubblesAndCancelable(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")

	var click, focus *document.Event
	doc.AddEventListener(cb, "click", false, func(e *document.Event) { click = e })
	doc.AddEventListener(cb, "focus", false, func(e *document.Event) { focus = e })

	rect, _ := cb.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	cb.Focus()

	if click == nil || !click.Bubbles || !click.Cancelable {
		t.Errorf("click event Bubbles/Cancelable = %v/%v, want true/true", click.Bubbles, click.Cancelable)
	}
	if focus == nil || !focus.Bubbles || focus.Cancelable {
		t.Errorf("focus event Bubbles/Cancelable = %v/%v, want true/false", focus.Bubbles, focus.Cancelable)
	}

	focus.PreventDefault()
	if focus.DefaultPrevented() {
		t.Error("PreventDefault on a non-cancelable focus event took effect, want no-op")
	}
}

func TestDispatchEventReservedTypeNameRunsNoDefaultAction(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")

	listenerRan := false
	doc.AddEventListener(cb, "click", false, func(e *document.Event) { listenerRan = true })

	ev := document.NewCustomEvent("click", document.CustomEventInit{})
	cb.DispatchEvent(ev)

	if !listenerRan {
		t.Error("listener for reserved type name \"click\" did not run via DispatchEvent")
	}
	if cb.HasAttribute("checked") {
		t.Error("checkbox toggled via DispatchEvent(\"click\"), want no default action to fire")
	}
}

func TestElementDispatchEventNilSafe(t *testing.T) {
	var el *document.Element
	if el.DispatchEvent(document.NewCustomEvent("x", document.CustomEventInit{})) {
		t.Error("nil Element.DispatchEvent(...) = true, want false")
	}

	doc := mustParseDoc(t, `<span id="inner">x</span>`)
	inner := doc.GetElementByID("inner")
	if inner.DispatchEvent(nil) {
		t.Error("Element.DispatchEvent(nil) = true, want false")
	}
}
