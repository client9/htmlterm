package document_test

import (
	"strings"
	"testing"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
)

const fruitDatalistHTML = `<form id="f"><input id="i" list="l"><datalist id="l">` +
	`<option value="apple"><option value="Apricot"><option value="banana">` +
	`</datalist></form>`

func mustParseDatalistDoc(t *testing.T, htmlStr string) (*document.Document, *document.Element) {
	t.Helper()
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 30, Height: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	in := doc.GetElementByID("i")
	if in == nil {
		t.Fatal("no element with id=i")
	}
	in.Focus()
	return doc, in
}

// suggestions returns the popup rows currently rendered, trimmed, in order.
// Rows are everything below the field's own line, which holds whatever is
// typed rather than a suggestion. The cutoff comes from the input's own Rect
// rather than assuming it sits on line 0, since some cases below put content
// above it.
func suggestions(t *testing.T, doc *document.Document, in *document.Element) []string {
	t.Helper()
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := in.Rect()
	if !ok {
		t.Fatal("no Rect for the input")
	}
	var rows []string
	for i, line := range strings.Split(stripANSI(out), "\n") {
		if i <= rect.Row {
			continue
		}
		if s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "▸")); s != "" {
			rows = append(rows, s)
		}
	}
	return rows
}

func typeKeys(doc *document.Document, keys string) {
	for _, r := range keys {
		doc.DispatchKey(string(r), document.Modifiers{})
	}
}

func TestDatalistTypingFiltersByPrefix(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Fatalf("suggestions before typing = %v, want none", got)
	}

	typeKeys(doc, "a")
	got := suggestions(t, doc, in)
	want := []string{"apple", "Apricot"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("after \"a\" suggestions = %v, want %v (banana doesn't match the prefix)", got, want)
	}

	typeKeys(doc, "p")
	got = suggestions(t, doc, in)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("after \"ap\" suggestions = %v, want %v", got, want)
	}

	typeKeys(doc, "r")
	got = suggestions(t, doc, in)
	if len(got) != 1 || got[0] != "Apricot" {
		t.Errorf("after \"apr\" suggestions = %v, want just Apricot (matching is case-insensitive)", got)
	}
}

func TestDatalistPrefixNotSubstring(t *testing.T) {
	doc, in := mustParseDatalistDoc(t,
		`<input id="i" list="l"><datalist id="l"><option value="pineapple"><option value="apple"></datalist>`)
	typeKeys(doc, "app")
	got := suggestions(t, doc, in)
	if len(got) != 1 || got[0] != "apple" {
		t.Errorf("suggestions = %v, want just apple (prefix match, not substring)", got)
	}
}

func TestDatalistNoMatchClosesPopup(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	typeKeys(doc, "zz")
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want none once nothing matches", got)
	}
	if in.Value() != "zz" {
		t.Errorf("Value = %q, want the typed text left alone", in.Value())
	}
	// Deleting back to a matching prefix brings the popup back.
	doc.DispatchKey("Backspace", document.Modifiers{})
	doc.DispatchKey("Backspace", document.Modifiers{})
	typeKeys(doc, "b")
	if got := suggestions(t, doc, in); len(got) != 1 || got[0] != "banana" {
		t.Errorf("suggestions = %v, want banana after deleting back and retyping", got)
	}
}

func TestDatalistArrowDownOpensClosedPopup(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	got := suggestions(t, doc, in)
	if len(got) != 3 {
		t.Errorf("suggestions = %v, want all three (an empty field matches everything)", got)
	}
}

func TestDatalistArrowUpDoesNotOpenClosedPopup(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	doc.DispatchKey("ArrowUp", document.Modifiers{})
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want none (only ArrowDown opens a closed popup)", got)
	}
}

