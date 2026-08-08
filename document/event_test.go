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
// TestDispatchClickHitTestsFlexItemSizedByMinWidth pins that a flex item's
// recorded Rect matches where it actually paints when the item's own
// min-width is what determines its size. Flex layout used to resolve an
// item's main-axis size from flex-basis/width alone while the item's own
// render honored min-width too, so the second item painted six columns right
// of the Rect recorded for it — clicking the text you can see hit nothing,
// and clicking blank padding hit the wrong element.
func TestDispatchClickHitTestsFlexItemSizedByMinWidth(t *testing.T) {
	doc := mustParseDoc(t, `<div style="display:flex;width:20"><div id="a" style="width:3;min-width:9">aa</div><div id="b">bb</div></div>`)
	b := doc.GetElementByID("b")

	var target string
	doc.AddEventListener(b, "click", false, func(e *document.Event) { target = e.Target.ID() })

	rect, ok := b.Rect()
	if !ok {
		t.Fatalf("Rect(b) not found")
	}
	if rect.Col != 9 {
		t.Errorf("Rect(b).Col = %d, want 9 (just past the min-width:9 first item)", rect.Col)
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("DispatchClick at b's own Rect did not hit anything")
	}
	if target != "b" {
		t.Errorf("click at b's Rect hit %q, want %q", target, "b")
	}
}

// TestDispatchClickHitTestsPastAnOverflowingFlexItem pins that an item whose
// content is too wide for its resolved main-axis size — which overflows by
// design rather than being truncated — pushes the Rects of the items after it
// on the line. Flex layout used to advance by each item's allotted width, so
// the columns the overflow painted over were still attributed to the next
// item.
func TestDispatchClickHitTestsPastAnOverflowingFlexItem(t *testing.T) {
	doc := mustParseDoc(t, `<div style="display:flex;width:10"><div id="a" style="flex-basis:4">aaaaaa</div><div id="b" style="flex-basis:4">bb</div></div>`)
	b := doc.GetElementByID("b")

	var target string
	doc.AddEventListener(b, "click", false, func(e *document.Event) { target = e.Target.ID() })

	rect, ok := b.Rect()
	if !ok {
		t.Fatalf("Rect(b) not found")
	}
	if rect.Col != 6 {
		t.Errorf("Rect(b).Col = %d, want 6 (past all six columns the first item painted)", rect.Col)
	}
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatalf("DispatchClick at b's own Rect did not hit anything")
	}
	if target != "b" {
		t.Errorf("click at b's Rect hit %q, want %q", target, "b")
	}
}

// TestDispatchClickHitTestsStretchedColumnFlexItem pins that a column-direction
// flex item stretched by align-items (the default) claims the container's full
// content width for hit-testing, even though its painted box is left at its own
// narrower content width — padding the box out would stop an inline-flex
// container shrinking to fit, so only the Rect carries the stretched size. The
// Rect used to report the painted width, so a click in the blank columns to the
// item's right hit-tested to the container instead of to the item.
func TestDispatchClickHitTestsStretchedColumnFlexItem(t *testing.T) {
	doc := mustParseDoc(t, `<div id="box" style="display:flex;flex-direction:column;width:20;border-style:solid"><div id="a">short</div></div>`)
	a := doc.GetElementByID("a")

	rect, ok := a.Rect()
	if !ok {
		t.Fatalf("Rect(a) not found")
	}
	if rect.Width != 18 {
		t.Errorf("Rect(a).Width = %d, want 18 (the container's full content width)", rect.Width)
	}

	var target string
	doc.AddEventListener(doc.DocumentElement(), "click", false, func(e *document.Event) { target = e.Target.ID() })
	// Well past "short", but still inside the stretched item's own box.
	doc.DispatchClick(rect.Row, rect.Col+12, document.Modifiers{})
	if target != "a" {
		t.Errorf("click in the stretched item's blank columns hit %q, want %q", target, "a")
	}
}

// TestDispatchClickHitTestsUnstretchedColumnFlexItem is the control: an item the
// container does *not* stretch really is only as wide as it paints, so the same
// click belongs to the container.
func TestDispatchClickHitTestsUnstretchedColumnFlexItem(t *testing.T) {
	doc := mustParseDoc(t, `<div id="box" style="display:flex;flex-direction:column;align-items:flex-start;width:20;border-style:solid"><div id="a">short</div></div>`)
	a := doc.GetElementByID("a")

	rect, ok := a.Rect()
	if !ok {
		t.Fatalf("Rect(a) not found")
	}
	if rect.Width != 5 {
		t.Errorf("Rect(a).Width = %d, want 5 (its own content width)", rect.Width)
	}

	var target string
	doc.AddEventListener(doc.DocumentElement(), "click", false, func(e *document.Event) { target = e.Target.ID() })
	doc.DispatchClick(rect.Row, rect.Col+12, document.Modifiers{})
	if target != "box" {
		t.Errorf("click past an unstretched item hit %q, want %q", target, "box")
	}
}

