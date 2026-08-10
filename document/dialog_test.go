package document_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
)

func mustParseDialogDoc(t *testing.T, htmlStr string) (*document.Document, *document.Element) {
	t.Helper()
	doc := mustParseDoc(t, htmlStr)
	dlg := doc.GetElementByID("d")
	if dlg == nil {
		t.Fatalf("no element with id=d in %s", htmlStr)
	}
	return doc, dlg
}

func TestDialogShowOpensAndRendersContent(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d">Hello</dialog>`)

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(out), "Hello") {
		t.Fatalf("closed dialog rendered its content: %q", stripANSI(out))
	}
	if dlg.Open() {
		t.Fatal("dialog reported open before Show")
	}

	if !dlg.Show() {
		t.Fatal("Show returned false on a closed dialog")
	}
	if !dlg.Open() {
		t.Fatal("dialog not open after Show")
	}
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "Hello") {
		t.Fatalf("open dialog did not render its content: %q", stripANSI(out))
	}
}

func TestDialogShowOnOpenDialogIsNoOp(t *testing.T) {
	_, dlg := mustParseDialogDoc(t, `<dialog id="d" open>Hi</dialog>`)
	if dlg.Show() {
		t.Fatal("Show returned true for an already-open dialog")
	}
}

func TestDialogShowOnNonDialogIsNoOp(t *testing.T) {
	doc := mustParseDoc(t, `<div id="x">Hi</div>`)
	div := doc.GetElementByID("x")
	if div.Show() {
		t.Fatal("Show returned true for a <div>")
	}
	if div.Open() {
		t.Fatal("Show set open on a <div>")
	}
}

func TestDialogCloseFiresCloseAndRecordsReturnValue(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d" open>Hi</dialog>`)

	var seen []string
	doc.AddEventListener(dlg, "close", false, func(ev *document.Event) {
		// Read through the DOM, not the event: this pins that "close"
		// fires after the state has settled.
		seen = append(seen, dlg.ReturnValue())
		if dlg.Open() {
			t.Error("dialog still open inside its own close listener")
		}
	})

	if !dlg.CloseWith("ok") {
		t.Fatal("CloseWith returned false on an open dialog")
	}
	if dlg.Open() {
		t.Fatal("dialog still open after CloseWith")
	}
	if got := dlg.ReturnValue(); got != "ok" {
		t.Fatalf("ReturnValue = %q, want %q", got, "ok")
	}
	if len(seen) != 1 || seen[0] != "ok" {
		t.Fatalf("close listener saw %v, want one call with %q", seen, "ok")
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(out), "Hi") {
		t.Fatalf("closed dialog still rendered: %q", stripANSI(out))
	}
}

func TestDialogCloseOnClosedDialogFiresNothing(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d">Hi</dialog>`)
	fired := 0
	doc.AddEventListener(dlg, "close", false, func(ev *document.Event) { fired++ })

	if dlg.Close() {
		t.Fatal("Close returned true for an already-closed dialog")
	}
	if fired != 0 {
		t.Fatalf("close fired %d times on an already-closed dialog, want 0", fired)
	}
}