func TestDatalistArrowKeysMoveHighlightClamped(t *testing.T) {
	doc, _ := mustParseDatalistDoc(t, fruitDatalistHTML)
	typeKeys(doc, "a")

	highlighted := func() string {
		out, _ := doc.Render()
		for _, line := range strings.Split(stripANSI(out), "\n") {
			if s := strings.TrimSpace(line); strings.HasPrefix(s, "▸") {
				return strings.TrimSpace(strings.TrimPrefix(s, "▸"))
			}
		}
		return ""
	}
	if got := highlighted(); got != "" {
		t.Errorf("highlight = %q before any arrow key, want none", got)
	}
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	if got := highlighted(); got != "apple" {
		t.Errorf("highlight = %q after one ArrowDown, want apple", got)
	}
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	if got := highlighted(); got != "Apricot" {
		t.Errorf("highlight = %q after two, want Apricot", got)
	}
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	if got := highlighted(); got != "Apricot" {
		t.Errorf("highlight = %q past the end, want it clamped to Apricot", got)
	}
	doc.DispatchKey("ArrowUp", document.Modifiers{})
	doc.DispatchKey("ArrowUp", document.Modifiers{})
	if got := highlighted(); got != "apple" {
		t.Errorf("highlight = %q past the start, want it clamped to apple", got)
	}
}

func TestDatalistEnterCommitsHighlightedFiringInputThenChange(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	var order []string
	doc.AddEventListener(in, "input", false, func(e *document.Event) { order = append(order, "input") })
	doc.AddEventListener(in, "change", false, func(e *document.Event) { order = append(order, "change") })

	typeKeys(doc, "a")
	order = nil // discard the events typing itself produced
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	doc.DispatchKey("Enter", document.Modifiers{})

	if in.Value() != "Apricot" {
		t.Errorf("Value = %q, want the highlighted suggestion", in.Value())
	}
	if strings.Join(order, ",") != "input,change" {
		t.Errorf("events = %v, want [input change]", order)
	}
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want the popup closed after committing", got)
	}
	if start, end := in.SelectionStart(), in.SelectionEnd(); start != 7 || end != 7 {
		t.Errorf("selection = (%d,%d), want the caret collapsed to the end (7,7)", start, end)
	}
}

func TestDatalistEnterWithNoHighlightSubmitsForm(t *testing.T) {
	// An open popup that the user hasn't browsed into must not swallow Enter:
	// they typed something and pressed Enter to submit it.
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	submitted := 0
	doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *document.Event) { submitted++ })

	typeKeys(doc, "a")
	doc.DispatchKey("Enter", document.Modifiers{})

	if submitted != 1 {
		t.Errorf("submit fired %d times, want 1", submitted)
	}
	if in.Value() != "a" {
		t.Errorf("Value = %q, want the typed text uncommitted", in.Value())
	}
}

func TestDatalistEscapeClosesWithoutCommitting(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	typeKeys(doc, "a")
	doc.DispatchKey("ArrowDown", document.Modifiers{})
	doc.DispatchKey("Escape", document.Modifiers{})

	if in.Value() != "a" {
		t.Errorf("Value = %q, want the typed text, not the highlighted suggestion", in.Value())
	}
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want the popup closed", got)
	}
}

func TestDatalistEscapeClosesPopupBeforeModal(t *testing.T) {
	// Escape unwinds one layer per press, the same rule the <select> popup
	// follows inside a modal.
	doc, err := document.ParseDocument(
		`<dialog id="d"><input id="i" list="l"><datalist id="l"><option value="apple"></datalist></dialog>`,
		htmlterm.Options{Width: 30, Height: 12})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	dlg := doc.GetElementByID("d")
	in := doc.GetElementByID("i")
	dlg.ShowModal()
	typeKeys(doc, "a")
	if got := suggestions(t, doc, in); len(got) == 0 {
		t.Fatal("popup did not open inside the modal")
	}

	doc.DispatchKey("Escape", document.Modifiers{})
	if !dlg.Open() {
		t.Fatal("the first Escape closed the modal; it should have closed the popup only")
	}
	doc.DispatchKey("Escape", document.Modifiers{})
	if dlg.Open() {
		t.Error("the second Escape did not close the modal")
	}
}