func TestDispatchClickFocusesTextEntryAndPositionsCaret(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="a" value="hello">`)
	a := doc.GetElementByID("a")
	rect, ok := a.Rect()
	if !ok {
		t.Fatalf("Rect(a) not found")
	}
	// A text input renders its value plain, so rect.Col+0 is "h" and
	// rect.Col+3 is the gap between "l" and "l" — rune offset 3.
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

func TestDispatchClickTogglesDetailsOpen(t *testing.T) {
	doc := mustParseDoc(t, `<details id="d"><summary id="s">Title</summary><p>body</p></details>`)
	d := doc.GetElementByID("d")
	s := doc.GetElementByID("s")
	rect, _ := s.Rect()

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !d.Open() {
		t.Fatal("details not open after clicking summary")
	}
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if d.Open() {
		t.Fatal("details still open after second click on summary")
	}
}

// TestDispatchClickOnNestedSummaryContentTogglesDetails pins the same
// click-target-resolution problem <label> already solves: a click landing
// on inline content nested inside <summary> (its own hit-tested node) still
// resolves to the enclosing <summary> for the disclosure toggle, via
// nearestSummary's ancestor walk.
func TestDispatchClickOnNestedSummaryContentTogglesDetails(t *testing.T) {
	doc := mustParseDoc(t, `<details id="d"><summary><strong id="strong">Title</strong></summary><p>body</p></details>`)
	d := doc.GetElementByID("d")
	strong := doc.GetElementByID("strong")
	rect, _ := strong.Rect()

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !d.Open() {
		t.Fatal("details not open after clicking nested content inside summary")
	}
}

func TestDispatchClickTogglesDetailsFiresNonBubblingToggle(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><details id="d"><summary id="s">Title</summary><p>body</p></details></div>`)
	outer := doc.GetElementByID("outer")
	d := doc.GetElementByID("d")
	s := doc.GetElementByID("s")

	var targetCalled, bubbleCalled bool
	doc.AddEventListener(d, "toggle", false, func(e *document.Event) { targetCalled = true })
	doc.AddEventListener(outer, "toggle", false, func(e *document.Event) { bubbleCalled = true })

	rect, _ := s.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !targetCalled {
		t.Error("\"toggle\" listener on the details itself did not run")
	}
	if bubbleCalled {
		t.Error("\"toggle\" bubbled to an ancestor listener, want non-bubbling (matches HTMLDetailsElement's real toggle event)")
	}
}

func TestDispatchKeyEnterAndSpaceToggleFocusedSummary(t *testing.T) {
	for _, key := range []string{"Enter", " "} {
		doc := mustParseDoc(t, `<details id="d"><summary id="s">Title</summary><p>body</p></details>`)
		d := doc.GetElementByID("d")
		s := doc.GetElementByID("s")
		s.Focus()

		doc.DispatchKey(key, document.Modifiers{})
		if !d.Open() {
			t.Errorf("details not open after key %q on focused summary", key)
		}
	}
}

// TestDispatchKeyEnterOnControlNestedInSummaryDoesNotToggleDetails pins a
// regression: DispatchKey's disclosure-toggle case used to key off
// nearestSummary, which walks ancestors and so also matched a focused
// control nested inside <summary>, shadowing that control's own Enter
// default action (a <textarea>'s newline, here) with the details toggle
// instead. Only <summary> itself is ever a tab stop (isFormFocusable), so
// the case now checks isSummaryControl(target) directly.
func TestDispatchKeyEnterOnControlNestedInSummaryDoesNotToggleDetails(t *testing.T) {
	doc := mustParseDoc(t, `<details id="d"><summary>Notes: <textarea id="ta"></textarea></summary><p>body</p></details>`)
	d := doc.GetElementByID("d")
	ta := doc.GetElementByID("ta")
	ta.Focus()

	doc.DispatchKey("Enter", document.Modifiers{})
	if d.Open() {
		t.Error("details opened by Enter on a nested textarea, want the textarea's own newline action to run instead")
	}
	if got := ta.Value(); got != "\n" {
		t.Errorf("textarea value after Enter = %q, want %q (newline inserted, not shadowed)", got, "\n")
	}
}