func TestDialogCloseEventDoesNotBubble(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<div id="outer"><dialog id="d" open>Hi</dialog></div>`)
	outer := doc.GetElementByID("outer")

	onTarget, onAncestor := 0, 0
	doc.AddEventListener(dlg, "close", false, func(ev *document.Event) { onTarget++ })
	doc.AddEventListener(outer, "close", false, func(ev *document.Event) { onAncestor++ })

	dlg.Close()

	if onTarget != 1 {
		t.Fatalf("target close listener fired %d times, want 1", onTarget)
	}
	if onAncestor != 0 {
		t.Fatalf("ancestor close listener fired %d times, want 0 (close must not bubble)", onAncestor)
	}
}

func TestDialogSetReturnValueWithoutClosing(t *testing.T) {
	_, dlg := mustParseDialogDoc(t, `<dialog id="d" open>Hi</dialog>`)
	dlg.SetReturnValue("draft")
	if got := dlg.ReturnValue(); got != "draft" {
		t.Fatalf("ReturnValue = %q, want %q", got, "draft")
	}
	if !dlg.Open() {
		t.Fatal("SetReturnValue closed the dialog")
	}
}

func TestDialogReturnValueAttributeIsNotSerialized(t *testing.T) {
	_, dlg := mustParseDialogDoc(t, `<dialog id="d" open>Hi</dialog>`)
	dlg.CloseWith("secret")

	out, err := dlg.OuterHTML()
	if err != nil {
		t.Fatalf("OuterHTML: %v", err)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "data-htmlterm") {
		t.Fatalf("reserved return-value attribute leaked into OuterHTML: %q", out)
	}
}

// --- modal ---

// modalDocHTML is one modal dialog with a focusable control inside it, and
// one outside it, which is what every containment test below needs.
const modalDocHTML = `<p>page text</p><button id="outside">Outside</button>` +
	`<dialog id="d"><p>Delete?</p><button id="ok">OK</button></dialog>`

func mustParseModalDoc(t *testing.T) (*document.Document, *document.Element) {
	t.Helper()
	doc, err := document.ParseDocument(modalDocHTML, htmlterm.Options{Width: 34, Height: 12})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return doc, doc.GetElementByID("d")
}

func TestShowModalCentersAndShrinksToFit(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := dlg.Rect()
	if !ok {
		t.Fatal("no Rect for the open modal")
	}
	// The dialog rides position:fixed via the UA `dialog:modal` rule, so it
	// shrinks to its own content rather than filling the 34-wide viewport,
	// and auto margins center it on both axes.
	if rect.Width >= 34 {
		t.Errorf("Rect.Width = %d, want < 34 (shrink-to-fit, not filling the viewport)", rect.Width)
	}
	if wantCol := (34 - rect.Width) / 2; rect.Col != wantCol {
		t.Errorf("Rect.Col = %d, want %d (centered horizontally)", rect.Col, wantCol)
	}
	if wantRow := (12 - rect.Height) / 2; rect.Row != wantRow {
		t.Errorf("Rect.Row = %d, want %d (centered vertically)", rect.Row, wantRow)
	}
}

func TestShowModalIsOutOfNormalFlow(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	before, _ := doc.Render()
	beforeRect, _ := doc.GetElementByID("outside").Rect()

	dlg.ShowModal()
	after, _ := doc.Render()
	afterRect, _ := doc.GetElementByID("outside").Rect()

	if beforeRect.Row != afterRect.Row {
		t.Errorf("the button outside moved from row %d to %d; a modal must reserve no space in normal flow",
			beforeRect.Row, afterRect.Row)
	}
	if strings.Contains(stripANSI(before), "Delete?") {
		t.Fatal("closed dialog rendered its content")
	}
	if !strings.Contains(stripANSI(after), "Delete?") {
		t.Fatal("open modal did not render its content")
	}
}

func TestShowModalFocusesFirstControlInside(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	el := doc.FocusedElement()
	if el == nil || el.ID() != "ok" {
		t.Fatalf("focused = %v, want the modal's own first focusable control (ok)", focusedID(doc))
	}
}

func TestShowModalTrapsTabOrder(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	// Only one focusable control is inside, so every Tab must land back on
	// it rather than escaping to the button outside.
	for i := range 3 {
		doc.FocusNext()
		if got := focusedID(doc); got != "ok" {
			t.Fatalf("after %d Tab presses focus = %q, want it trapped on \"ok\"", i+1, got)
		}
	}
	doc.FocusPrev()
	if got := focusedID(doc); got != "ok" {
		t.Fatalf("after Shift+Tab focus = %q, want it trapped on \"ok\"", got)
	}
}

func TestShowModalRefusesFocusOutside(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	if doc.GetElementByID("outside").Focus() {
		t.Error("Element.Focus() succeeded on a control outside the open modal")
	}
	if got := focusedID(doc); got != "ok" {
		t.Errorf("focus moved to %q; it should have stayed inside the modal", got)
	}
}

func TestShowModalSwallowsClicksOutside(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	outside := doc.GetElementByID("outside")
	rect, ok := outside.Rect()
	if !ok {
		t.Fatal("no Rect for the outside button")
	}
	dlg.ShowModal()

	clicks := 0
	doc.AddEventListener(outside, "click", false, func(ev *document.Event) { clicks++ })

	if doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{}) {
		t.Error("DispatchClick reported a hit outside an open modal")
	}
	if clicks != 0 {
		t.Errorf("click listener outside the modal fired %d times, want 0", clicks)
	}
}

func TestShowModalStillDeliversClicksInside(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	ok := doc.GetElementByID("ok")
	rect, found := ok.Rect()
	if !found {
		t.Fatal("no Rect for the control inside the modal")
	}

	clicks := 0
	doc.AddEventListener(ok, "click", false, func(ev *document.Event) { clicks++ })
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if clicks != 1 {
		t.Errorf("click listener inside the modal fired %d times, want 1", clicks)
	}
}

func TestModalEscapeFiresCancelThenCloses(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()

	var order []string
	doc.AddEventListener(dlg, "cancel", false, func(ev *document.Event) { order = append(order, "cancel") })
	doc.AddEventListener(dlg, "close", false, func(ev *document.Event) { order = append(order, "close") })

	doc.DispatchKey("Escape", document.Modifiers{})

	if dlg.Open() {
		t.Error("modal still open after Escape")
	}
	if len(order) != 2 || order[0] != "cancel" || order[1] != "close" {
		t.Errorf("event order = %v, want [cancel close]", order)
	}
}

func TestModalEscapeCancelPreventDefaultKeepsItOpen(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	dlg.ShowModal()

	closed := 0
	doc.AddEventListener(dlg, "cancel", false, func(ev *document.Event) { ev.PreventDefault() })
	doc.AddEventListener(dlg, "close", false, func(ev *document.Event) { closed++ })

	doc.DispatchKey("Escape", document.Modifiers{})

	if !dlg.Open() {
		t.Error("modal closed despite the cancel event being prevented")
	}
	if closed != 0 {
		t.Errorf("close fired %d times after a prevented cancel, want 0", closed)
	}
}

func TestEscapeClosesSelectPopupBeforeModal(t *testing.T) {
	// Escape unwinds one layer per press: an open <select> popup inside the
	// modal first, the modal itself only on the next press.
	doc, err := document.ParseDocument(
		`<dialog id="d"><select id="s"><option>a</option><option>b</option></select></dialog>`,
		htmlterm.Options{Width: 30, Height: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	dlg := doc.GetElementByID("d")
	dlg.ShowModal()

	doc.DispatchKey("Enter", document.Modifiers{}) // opens the select popup
	out, _ := doc.Render()
	if !strings.Contains(stripANSI(out), "▸") {
		t.Fatal("select popup did not open inside the modal")
	}

	doc.DispatchKey("Escape", document.Modifiers{})
	if !dlg.Open() {
		t.Fatal("the first Escape closed the modal; it should have closed the select popup only")
	}
	doc.DispatchKey("Escape", document.Modifiers{})
	if dlg.Open() {
		t.Error("the second Escape did not close the modal")
	}
}

func TestNestedModalsUnwindInShowModalOrder(t *testing.T) {
	doc, err := document.ParseDocument(
		`<dialog id="a"><button id="ab">A</button></dialog><dialog id="b"><button id="bb">B</button></dialog>`,
		htmlterm.Options{Width: 30, Height: 12})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	first, second := doc.GetElementByID("a"), doc.GetElementByID("b")

	first.ShowModal()
	second.ShowModal()
	if got := focusedID(doc); got != "bb" {
		t.Fatalf("focus = %q after opening the second modal, want it inside that one (bb)", got)
	}

	// The topmost is the most recently shown, not the first in document
	// order, so Escape must take them off in reverse order.
	doc.DispatchKey("Escape", document.Modifiers{})
	if second.Open() {
		t.Error("the second modal stayed open")
	}
	if !first.Open() {
		t.Fatal("the first modal closed too; Escape must unwind one layer at a time")
	}
	doc.DispatchKey("Escape", document.Modifiers{})
	if first.Open() {
		t.Error("the first modal did not close on the second Escape")
	}
}

func TestModalCloseRestoresFocusAndClicksOutside(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	outside := doc.GetElementByID("outside")
	dlg.ShowModal()
	dlg.Close()

	if !outside.Focus() {
		t.Error("Element.Focus() outside still refused after the modal closed")
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, _ := outside.Rect()
	clicks := 0
	doc.AddEventListener(outside, "click", false, func(ev *document.Event) { clicks++ })
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})
	if clicks != 1 {
		t.Errorf("click outside fired %d times after the modal closed, want 1", clicks)
	}
}

func TestModalMarkerAttributeIsNotSerialized(t *testing.T) {
	_, dlg := mustParseModalDoc(t)
	dlg.ShowModal()
	out, err := dlg.OuterHTML()
	if err != nil {
		t.Fatalf("OuterHTML: %v", err)
	}
	if strings.Contains(out, "data-htmlterm") {
		t.Errorf("reserved modal marker leaked into OuterHTML: %q", out)
	}
}

func TestModalPseudoClassMatchesOnlyWhileModal(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	if doc.QuerySelector("dialog:modal") != nil {
		t.Error("dialog:modal matched a closed dialog")
	}
	dlg.Show()
	if doc.QuerySelector("dialog:modal") != nil {
		t.Error("dialog:modal matched a non-modal open dialog")
	}
	dlg.Close()
	dlg.ShowModal()
	if el := doc.QuerySelector("dialog:modal"); el == nil || el.ID() != "d" {
		t.Error("dialog:modal did not match an open modal dialog")
	}
	dlg.Close()
	if doc.QuerySelector("dialog:modal") != nil {
		t.Error("dialog:modal still matched after the modal closed")
	}
}

// --- regressions ---

func TestModalWithNothingFocusableStillClosesOnEscape(t *testing.T) {
	// A content-only modal focuses nothing, and it swallows every click
	// outside itself, so if Escape didn't reach it the app would be wedged
	// with no way out. DispatchKey normally bails when nothing is focused.
	doc, err := document.ParseDocument(
		`<p>page</p><dialog id="d"><p>text only, nothing to focus</p></dialog>`,
		htmlterm.Options{Width: 30, Height: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	dlg := doc.GetElementByID("d")
	dlg.ShowModal()

	if doc.FocusedElement() != nil {
		t.Fatalf("focus = %q, want nothing focusable inside this dialog", focusedID(doc))
	}
	if !doc.DispatchKey("Escape", document.Modifiers{}) {
		t.Error("DispatchKey(Escape) reported unhandled with a modal open and nothing focused")
	}
	if dlg.Open() {
		t.Error("modal with no focusable content could not be closed by Escape")
	}
}

func TestClosedDialogContentIsNotFocusable(t *testing.T) {
	// A closed <dialog> is display:none via the UA stylesheet, so Tab must
	// not stop on a control inside it.
	doc, err := document.ParseDocument(
		`<input id="a"><dialog id="d"><button id="ok">x</button></dialog><input id="b">`,
		htmlterm.Options{Width: 30, Height: 8})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var order []string
	for range 3 {
		doc.FocusNext()
		order = append(order, focusedID(doc))
	}
	want := []string{"a", "b", "a"}
	if !slices.Equal(order, want) {
		t.Errorf("tab order = %v, want %v (a control inside a closed dialog renders nothing)", order, want)
	}
	if doc.GetElementByID("ok").Focus() {
		t.Error("Element.Focus() succeeded on a control inside a closed dialog")
	}
}

func TestClosingDialogRestoresPreviousFocus(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	outside := doc.GetElementByID("outside")
	outside.Focus()

	dlg.ShowModal()
	if got := focusedID(doc); got != "ok" {
		t.Fatalf("focus = %q after ShowModal, want it inside the dialog", got)
	}
	dlg.Close()

	// Leaving focus inside a now-display:none dialog would keep delivering
	// keystrokes to an element rendering nothing.
	if got := focusedID(doc); got != "outside" {
		t.Errorf("focus = %q after close, want it restored to %q", got, "outside")
	}
}

func TestClosingDialogDropsFocusWhenPreviousIsGone(t *testing.T) {
	doc, dlg := mustParseModalDoc(t)
	outside := doc.GetElementByID("outside")
	outside.Focus()
	dlg.ShowModal()
	outside.SetAttribute("disabled", "")
	dlg.Close()

	if el := doc.FocusedElement(); el != nil {
		// Anything but nil means focus is on a disabled or hidden element.
		t.Errorf("focus = %q after close, want it dropped since the previous element is no longer focusable", el.ID())
	}
}

func TestNestedModalsPaintInShowModalOrder(t *testing.T) {
	// Both modals get the same z-index from the UA stylesheet, so without an
	// explicit top-layer tiebreak they would paint in document order and the
	// dialog opened second, which owns focus and Escape, would be painted
	// over by the one that happens to come first in the source.
	doc, err := document.ParseDocument(
		`<dialog id="b">BBBB</dialog><dialog id="a">AAAA</dialog>`,
		htmlterm.Options{Width: 20, Height: 8})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	doc.GetElementByID("a").ShowModal()
	doc.GetElementByID("b").ShowModal()

	out, _ := doc.Render()
	if !strings.Contains(stripANSI(out), "BBBB") {
		t.Errorf("the modal opened second is not visible:\n%s", stripANSI(out))
	}
}

func TestBackdropCoversRowsTheModalAdded(t *testing.T) {
	// With no Options.Height the document grows to fit the modal. The
	// backdrop has to cover the grown rows too, not just the ones that
	// existed when it painted.
	doc, err := document.ParseDocument(
		`<p>x</p><dialog id="d"><p>a</p><p>b</p><p>c</p></dialog>`,
		htmlterm.Options{Width: 20, CSS: `dialog::backdrop{background-color:#000080}`, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	doc.GetElementByID("d").ShowModal()
	out, _ := doc.Render()

	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.Contains(line, "\x1b[48;2;0;0;128m") {
			t.Errorf("row %d has no backdrop fill: %q", i, line)
		}
	}
}