func TestDatalistClickOnSuggestionCommits(t *testing.T) {
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	typeKeys(doc, "a")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Second suggestion row: one line below the field, plus one for the
	// first row.
	rect, ok := in.Rect()
	if !ok {
		t.Fatal("no Rect for the input")
	}
	doc.DispatchClick(rect.Row+2, rect.Col+2, document.Modifiers{})

	if in.Value() != "Apricot" {
		t.Errorf("Value = %q, want the clicked suggestion", in.Value())
	}
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want the popup closed after a pick", got)
	}
}

func TestDatalistClickOutsideDismisses(t *testing.T) {
	doc, in := mustParseDatalistDoc(t,
		`<p id="p">elsewhere</p><input id="i" list="l"><datalist id="l"><option value="apple"></datalist>`)
	typeKeys(doc, "a")
	if got := suggestions(t, doc, in); len(got) == 0 {
		t.Fatal("popup did not open")
	}
	rect, _ := doc.GetElementByID("p").Rect()
	doc.DispatchClick(rect.Row, rect.Col, document.Modifiers{})

	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want the popup dismissed by a click elsewhere", got)
	}
	if in.Value() != "a" {
		t.Errorf("Value = %q, want it untouched by the dismissal", in.Value())
	}
}

func TestDatalistBlurDismisses(t *testing.T) {
	doc, in := mustParseDatalistDoc(t,
		`<input id="i" list="l"><datalist id="l"><option value="apple"></datalist><button id="b">x</button>`)
	typeKeys(doc, "a")
	if got := suggestions(t, doc, in); len(got) == 0 {
		t.Fatal("popup did not open")
	}
	doc.GetElementByID("b").Focus()
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want the popup closed once focus left the field", got)
	}
}

func TestDatalistSkipsDisabledOptions(t *testing.T) {
	doc, in := mustParseDatalistDoc(t,
		`<input id="i" list="l"><datalist id="l"><option value="apple" disabled><option value="apricot"></datalist>`)
	typeKeys(doc, "a")
	got := suggestions(t, doc, in)
	if len(got) != 1 || got[0] != "apricot" {
		t.Errorf("suggestions = %v, want only the enabled option", got)
	}
}

func TestDatalistUnfocusedInputNeverOpens(t *testing.T) {
	// A programmatic value change on a field the user isn't in must not make
	// a popup appear under it.
	doc, in := mustParseDatalistDoc(t, fruitDatalistHTML)
	in.Blur()
	in.SetValue("a")
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want none for an unfocused field", got)
	}
}

func TestDatalistWithoutListAttributeIsInert(t *testing.T) {
	doc, in := mustParseDatalistDoc(t,
		`<input id="i"><datalist id="l"><option value="apple"></datalist>`)
	typeKeys(doc, "a")
	if got := suggestions(t, doc, in); len(got) != 0 {
		t.Errorf("suggestions = %v, want none without a list attribute", got)
	}
	if in.Value() != "a" {
		t.Errorf("Value = %q, want typing to work normally", in.Value())
	}
}

func TestDatalistMarkersAreNotSerialized(t *testing.T) {
	doc, _ := mustParseDatalistDoc(t, fruitDatalistHTML)
	typeKeys(doc, "a")
	doc.DispatchKey("ArrowDown", document.Modifiers{})

	for _, id := range []string{"i", "l"} {
		out, err := doc.GetElementByID(id).OuterHTML()
		if err != nil {
			t.Fatalf("OuterHTML(%s): %v", id, err)
		}
		if strings.Contains(out, "data-htmlterm") {
			t.Errorf("reserved marker leaked into #%s's OuterHTML: %q", id, out)
		}
	}
}