func TestSummaryFocusableOnlyInsideDetails(t *testing.T) {
	doc := mustParseDoc(t, `<input id="before"><details><summary id="s">Title</summary></details><summary id="stray">stray</summary><input id="after">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "before" {
		t.Fatalf("first FocusNext() = %v, want \"before\"", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "s" {
		t.Errorf("second FocusNext() = %v, want \"s\" (summary inside details is a tab stop)", second)
	}
	third := doc.FocusNext()
	if third == nil || third.ID() != "after" {
		t.Errorf("third FocusNext() = %v, want \"after\" (stray summary outside details skipped)", third)
	}
}

// TestFocusSkipsContentHiddenByClosedDetails pins a regression: Tab
// traversal and Element.Focus both walk the DOM tree directly, unlike a
// click, which can never hit-test into hidden content because it has no
// Rect (see hiddenByClosedDetails). Content nested arbitrarily deep inside
// a closed details' non-summary body used to stay reachable and operable by
// keyboard even though it's invisible.
func TestFocusSkipsContentHiddenByClosedDetails(t *testing.T) {
	doc := mustParseDoc(t, `<input id="before"><details id="d"><summary id="s">More</summary><div><button id="hidden">Delete account</button></div></details><input id="after">`)
	hidden := doc.GetElementByID("hidden")

	if hidden.Focus() {
		t.Error("Focus() on a button hidden by a closed details returned true, want false")
	}
	if got := doc.FocusedElement(); got != nil {
		t.Errorf("FocusedElement() after failed Focus() = %v, want nil", got)
	}

	first := doc.FocusNext()
	if first == nil || first.ID() != "before" {
		t.Fatalf("first FocusNext() = %v, want \"before\"", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "s" {
		t.Fatalf("second FocusNext() = %v, want \"s\"", second)
	}
	third := doc.FocusNext()
	if third == nil || third.ID() != "after" {
		t.Errorf("third FocusNext() = %v, want \"after\" (button hidden by the closed details skipped)", third)
	}

	// Once open, the same button becomes focusable again.
	d := doc.GetElementByID("d")
	d.SetOpen(true)
	if !hidden.Focus() {
		t.Error("Focus() on the button failed after opening its details, want it reachable once visible")
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "hidden" {
		t.Errorf("FocusedElement() after open+Focus() = %v, want \"hidden\"", got)
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

func TestRadioGroupIsOneTabStopAtFirstMemberWhenNoneChecked(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1"><input type="radio" name="r" id="r2"><input type="text" id="after">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "r1" {
		t.Fatalf("first FocusNext() = %v, want \"r1\"", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "after" {
		t.Errorf("second FocusNext() = %v, want \"after\" (r2 skipped, same group as r1)", second)
	}
}

func TestRadioGroupIsOneTabStopAtCheckedMember(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1"><input type="radio" name="r" id="r2" checked><input type="text" id="after">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "r2" {
		t.Errorf("first FocusNext() = %v, want \"r2\" (the checked member)", first)
	}
}

func TestRadioGroupTabStopSkipsDisabledCanonicalMember(t *testing.T) {
	// Regression test: isRadioGroupTabStop used to designate the checked
	// (or first) member as the group's Tab stop without checking whether
	// that member was itself disabled. A disabled checked/first member
	// made the entire group untabbable, even with an enabled member still
	// in it, since no other member could ever match "the checked member"
	// or "the first member" either.
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked disabled><input type="radio" name="r" id="r2"><input type="text" id="after">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "r2" {
		t.Errorf("first FocusNext() with a disabled checked member = %v, want \"r2\" (the next enabled member)", first)
	}
}

func TestRadioGroupTabStopSkipsDisabledFirstMember(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" disabled><input type="radio" name="r" id="r2"><input type="text" id="after">`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "r2" {
		t.Errorf("first FocusNext() with a disabled first, unchecked member = %v, want \"r2\"", first)
	}
}

func TestRadioGroupTabStopScopedToForm(t *testing.T) {
	// Two same-name groups in different forms are independent, mirroring
	// clearRadioSiblings' own form-scoping.
	doc := mustParseDoc(t, `<form><input type="radio" name="r" id="a1"><input type="radio" name="r" id="a2" checked></form>`+
		`<form><input type="radio" name="r" id="b1"><input type="radio" name="r" id="b2"></form>`)

	first := doc.FocusNext()
	if first == nil || first.ID() != "a2" {
		t.Errorf("first FocusNext() = %v, want \"a2\" (checked, in the first form's own group)", first)
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "b1" {
		t.Errorf("second FocusNext() = %v, want \"b1\" (first, unchecked, in the second form's own group)", second)
	}
}

func TestRadioGroupMemberStillReachableByFocusAndClickDespiteTabSkip(t *testing.T) {
	// Only sequential Tab navigation skips a non-canonical group member;
	// Focus() and a direct click still reach it, the same "skipped by Tab,
	// not by everything else" split tabindex="-1" already has.
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1"><input type="radio" name="r" id="r2">`)
	r2 := doc.GetElementByID("r2")

	if !r2.Focus() {
		t.Fatal("Focus() on a non-canonical radio group member = false, want true")
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "r2" {
		t.Errorf("FocusedElement() after Focus() = %v, want \"r2\"", got)
	}

	doc.GetElementByID("r1").Blur()
	rect, _ := r2.Rect()
	if !doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Fatal("DispatchClick() on a non-canonical radio group member = false, want true")
	}
	if !r2.Checked() {
		t.Error("r2 not checked after a direct click")
	}
}

func TestFocusNextAfterFocusingNonCanonicalRadioAdvancesPastGroup(t *testing.T) {
	// Regression test: FocusNext/FocusPrev used to search for d.focused by
	// exact match in focusableList and silently fall back to list[0] (or
	// list[len-1] for FocusPrev) whenever it wasn't found there. A
	// non-canonical radio group member reached via Focus() is exactly that
	// case now that only one member of a group is ever in focusableList,
	// so Tab used to reset all the way to the document's first tab stop
	// instead of advancing past the group the user was actually on.
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2"><input type="text" id="after">`)
	doc.GetElementByID("r2").Focus()

	next := doc.FocusNext()
	if next == nil || next.ID() != "after" {
		t.Errorf("FocusNext() after Focus()ing a non-canonical group member = %v, want \"after\" (the stop past the whole group), not a reset to the document's first stop", next)
	}
}

func TestFocusPrevAfterFocusingNonCanonicalRadioAdvancesPastGroup(t *testing.T) {
	doc := mustParseDoc(t, `<input type="text" id="before"><input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2">`)
	doc.GetElementByID("r2").Focus()

	prev := doc.FocusPrev()
	if prev == nil || prev.ID() != "before" {
		t.Errorf("FocusPrev() after Focus()ing a non-canonical group member = %v, want \"before\" (the stop before the whole group)", prev)
	}
}

func TestDispatchKeyArrowDownMovesAndChecksNextRadio(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2"><input type="radio" name="r" id="r3">`)
	doc.GetElementByID("r1").Focus()

	doc.DispatchKey("ArrowDown", document.Modifiers{})

	got := doc.FocusedElement()
	if got == nil || got.ID() != "r2" {
		t.Fatalf("FocusedElement() after ArrowDown = %v, want \"r2\"", got)
	}
	if doc.GetElementByID("r1").Checked() {
		t.Error("r1 still checked after ArrowDown moved to r2")
	}
	if !doc.GetElementByID("r2").Checked() {
		t.Error("r2 not checked after ArrowDown")
	}
}

func TestDispatchKeyArrowUpWrapsToLastRadio(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2"><input type="radio" name="r" id="r3">`)
	doc.GetElementByID("r1").Focus()

	doc.DispatchKey("ArrowUp", document.Modifiers{})

	got := doc.FocusedElement()
	if got == nil || got.ID() != "r3" {
		t.Errorf("FocusedElement() after ArrowUp from the first member = %v, want \"r3\" (wraps to the last)", got)
	}
	if !doc.GetElementByID("r3").Checked() {
		t.Error("r3 not checked after ArrowUp wrapped to it")
	}
}

func TestDispatchKeyArrowLeftRightAlsoNavigateRadioGroup(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2">`)
	doc.GetElementByID("r1").Focus()

	doc.DispatchKey("ArrowRight", document.Modifiers{})
	if got := doc.FocusedElement(); got == nil || got.ID() != "r2" {
		t.Fatalf("FocusedElement() after ArrowRight = %v, want \"r2\"", got)
	}

	doc.DispatchKey("ArrowLeft", document.Modifiers{})
	if got := doc.FocusedElement(); got == nil || got.ID() != "r1" {
		t.Errorf("FocusedElement() after ArrowLeft = %v, want \"r1\"", got)
	}
}

func TestDispatchKeyRadioGroupArrowSkipsDisabledMember(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked><input type="radio" name="r" id="r2" disabled><input type="radio" name="r" id="r3">`)
	doc.GetElementByID("r1").Focus()

	doc.DispatchKey("ArrowDown", document.Modifiers{})

	got := doc.FocusedElement()
	if got == nil || got.ID() != "r3" {
		t.Errorf("FocusedElement() after ArrowDown past a disabled member = %v, want \"r3\"", got)
	}
}

func TestDispatchKeyRadioGroupArrowNoOpOnGroupOfOne(t *testing.T) {
	doc := mustParseDoc(t, `<input type="radio" name="r" id="r1" checked>`)
	doc.GetElementByID("r1").Focus()

	if doc.DispatchKey("ArrowDown", document.Modifiers{}) != true {
		t.Fatal("DispatchKey() = false, want true (still dispatches keydown even if the default action is a no-op)")
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "r1" {
		t.Errorf("FocusedElement() after ArrowDown in a group of one = %v, want unchanged \"r1\"", got)
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

func TestDispatchClickResetButtonRestoresFormDefaults(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f">
		<input type="text" id="name" value="Ada">
		<input type="checkbox" id="agree" checked>
		<input type="radio" name="color" id="red" value="red" checked>
		<input type="radio" name="color" id="blue" value="blue">
		<select id="pick"><option value="a">A</option><option value="b" selected>B</option></select>
		<button type="reset" id="go">Reset</button>
	</form>`)
	name := doc.GetElementByID("name")
	agree := doc.GetElementByID("agree")
	red := doc.GetElementByID("red")
	blue := doc.GetElementByID("blue")
	pick := doc.GetElementByID("pick")
	btn := doc.GetElementByID("go")

	// Dirty every control away from its parsed default.
	name.SetValue("Grace")
	agree.SetChecked(false)
	red.SetChecked(false)
	blue.SetChecked(true)
	pick.SetValue("a")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	reset := false
	doc.AddEventListener(doc.GetElementByID("f"), "reset", false, func(e *document.Event) { reset = true })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !reset {
		t.Error("clicking a reset button did not fire \"reset\" on its form")
	}
	if got := name.Value(); got != "Ada" {
		t.Errorf("name.Value() after reset = %q, want \"Ada\"", got)
	}
	if !agree.Checked() {
		t.Error("agree.Checked() after reset = false, want true")
	}
	if !red.Checked() {
		t.Error("red.Checked() after reset = false, want true")
	}
	if blue.Checked() {
		t.Error("blue.Checked() after reset = true, want false")
	}
	if got := pick.Value(); got != "b" {
		t.Errorf("pick.Value() after reset = %q, want \"b\"", got)
	}
}

func TestDispatchClickResetPreventDefaultLeavesValuesAlone(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name" value="Ada"><button type="reset" id="go">Reset</button></form>`)
	form := doc.GetElementByID("f")
	name := doc.GetElementByID("name")
	btn := doc.GetElementByID("go")

	name.SetValue("Grace")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	doc.AddEventListener(form, "reset", false, func(e *document.Event) { e.PreventDefault() })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if got := name.Value(); got != "Grace" {
		t.Errorf("name.Value() after prevented reset = %q, want unchanged \"Grace\"", got)
	}
}

func TestDispatchClickResetInputRestoresFormDefaults(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name" value="Ada"><input type="reset" id="go" value="Reset"></form>`)
	name := doc.GetElementByID("name")
	btn := doc.GetElementByID("go")

	name.SetValue("Grace")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if got := name.Value(); got != "Ada" {
		t.Errorf("name.Value() after resetting via input[type=reset] = %q, want \"Ada\"", got)
	}
}

func TestDispatchKeyEnterOnResetButtonRestoresFormDefaults(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name" value="Ada"><button type="reset" id="go">Reset</button></form>`)
	name := doc.GetElementByID("name")
	btn := doc.GetElementByID("go")
	btn.Focus()

	name.SetValue("Grace")

	doc.DispatchKey("Enter", document.Modifiers{})

	if got := name.Value(); got != "Ada" {
		t.Errorf("name.Value() after Enter on a focused reset button = %q, want \"Ada\"", got)
	}
}

func TestDispatchClickResetControlAddedAfterParseHasNoDefault(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"></form>`)
	form := doc.GetElementByID("f")

	input := doc.CreateElement("input")
	input.SetAttribute("id", "late")
	input.SetAttribute("value", "original")
	form.AppendChild(input)

	resetBtn := doc.CreateElement("button")
	resetBtn.SetAttribute("type", "reset")
	resetBtn.SetAttribute("id", "go")
	form.AppendChild(resetBtn)

	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	input.SetValue("changed")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	rect, _ := resetBtn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	// input never had a parse-time default recorded (it didn't exist at
	// ParseDocument), so reset leaves its current value untouched.
	if got := input.Value(); got != "changed" {
		t.Errorf("input.Value() after reset on a post-parse control = %q, want unchanged \"changed\"", got)
	}
}

func TestDispatchClickResetOnStillFocusedFieldDoesNotFireSpuriousChange(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="text" id="name" value="Ada"><button type="reset" id="go">Reset</button></form>`)
	name := doc.GetElementByID("name")
	btn := doc.GetElementByID("go")

	// Focus the field and commit an edit via Enter, which snapshots a new
	// valueAtFocus baseline while leaving the field focused.
	name.Focus()
	doc.DispatchKey("Backspace", document.Modifiers{})
	doc.DispatchKey("Backspace", document.Modifiers{})
	doc.DispatchKey("Enter", document.Modifiers{})
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	changed := false
	doc.AddEventListener(name, "change", false, func(e *document.Event) { changed = true })

	// Clicking the reset button doesn't move focus away from name (only a
	// click that targets a text entry does), so name is still focused when
	// triggerReset restores its value.
	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got := name.Value(); got != "Ada" {
		t.Fatalf("name.Value() after reset = %q, want \"Ada\"", got)
	}
	if got := doc.FocusedElement(); got == nil || got.ID() != "name" {
		t.Fatalf("FocusedElement() after reset click = %v, want still \"name\"", got)
	}

	// Committing again with no further edit must not see a stale
	// pre-reset baseline and fire a spurious "change".
	doc.DispatchKey("Enter", document.Modifiers{})
	if changed {
		t.Error("\"change\" fired after a reset with no user edit since, want none (stale valueAtFocus baseline)")
	}
}

func TestDispatchClickResetSurvivesTypeAttributeMutation(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><input type="checkbox" id="cb" checked><button type="reset" id="go">Reset</button></form>`)
	cb := doc.GetElementByID("cb")
	btn := doc.GetElementByID("go")

	// Mutate the control's type after parse: applyFormDefaults captured its
	// default under checkedDefaultKind, and triggerReset must keep restoring
	// checkedness from that stored kind rather than re-deriving (and
	// disagreeing on) the control's kind from its now-different live type.
	cb.SetAttribute("type", "text")
	cb.SetChecked(false)
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !cb.Checked() {
		t.Error("cb.Checked() after reset = false, want true (checkedness restored from captured kind, not live type)")
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

func TestDispatchClickCheckboxInDisabledFieldsetDoesNotToggle(t *testing.T) {
	doc := mustParseDoc(t, `<fieldset disabled><input type="checkbox" id="cb"></fieldset>`)
	cb := doc.GetElementByID("cb")
	rect, ok := cb.Rect()
	if !ok {
		t.Fatal("Rect(cb) ok = false, want true")
	}

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if cb.Checked() {
		t.Error("checkbox in a disabled fieldset toggled on click, want no-op")
	}
}

func TestDispatchClickNonFormContentInDisabledFieldsetStillFires(t *testing.T) {
	// Regression test: DispatchClick's disabled-target check used to call
	// isFieldsetDisabled with no restriction to form-associated elements,
	// so a disabled <fieldset> silently swallowed clicks on any nested
	// content at all, not just the form controls real HTML's
	// fieldset-disabling algorithm actually reaches.
	doc := mustParseDoc(t, `<fieldset disabled><div id="d">Click me</div></fieldset>`)
	d := doc.GetElementByID("d")
	clicked := false
	doc.AddEventListener(d, "click", false, func(e *document.Event) { clicked = true })

	rect, ok := d.Rect()
	if !ok {
		t.Fatal("Rect(d) ok = false, want true")
	}
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !clicked {
		t.Error("clicking a <div> inside a disabled <fieldset> did not fire \"click\", want it unaffected since <div> isn't a form-associated element")
	}
}

func TestFocusInDisabledFieldsetFails(t *testing.T) {
	doc := mustParseDoc(t, `<fieldset disabled><input id="in" value="x"></fieldset>`)
	in := doc.GetElementByID("in")

	if in.Focus() {
		t.Error("Focus() on a control inside a disabled fieldset = true, want false")
	}
	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() != nil after a rejected Focus() call")
	}
}

func TestFocusInFieldsetFirstLegendSucceedsDespiteDisabled(t *testing.T) {
	// HTML's one exemption: a fieldset's own disabled attribute doesn't
	// reach into its first <legend> child.
	doc := mustParseDoc(t, `<fieldset disabled><legend><input id="in" value="x"></legend><input id="other" value="y"></fieldset>`)

	if !doc.GetElementByID("in").Focus() {
		t.Error("Focus() on a control inside the fieldset's first legend = false, want true")
	}
	if doc.GetElementByID("other").Focus() {
		t.Error("Focus() on a control outside the legend, still inside the disabled fieldset, = true, want false")
	}
}

func TestFocusInNestedFieldsetsRequiresEscapingEveryDisabledOne(t *testing.T) {
	// An inner fieldset's own legend only exempts its own fieldset's
	// disabling; an outer disabled fieldset still applies.
	doc := mustParseDoc(t, `<fieldset disabled><fieldset disabled><legend><input id="in" value="x"></legend></fieldset></fieldset>`)

	if doc.GetElementByID("in").Focus() {
		t.Error("Focus() inside the inner fieldset's legend, but still inside an outer disabled fieldset, = true, want false")
	}
}

func TestDispatchKeyTypingBlockedInDisabledFieldset(t *testing.T) {
	// A host can focus a field, then have the fieldset around it disabled
	// afterward (SetAttribute, not through Focus's own isFocusable gate);
	// isEditable's fieldset check is the safety net for that case,
	// mirroring the same defensive check it already does for a directly
	// disabled control.
	doc := mustParseDoc(t, `<fieldset id="fs"><input id="in" value=""></fieldset>`)
	in := doc.GetElementByID("in")
	if !in.Focus() {
		t.Fatal("Focus() before disabling = false, want true")
	}
	doc.GetElementByID("fs").SetAttribute("disabled", "")

	doc.DispatchKey("x", document.Modifiers{})
	if in.Value() != "" {
		t.Errorf("Value() = %q after typing into a since-disabled fieldset's control, want unchanged", in.Value())
	}
}

func TestDispatchClickLabelTextTogglesNestedCheckbox(t *testing.T) {
	// The implicit-association case: no for attribute, the checkbox is a
	// descendant of the label. Clicking the label's own text ("Remember
	// me"), not the checkbox glyph itself, must still toggle it.
	doc := mustParseDoc(t, `<label id="lbl"><input type="checkbox" id="cb"> Remember me</label>`)
	cb := doc.GetElementByID("cb")
	lbl := doc.GetElementByID("lbl")
	lblRect, ok := lbl.Rect()
	if !ok {
		t.Fatal("Rect(lbl) ok = false, want true")
	}
	cbRect, _ := cb.Rect()
	textCol := cbRect.Col + cbRect.Width + 1 // inside "Remember me", past the glyph
	if textCol < lblRect.Col || textCol >= lblRect.Col+lblRect.Width {
		t.Fatalf("test setup: textCol %d not inside label rect %+v", textCol, lblRect)
	}

	doc.DispatchClick(lblRect.Row, textCol, document.Modifiers{})
	if !cb.Checked() {
		t.Error("clicking label text did not toggle its nested checkbox")
	}
}

func TestDispatchClickLabelForFocusesNamedControl(t *testing.T) {
	// The explicit-association case: a for attribute pointing at a sibling
	// control's id.
	doc := mustParseDoc(t, `<label for="name" id="lbl">Name:</label> <input type="text" id="name">`)
	lbl := doc.GetElementByID("lbl")
	rect, ok := lbl.Rect()
	if !ok {
		t.Fatal("Rect(lbl) ok = false, want true")
	}

	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	got := doc.FocusedElement()
	if got == nil || got.ID() != "name" {
		t.Errorf("FocusedElement() after clicking label = %v, want \"name\"", got)
	}
}

func TestDispatchClickLabelForRedirectsClickEventToControl(t *testing.T) {
	// The "click" event itself, not just the default action, targets the
	// named control, matching a real browser's forwarded synthetic click:
	// a listener on the label never sees it, a listener on the control does.
	doc := mustParseDoc(t, `<label for="cb" id="lbl">Remember me</label> <input type="checkbox" id="cb">`)
	lbl := doc.GetElementByID("lbl")
	cb := doc.GetElementByID("cb")
	var labelClicked, controlClicked bool
	doc.AddEventListener(lbl, "click", false, func(e *document.Event) { labelClicked = true })
	doc.AddEventListener(cb, "click", false, func(e *document.Event) { controlClicked = true })

	rect, _ := lbl.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if labelClicked {
		t.Error("label's own \"click\" listener fired, want the redirected control to be the sole target")
	}
	if !controlClicked {
		t.Error("named control's \"click\" listener did not fire")
	}
	if !cb.Checked() {
		t.Error("named checkbox not toggled by the redirected click")
	}
}

func TestDispatchClickLabelWithNoAssociationDispatchesOwnClick(t *testing.T) {
	// No for, and no labelable descendant: labelledControl finds nothing,
	// so the label keeps its own "click" dispatch rather than being
	// silently swallowed.
	doc := mustParseDoc(t, `<label id="lbl">Just text</label>`)
	lbl := doc.GetElementByID("lbl")
	clicked := false
	doc.AddEventListener(lbl, "click", false, func(e *document.Event) { clicked = true })

	rect, ok := lbl.Rect()
	if !ok {
		t.Fatal("Rect(lbl) ok = false, want true")
	}
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !clicked {
		t.Error("label with no associated control did not dispatch its own click")
	}
}

func TestDispatchClickLabelForStaleIDFallsBackToOwnClick(t *testing.T) {
	// A for attribute pointing at a nonexistent id is the same as no
	// association at all, not an error.
	doc := mustParseDoc(t, `<label for="ghost" id="lbl">Name:</label>`)
	lbl := doc.GetElementByID("lbl")
	clicked := false
	doc.AddEventListener(lbl, "click", false, func(e *document.Event) { clicked = true })

	rect, _ := lbl.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if !clicked {
		t.Error("label with a stale for= did not fall back to dispatching its own click")
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

// TestDispatchKeyUpFiresEventWithNoDefaultAction checks that DispatchKeyUp
// dispatches a "keyup" Event carrying the same Key and modifier fields
// DispatchKey's "keydown" would, but runs no default action of its own: a
// key name that would type a character or move a caret under DispatchKey
// does neither here.
func TestDispatchKeyUpFiresEventWithNoDefaultAction(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="ab">`)
	a := doc.GetElementByID("a")
	a.Focus()

	var got document.Event
	var fired bool
	doc.AddEventListener(a, "keyup", false, func(e *document.Event) {
		fired = true
		got = *e
	})

	if ok := doc.DispatchKeyUp("x", document.Modifiers{Shift: true}); !ok {
		t.Fatalf("DispatchKeyUp returned false with a focused element")
	}
	if !fired {
		t.Fatalf("keyup listener never ran")
	}
	if got.Type != "keyup" || got.Key != "x" || !got.ShiftKey {
		t.Errorf("keyup event = %+v, want Type=keyup Key=x ShiftKey=true", got)
	}
	if v := a.Value(); v != "ab" {
		t.Errorf("value after DispatchKeyUp(%q) = %q, want unchanged %q", "x", v, "ab")
	}
}

// TestDispatchKeyUpNoFocusReturnsFalse mirrors DispatchKey's own
// nothing-focused behavior.
func TestDispatchKeyUpNoFocusReturnsFalse(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a">`)
	if doc.DispatchKeyUp("x", document.Modifiers{}) {
		t.Errorf("DispatchKeyUp with nothing focused = true, want false")
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

// TestDispatchPasteStripsNewlinesForSingleLineInput pins HTML's value
// sanitization for single-line controls: a multi-line paste into an <input>
// arrives with its CR/LFs removed. Without this the literal "\n" ends up in
// the value, and this renderer honors it as a real line break — tearing the
// input's own line in half.
func TestDispatchPasteStripsNewlinesForSingleLineInput(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="">`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.DispatchPaste("one\ntwo\r\nthree\r")
	if got, want := a.Value(), "onetwothree"; got != want {
		t.Fatalf("value after multi-line paste = %q, want %q", got, want)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n := strings.Count(strings.TrimRight(stripANSI(out), "\n"), "\n"); n != 0 {
		t.Errorf("rendered %d line breaks, want the input to stay on one line:\n%s", n, out)
	}
}

// TestDispatchPasteKeepsNewlinesForTextarea is the other half of
// TestDispatchPasteStripsNewlinesForSingleLineInput: <textarea> is the one
// multi-line text entry, so its pasted newlines must survive.
func TestDispatchPasteKeepsNewlinesForTextarea(t *testing.T) {
	doc := mustParseDoc(t, `<textarea id="a" value=""></textarea>`)
	a := doc.GetElementByID("a")
	a.Focus()

	doc.DispatchPaste("one\ntwo")
	if got, want := a.Value(), "one\ntwo"; got != want {
		t.Fatalf("value after multi-line paste into textarea = %q, want %q", got, want)
	}
}

// TestSetValueStripsNewlinesForSingleLineInput pins that programmatic
// assignment goes through the same sanitization as a paste (real DOM
// sanitizes in the value setter), while SetAttribute stays the raw escape
// hatch.
func TestSetValueStripsNewlinesForSingleLineInput(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value=""><textarea id="t"></textarea>`)
	a := doc.GetElementByID("a")
	a.SetValue("one\ntwo")
	if got, want := a.Value(), "onetwo"; got != want {
		t.Errorf("input SetValue = %q, want %q", got, want)
	}

	tex := doc.GetElementByID("t")
	tex.SetValue("one\ntwo")
	if got, want := tex.Value(), "one\ntwo"; got != want {
		t.Errorf("textarea SetValue = %q, want %q", got, want)
	}

	a.SetAttribute("value", "raw\nvalue")
	if got, want := a.Value(), "raw\nvalue"; got != want {
		t.Errorf("SetAttribute escape hatch = %q, want %q (unsanitized)", got, want)
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

// TestRemoveEventListenerDuringDispatch is a regression test for a bug where
// runDispatch ranged over the live d.listeners[n] slice while
// RemoveEventListener shifted it in place: a self-removing first listener made
// the second one get skipped entirely and the third run twice (observed order
// "one,three,three"). The listener list is snapshotted per node now, matching
// real DOM.
func TestRemoveEventListenerDuringDispatch(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	var order []string
	var h1 document.ListenerHandle
	h1 = doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "one")
		doc.RemoveEventListener(h1)
	})
	doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "two")
	})
	doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "three")
	})

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got, want := strings.Join(order, ","), "one,two,three"; got != want {
		t.Errorf("dispatch order = %q, want %q", got, want)
	}

	// The removal really took effect for the *next* dispatch.
	order = nil
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got, want := strings.Join(order, ","), "two,three"; got != want {
		t.Errorf("second dispatch order = %q, want %q", got, want)
	}
}