func TestModalSwallowsWheelOutside(t *testing.T) {
	doc, err := document.ParseDocument(
		`<div id="pane" style="height:3;overflow-y:scroll">l1<br>l2<br>l3<br>l4<br>l5<br>l6</div>`+
			`<dialog id="d"><button id="k">k</button></dialog>`,
		htmlterm.Options{Width: 24, Height: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	doc.GetElementByID("d").ShowModal()
	if doc.DispatchWheel(1, 1, 0, 1) {
		t.Error("a wheel notch scrolled a container behind an open modal")
	}
}

func backdropDoc(t *testing.T, css string) (*document.Document, *document.Element) {
	t.Helper()
	doc, err := document.ParseDocument(
		`<p>page text one</p><dialog id="d">Hi</dialog>`,
		htmlterm.Options{Width: 20, Height: 5, CSS: css, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return doc, doc.GetElementByID("d")
}

func TestModalHasNoBackdropByDefault(t *testing.T) {
	doc, dlg := backdropDoc(t, "")
	dlg.ShowModal()
	out, _ := doc.Render()
	// Nothing is painted behind the dialog without an explicit ::backdrop
	// rule, so the page underneath stays readable. A default scrim would
	// have to be opaque here, and would destroy it.
	if !strings.Contains(stripANSI(out), "page text one") {
		t.Errorf("content behind the modal was covered with no ::backdrop rule set:\n%q", stripANSI(out))
	}
}

func TestModalBackdropFillsViewportWhenStyled(t *testing.T) {
	doc, dlg := backdropDoc(t, `dialog::backdrop { background-color: #000080; }`)
	dlg.ShowModal()
	out, _ := doc.Render()

	const navy = "\x1b[48;2;0;0;128m"
	if !strings.Contains(out, navy) {
		t.Fatalf("::backdrop background-color was not painted:\n%q", out)
	}
	if strings.Contains(stripANSI(out), "page text one") {
		t.Error("content behind the modal survived an opaque ::backdrop fill")
	}
	// Every viewport row is covered, not just the dialog's own band.
	for i, line := range strings.Split(out, "\n")[:5] {
		if !strings.Contains(line, navy) {
			t.Errorf("row %d has no backdrop fill: %q", i, line)
		}
	}
	// The dialog still paints on top of its own backdrop.
	if !strings.Contains(stripANSI(out), "Hi") {
		t.Error("the backdrop covered the dialog it belongs to")
	}
}

func TestNonModalDialogGetsNoBackdrop(t *testing.T) {
	doc, dlg := backdropDoc(t, `dialog::backdrop { background-color: #000080; }`)
	dlg.Show()
	out, _ := doc.Render()
	if strings.Contains(out, "\x1b[48;2;0;0;128m") {
		t.Error("a non-modal dialog painted a ::backdrop; only top-layer elements get one")
	}
}

func TestDialogFormMethodDialogSubmitClosesWithSubmitterValue(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d" open><form method="dialog"><button id="b" type="submit" value="confirm">OK</button></form></dialog>`)
	btn := doc.GetElementByID("b")

	submits := 0
	doc.AddEventListener(dlg, "submit", false, func(ev *document.Event) { submits++ })

	rect, ok := btn.Rect()
	if !ok {
		t.Fatal("no Rect for the submit button")
	}
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if submits != 1 {
		t.Fatalf("submit fired %d times, want 1", submits)
	}
	if dlg.Open() {
		t.Fatal("dialog still open after a method=dialog submit")
	}
	if got := dlg.ReturnValue(); got != "confirm" {
		t.Fatalf("ReturnValue = %q, want the submitter's value %q", got, "confirm")
	}
}

func TestDialogFormMethodDialogSubmitPreventedKeepsDialogOpen(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d" open><form method="dialog"><button id="b" type="submit" value="confirm">OK</button></form></dialog>`)
	btn := doc.GetElementByID("b")
	doc.AddEventListener(dlg, "submit", false, func(ev *document.Event) { ev.PreventDefault() })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if !dlg.Open() {
		t.Fatal("dialog closed despite the submit event being prevented")
	}
	if got := dlg.ReturnValue(); got != "" {
		t.Fatalf("ReturnValue = %q, want empty after a prevented submit", got)
	}
}

func TestDialogFormWithoutMethodDialogDoesNotClose(t *testing.T) {
	doc, dlg := mustParseDialogDoc(t, `<dialog id="d" open><form><button id="b" type="submit" value="x">OK</button></form></dialog>`)
	btn := doc.GetElementByID("b")

	submits := 0
	doc.AddEventListener(dlg, "submit", false, func(ev *document.Event) { submits++ })

	rect, _ := btn.Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if submits != 1 {
		t.Fatalf("submit fired %d times, want 1", submits)
	}
	if !dlg.Open() {
		t.Fatal("an ordinary form's submit closed the surrounding dialog")
	}
}