// TestRemoveEventListenerDuringDispatchSkipsRemovedListener pins the other
// half of the snapshot rule: a listener removed by an *earlier* listener in
// the same dispatch must not run off the snapshot anyway — real DOM checks
// each listener's "removed" flag as it walks (see hasListener).
func TestRemoveEventListenerDuringDispatchSkipsRemovedListener(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	var order []string
	var h2 document.ListenerHandle
	doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "one")
		doc.RemoveEventListener(h2)
	})
	h2 = doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "two")
	})

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got, want := strings.Join(order, ","), "one"; got != want {
		t.Errorf("dispatch order = %q, want %q (removed listener must not run)", got, want)
	}
}

// TestAddEventListenerDuringDispatchDoesNotRunForSameEvent pins the third
// consequence of snapshotting: a listener added while an event is already
// being dispatched to that node isn't called for that event, only for later
// ones — again matching real DOM.
func TestAddEventListenerDuringDispatchDoesNotRunForSameEvent(t *testing.T) {
	doc := mustParseDoc(t, `<button id="btn">Go</button>`)
	btn := doc.GetElementByID("btn")

	var order []string
	doc.AddEventListener(btn, "click", false, func(e *document.Event) {
		order = append(order, "first")
		doc.AddEventListener(btn, "click", false, func(e *document.Event) {
			order = append(order, "added")
		})
	})

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got, want := strings.Join(order, ","), "first"; got != want {
		t.Errorf("dispatch order = %q, want %q", got, want)
	}
	order = nil
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if got, want := strings.Join(order, ","), "first,added"; got != want {
		t.Errorf("second dispatch order = %q, want %q", got, want)
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
	doc := mustParseDoc(t, `<input type="checkbox" id="cb"><details id="d"><summary id="s">Title</summary></details>`)
	cb := doc.GetElementByID("cb")
	d := doc.GetElementByID("d")
	s := doc.GetElementByID("s")

	var click, focus, toggle *document.Event
	doc.AddEventListener(cb, "click", false, func(e *document.Event) { click = e })
	doc.AddEventListener(cb, "focus", false, func(e *document.Event) { focus = e })
	doc.AddEventListener(d, "toggle", false, func(e *document.Event) { toggle = e })

	rect, _ := cb.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	cb.Focus()
	sRect, _ := s.Rect()
	doc.DispatchClick(sRect.Row, sRect.Col, document.Modifiers{})

	if click == nil || !click.Bubbles || !click.Cancelable {
		t.Errorf("click event Bubbles/Cancelable = %v/%v, want true/true", click.Bubbles, click.Cancelable)
	}
	if focus == nil || !focus.Bubbles || focus.Cancelable {
		t.Errorf("focus event Bubbles/Cancelable = %v/%v, want true/false", focus.Bubbles, focus.Cancelable)
	}
	if toggle == nil || toggle.Bubbles || toggle.Cancelable {
		t.Errorf("toggle event Bubbles/Cancelable = %v/%v, want false/false", toggle.Bubbles, toggle.Cancelable)
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

// TestDispatchKeyShiftTabMovesFocusBackward pins Shift+Tab's default action.
// DispatchKey's Tab case used to call FocusNext unconditionally, so
// Shift+Tab was indistinguishable from Tab and FocusPrev had no key binding
// at all.
func TestDispatchKeyShiftTabMovesFocusBackward(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a"><input id="b"><input id="c">`)
	c := doc.GetElementByID("c")

	c.Focus()
	doc.DispatchKey("Tab", document.Modifiers{Shift: true})
	if focusedID(doc) != "b" {
		t.Fatalf("Shift+Tab from c focused %q, want b", focusedID(doc))
	}
	doc.DispatchKey("Tab", document.Modifiers{Shift: true})
	if focusedID(doc) != "a" {
		t.Fatalf("Shift+Tab from b focused %q, want a", focusedID(doc))
	}
	// ...and wraps around to the last control, same as Tab wraps forward.
	doc.DispatchKey("Tab", document.Modifiers{Shift: true})
	if focusedID(doc) != "c" {
		t.Fatalf("Shift+Tab from a focused %q, want c (wrap-around)", focusedID(doc))
	}
	// Plain Tab still goes forward.
	doc.DispatchKey("Tab", document.Modifiers{})
	if focusedID(doc) != "a" {
		t.Fatalf("Tab from c focused %q, want a", focusedID(doc))
	}
}

// focusedID returns the currently focused element's id, or "" if nothing is
// focused — a readability helper for the focus-order assertions above.
func focusedID(doc *document.Document) string {
	el := doc.FocusedElement()
	if el == nil {
		return ""
	}
	id, _ := el.GetAttribute("id")
	return id
}

// TestReadonlyTextEntryRejectsEdits pins HTML's readonly semantics: the field
// still focuses, still moves its caret, still selects, and still submits with
// its form — but no user edit path (typing, Backspace/Delete, Enter in a
// textarea, paste) changes its value, and none of them fire "input". Before
// this, "readonly" appeared nowhere in the package and every one of these
// mutated the field.
func TestReadonlyTextEntryRejectsEdits(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hello" readonly>`)
	a := doc.GetElementByID("a")
	a.Focus()

	inputs := 0
	doc.AddEventListener(a, "input", false, func(e *document.Event) { inputs++ })

	a.SetSelectionRange(1, 3)
	doc.DispatchKey("x", document.Modifiers{})
	doc.DispatchKey("Backspace", document.Modifiers{})
	doc.DispatchKey("Delete", document.Modifiers{})
	doc.DispatchPaste("zzz")
	if got := a.Value(); got != "hello" {
		t.Errorf("value after edits to a readonly field = %q, want %q", got, "hello")
	}
	if inputs != 0 {
		t.Errorf(`fired %d "input" events on a readonly field, want 0`, inputs)
	}

	// Caret movement and selection still work — readonly is not disabled.
	a.SetSelectionRange(0, 0)
	doc.DispatchKey("End", document.Modifiers{})
	if got := a.SelectionEnd(); got != 5 {
		t.Errorf("End on a readonly field left caret at %d, want 5", got)
	}
	doc.DispatchKey("a", document.Modifiers{Ctrl: true})
	if start, end := a.SelectionStart(), a.SelectionEnd(); start != 0 || end != 5 {
		t.Errorf("Ctrl+A on a readonly field selected [%d,%d), want [0,5)", start, end)
	}
}

// TestReadonlyTextareaSwallowsEnter pins that Enter in a readonly <textarea>
// does nothing at all — in particular it must not fall through to the
// single-line implicit-submit branch, since a <textarea> never
// implicit-submits whether it's editable or not.
func TestReadonlyTextareaSwallowsEnter(t *testing.T) {
	doc := mustParseDoc(t, `<form id="f"><textarea id="t" value="ab" readonly></textarea></form>`)
	tex := doc.GetElementByID("t")
	tex.Focus()

	submits := 0
	doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *document.Event) { submits++ })

	doc.DispatchKey("Enter", document.Modifiers{})
	if got := tex.Value(); got != "ab" {
		t.Errorf("readonly textarea value after Enter = %q, want %q", got, "ab")
	}
	if submits != 0 {
		t.Errorf("readonly textarea Enter fired %d submits, want 0", submits)
	}
}

// TestDispatchCutOnReadonlyCopiesWithoutRemoving pins cut's degrade-to-copy
// behavior on a non-editable field: the host still gets the text for the
// system clipboard, but the field keeps it.
func TestDispatchCutOnReadonlyCopiesWithoutRemoving(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="secret" readonly>`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(0, 3)

	text, ok := doc.DispatchCut()
	if !ok {
		t.Fatal("DispatchCut returned ok = false with a focused element")
	}
	if text != "sec" {
		t.Errorf("clipboard text = %q, want %q", text, "sec")
	}
	if got := a.Value(); got != "secret" {
		t.Errorf("readonly value after cut = %q, want it unchanged", got)
	}
}

// TestDisabledFocusedTextEntryRejectsEdits pins the same gate for an element
// disabled *after* it was focused — isFocusable already keeps focus off a
// disabled control, but a host can disable the one that already has focus.
func TestDisabledFocusedTextEntryRejectsEdits(t *testing.T) {
	doc := mustParseDoc(t, `<input id="a" value="hi">`)
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetAttribute("disabled", "")

	doc.DispatchKey("x", document.Modifiers{})
	if got := a.Value(); got != "hi" {
		t.Errorf("value after typing into a disabled field = %q, want %q", got, "hi")
	}
}
