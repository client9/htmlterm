package document_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
)

func TestDocumentRenderParityWithRenderer(t *testing.T) {
	htmlStr := `<ul><li>one</li><li>two</li></ul><p>hello <b>world</b></p>`
	opts := htmlterm.Options{Width: 40}

	r, err := htmlterm.New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want, err := r.Render(htmlStr)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, err := document.ParseDocument(htmlStr, opts)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	got, err := doc.Render()
	if err != nil {
		t.Fatalf("Document.Render: %v", err)
	}
	if got != want {
		t.Errorf("Document.Render() = %q, want %q (parity with Renderer.Render)", got, want)
	}
}

func TestDocumentRenderIgnoresStripHiddenInline(t *testing.T) {
	htmlStr := `<div>a</div><div style="opacity:0">hidden</div><div>b</div>`
	opts := htmlterm.Options{Width: 40, StripHiddenInline: true}

	r, err := htmlterm.New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rendererOut, err := r.Render(htmlStr)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(rendererOut), "hidden") {
		t.Errorf("Renderer.Render with StripHiddenInline should remove the hidden div, got: %q", rendererOut)
	}

	doc, err := document.ParseDocument(htmlStr, opts)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	docOut, err := doc.Render()
	if err != nil {
		t.Fatalf("Document.Render: %v", err)
	}
	// The hidden div's text is legitimately blanked (opacity:0 paints
	// nothing) but the node itself must still occupy a line, unlike the
	// Renderer.Render/StripHiddenInline path above which removes it from
	// the tree outright. So compare line counts, not the (now correctly
	// invisible) text content.
	rendererLines := strings.Count(stripANSI(rendererOut), "\n")
	docLines := strings.Count(stripANSI(docOut), "\n")
	if docLines <= rendererLines {
		t.Errorf("Document.Render must not honor StripHiddenInline (destructive on a persistent tree): want more lines than Renderer.Render's stripped output (%d), got %d: %q", rendererLines, docLines, docOut)
	}
}

func TestGetElementByID(t *testing.T) {
	htmlStr := `<div><p id="a">first</p><p id="b">second</p></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	el := doc.GetElementByID("b")
	if el == nil {
		t.Fatal("GetElementByID(\"b\") = nil, want element")
	}
	if got := el.TextContent(); got != "second" {
		t.Errorf("TextContent() = %q, want %q", got, "second")
	}
	if got := el.TagName(); got != "p" {
		t.Errorf("TagName() = %q, want %q", got, "p")
	}
	if got := el.ID(); got != "b" {
		t.Errorf("ID() = %q, want %q", got, "b")
	}

	if doc.GetElementByID("missing") != nil {
		t.Error("GetElementByID(\"missing\") = non-nil, want nil")
	}
}

func TestQuerySelector(t *testing.T) {
	htmlStr := `<div><p class="note">one</p><p>two</p></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	el := doc.QuerySelector(".note")
	if el == nil {
		t.Fatal("QuerySelector(\".note\") = nil, want element")
	}
	if got := el.TextContent(); got != "one" {
		t.Errorf("TextContent() = %q, want %q", got, "one")
	}

	if doc.QuerySelector(".missing") != nil {
		t.Error("QuerySelector(\".missing\") = non-nil, want nil")
	}
}

func TestQuerySelectorAllDocumentOrderAndGroups(t *testing.T) {
	htmlStr := `<p>1</p><div class="foo">2</div><span>3</span><div class="foo">4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	els := doc.QuerySelectorAll("p, .foo")
	got := make([]string, 0, len(els))
	for _, e := range els {
		got = append(got, e.TextContent())
	}
	want := []string{"1", "2", "4"}
	if len(got) != len(want) {
		t.Fatalf("QuerySelectorAll(\"p, .foo\") = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("QuerySelectorAll(\"p, .foo\")[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestElementAttributeMutation(t *testing.T) {
	// abbr[title]::after is a UA-stylesheet rule (see uaCSS), so mutating
	// title is observable in rendered output without any custom CSS.
	htmlStr := `<abbr id="a">HTML</abbr>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("a")
	if el == nil {
		t.Fatal("GetElementByID(\"a\") = nil")
	}

	if _, ok := el.GetAttribute("title"); ok {
		t.Error("GetAttribute(\"title\") ok = true before SetAttribute, want false")
	}
	if el.HasAttribute("title") {
		t.Error("HasAttribute(\"title\") = true before SetAttribute, want false")
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(out), "(") {
		t.Errorf("Render() without title should not contain parenthetical, got: %q", out)
	}

	el.SetAttribute("title", "HyperText Markup Language")
	if v, ok := el.GetAttribute("title"); !ok || v != "HyperText Markup Language" {
		t.Errorf("GetAttribute(\"title\") = (%q, %v), want (%q, true)", v, ok, "HyperText Markup Language")
	}

	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "(HyperText Markup Language)") {
		t.Errorf("Render() after SetAttribute(title) = %q, want it to contain the title", out)
	}

	el.RemoveAttribute("title")
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(out), "(") {
		t.Errorf("Render() after RemoveAttribute(title) should not contain parenthetical, got: %q", out)
	}
}

func TestElementClassList(t *testing.T) {
	css := `.secret { display: none; }`
	htmlStr := `<div>before</div><span id="s">hidden text</span><div>after</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40, CSS: css})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := doc.GetElementByID("s")
	if el == nil {
		t.Fatal("GetElementByID(\"s\") = nil")
	}
	cl := el.ClassList()

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "hidden text") {
		t.Errorf("Render() before adding class should contain content, got: %q", out)
	}

	if cl.Contains("secret") {
		t.Error("Contains(\"secret\") = true before Add, want false")
	}
	cl.Add("secret")
	if !cl.Contains("secret") {
		t.Error("Contains(\"secret\") = false after Add, want true")
	}

	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(stripANSI(out), "hidden text") {
		t.Errorf("Render() after Add(\"secret\") should hide content, got: %q", out)
	}
	if !strings.Contains(stripANSI(out), "before") || !strings.Contains(stripANSI(out), "after") {
		t.Errorf("Render() after Add(\"secret\") should still contain siblings, got: %q", out)
	}

	toggled := cl.Toggle("secret")
	if toggled {
		t.Error("Toggle(\"secret\") = true after it was present, want false")
	}
	if cl.Contains("secret") {
		t.Error("Contains(\"secret\") = true after Toggle removed it, want false")
	}

	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "hidden text") {
		t.Errorf("Render() after Toggle removed \"secret\" should show content again, got: %q", out)
	}

	cl.Remove("secret") // no-op, already absent
	if cl.Contains("secret") {
		t.Error("Contains(\"secret\") = true after redundant Remove, want false")
	}
}

func TestElementValueAndChecked(t *testing.T) {
	htmlStr := `<input id="i" value="x"><input id="c" type="checkbox">`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	input := doc.GetElementByID("i")
	if got := input.Value(); got != "x" {
		t.Errorf("Value() = %q, want %q", got, "x")
	}
	input.SetValue("y")
	if got := input.Value(); got != "y" {
		t.Errorf("Value() after SetValue = %q, want %q", got, "y")
	}

	checkbox := doc.GetElementByID("c")
	if checkbox.Checked() {
		t.Error("Checked() = true initially, want false")
	}
	checkbox.SetChecked(true)
	if !checkbox.Checked() {
		t.Error("Checked() = false after SetChecked(true), want true")
	}
	if !checkbox.HasAttribute("checked") {
		t.Error("HasAttribute(\"checked\") = false after SetChecked(true), want true")
	}
	checkbox.SetChecked(false)
	if checkbox.Checked() {
		t.Error("Checked() = true after SetChecked(false), want false")
	}
	if checkbox.HasAttribute("checked") {
		t.Error("HasAttribute(\"checked\") = true after SetChecked(false), want false")
	}
}

func TestDocumentRectBeforeRenderIsNotOK(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="d">hi</div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, ok := doc.GetElementByID("d").Rect(); ok {
		t.Error("Rect() before any Render() call: ok = true, want false")
	}
}

func TestDocumentRectNilElement(t *testing.T) {
	doc, err := document.ParseDocument(`<div>hi</div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var el *document.Element
	if _, ok := el.Rect(); ok {
		t.Error("(*Element)(nil).Rect(): ok = true, want false")
	}
}

func TestDocumentRectBlockElement(t *testing.T) {
	// blockquote's UA padding-left:1/padding-right:2 and left border are
	// baked into its box, so its border-box width includes them.
	doc, err := document.ParseDocument(`<blockquote id="bq">hi</blockquote>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("bq").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	want := document.Rect{Row: 0, Col: 0, Width: 6, Height: 1} // "│ hi  "
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectFlexItems(t *testing.T) {
	// Flex items are laid out via internal/render's own row-composition pass
	// (not wordWrapTokens), so each item's own Rect has to be recorded
	// explicitly by that pass — this pins that it actually is.
	doc, err := document.ParseDocument(
		`<div style="display:flex;width:100%;gap:2"><button id="a">A</button><button id="b">B</button></div>`,
		htmlterm.Options{Width: 20},
	)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rectA, ok := doc.GetElementByID("a").Rect()
	if !ok {
		t.Fatal("Rect(a) ok = false, want true")
	}
	if want := (document.Rect{Row: 0, Col: 0, Width: 5, Height: 1}); rectA != want { // "[ A ]"
		t.Errorf("Rect(a) = %+v, want %+v (rendered: %q)", rectA, want, out)
	}
	rectB, ok := doc.GetElementByID("b").Rect()
	if !ok {
		t.Fatal("Rect(b) ok = false, want true")
	}
	if want := (document.Rect{Row: 0, Col: 7, Width: 5, Height: 1}); rectB != want { // gap:2 after "[ A ]"
		t.Errorf("Rect(b) = %+v, want %+v (rendered: %q)", rectB, want, out)
	}
}

func TestDocumentRectFlexItemsWithOrder(t *testing.T) {
	// order re-sequences items for layout purposes only — each item's Rect
	// still has to key correctly back to its own *html.Node afterward, not
	// whichever node happens to occupy that slot post-sort.
	doc, err := document.ParseDocument(
		`<div style="display:flex;width:100%"><div id="a" style="order:2">a</div><div id="b" style="order:1">b</div></div>`,
		htmlterm.Options{Width: 10},
	)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rectA, ok := doc.GetElementByID("a").Rect()
	if !ok {
		t.Fatal("Rect(a) ok = false, want true")
	}
	if want := (document.Rect{Row: 0, Col: 1, Width: 1, Height: 1}); rectA != want {
		t.Errorf("Rect(a) = %+v, want %+v (rendered: %q)", rectA, want, out)
	}
	rectB, ok := doc.GetElementByID("b").Rect()
	if !ok {
		t.Fatal("Rect(b) ok = false, want true")
	}
	if want := (document.Rect{Row: 0, Col: 0, Width: 1, Height: 1}); rectB != want {
		t.Errorf("Rect(b) = %+v, want %+v (rendered: %q)", rectB, want, out)
	}
}

func TestDocumentRectFormControlInsideLabel(t *testing.T) {
	// The realistic pattern this feature exists for: an <input> wrapped in
	// a <label>, a plain inline (non-inline-block, non-anchor) element —
	// verifies token-splicing (not string-flattening) preserves the
	// input's own trackable position through the label's boundary.
	doc, err := document.ParseDocument(`<label>Name: <input type="text" value="Bob" id="in"></label>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("in").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	want := document.Rect{Row: 0, Col: 6, Width: 5, Height: 1} // "[Bob]" after "Name: "
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectFormControlInsideListItem(t *testing.T) {
	// Regression test: renderList used to discard the position map
	// wordWrapTokens returned for each <li>'s content, so a trackable
	// element (e.g. an <input>) nested inside a list item silently never
	// got a Rect, unlike the same element wrapped in a plain <div>/<label>.
	doc, err := document.ParseDocument(`<ul><li><input type="checkbox" id="cb"></li></ul>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("cb").Rect()
	if !ok {
		t.Fatalf("Rect() ok = false, want true (rendered: %q)", out)
	}
	want := document.Rect{Row: 0, Col: 6, Width: 1, Height: 1} // "  • ☑" — checkbox after "  • " prefix
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectFormControlInsideTableCell(t *testing.T) {
	// Regression test: fillTableCellLines used to discard the position map
	// wordWrapTokens returned for each cell's content (and the noWrap path
	// flattened to a string before any wrapping box existed at all), so a
	// trackable element nested inside a <td>/<th> silently never got a
	// Rect, breaking hit-testing/click dispatch for a common pattern (a
	// form control inside a table).
	doc, err := document.ParseDocument(`<table><tr><td><input type="checkbox" id="cb"></td></tr></table>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("cb").Rect()
	if !ok {
		t.Fatalf("Rect() ok = false, want true (rendered: %q)", out)
	}
	want := document.Rect{Row: 1, Col: 1, Width: 1, Height: 1} // "│☑│" — checkbox inside the border
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}

	// The whole point of a correct Rect: DispatchClick can actually reach
	// the control.
	doc.DispatchClick(rect.Row, rect.Col)
	cb := doc.GetElementByID("cb")
	if !cb.Checked() {
		t.Error("DispatchClick at the checkbox's Rect did not toggle it")
	}
}

func TestDocumentRectClippedByOptionsHeightIsNotOK(t *testing.T) {
	// Regression test: renderTree used to remap positions for capBlankRuns'
	// removed rows but never for forceHeight's truncation, so an element on
	// a row past Options.Height (clipped out of the rendered output) still
	// reported a valid Rect — and since Loop's full-screen model always sets
	// the root height to the real terminal height on every frame, this let
	// focusCursorPos compute an out-of-bounds cursor position for any
	// document taller than the terminal whenever focus landed below the
	// fold.
	doc, err := document.ParseDocument(`<div>line1</div><div>line2</div><div>line3</div><div><input type="checkbox" id="cb"></div>`, htmlterm.Options{Width: 40, Height: 2})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if want := "line1\nline2\n"; out != want {
		t.Fatalf("Render() = %q, want %q", out, want)
	}
	if _, ok := doc.GetElementByID("cb").Rect(); ok {
		t.Error("Rect() ok = true for an element clipped by Options.Height, want false")
	}

	// An element within the retained rows must still track correctly.
	visDoc, err := document.ParseDocument(`<div><input type="checkbox" id="cb2"></div><div>line2</div>`, htmlterm.Options{Width: 40, Height: 2})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := visDoc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := visDoc.GetElementByID("cb2").Rect()
	if !ok {
		t.Fatal("Rect() ok = false for a visible element, want true")
	}
	if rect.Row != 0 {
		t.Errorf("Rect().Row = %d, want 0", rect.Row)
	}
}

func TestDocumentRectRootLevelInlineBlock(t *testing.T) {
	// A root-level, single-line inline-block element (no embedded "\n")
	// must still be tracked — regression test for a bug where render.go's
	// root dispatch only boxed inline-block content when it happened to
	// contain a newline, silently leaving single-line content as an
	// untracked plain text token.
	doc, err := document.ParseDocument(`<button id="btn">Click</button>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("btn").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	want := document.Rect{Row: 0, Col: 0, Width: 9, Height: 1} // "[ Click ]"
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectMultiLineBox(t *testing.T) {
	doc, err := document.ParseDocument("<div><textarea id=\"ta\">line one\nline two</textarea></div>", htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	rect, ok := doc.GetElementByID("ta").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	want := document.Rect{Row: 0, Col: 0, Width: 40, Height: 4} // top border + 2 lines + bottom border
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectRowShiftsForPaddingAndBorder(t *testing.T) {
	// padding-top and a top border rule both prepend rows before the
	// content's own wrapped position — verifies renderBlockContentBox's
	// row-shift calculation (pt, plus one more if a top rule is drawn).
	doc, err := document.ParseDocument(`<div id="d" style="border-style:solid; padding-top:2; width:100%">hi</div>`, htmlterm.Options{Width: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// want: "┌────────┐\n│        │\n│        │\n│hi      │\n└────────┘\n"
	// row 0 = top border, rows 1-2 = padding-top, row 3 = content.
	rect, ok := doc.GetElementByID("d").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	if rect.Row != 0 {
		t.Errorf("Rect().Row = %d, want 0 (the element's own border-box starts at the top rule; rendered: %q)", rect.Row, out)
	}
	if rect.Height != 5 {
		t.Errorf("Rect().Height = %d, want 5 (top border + 2 padding + content + bottom border; rendered: %q)", rect.Height, out)
	}
}

func TestDocumentRectUpdatesAcrossReRender(t *testing.T) {
	// Document.Render refreshes the position map on every call, so a
	// mutate-then-re-render loop always has Rects matching the latest
	// output — the core host-driven interactive use case from
	// docs/INTERACTIVE.md.
	doc, err := document.ParseDocument(`<p id="first">A</p><div id="target">B</div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	before, ok := doc.GetElementByID("target").Rect()
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}

	doc.GetElementByID("first").SetAttribute("style", "margin-bottom:5")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render (after mutation): %v", err)
	}
	after, ok := doc.GetElementByID("target").Rect()
	if !ok {
		t.Fatal("Rect() after re-render: ok = false, want true")
	}
	if after.Row <= before.Row {
		t.Errorf("Rect().Row after adding margin-bottom = %d, want > %d (before)", after.Row, before.Row)
	}
}

// TestDocumentRenderCachesRulesButStillReflectsMutations is a regression
// suite for the CSS-rule-set/selector caching Document.Render does now:
// Options.CSS/<style> rules (and each rule's parsed selector) are parsed
// once and reused across every Render call for the document's lifetime
// (rather than re-lexed and every selector re-parsed on every single call)
// — safe specifically because there is no public API to add/remove a
// <style> element, edit one's text, or change Options.CSS/IgnoreDocumentCSS
// after ParseDocument. What every one of these still must do correctly is
// re-*match* the cached, unchanged rule set against a node's *current*
// attributes on every render, since attributes (class, style=, checked,
// disabled, ...) are exactly what the public API can mutate.
func TestDocumentRenderCachesRulesButStillReflectsMutations(t *testing.T) {
	t.Run("class attribute mutation changes which cached rule matches", func(t *testing.T) {
		doc, err := document.ParseDocument(`<style>p.hot { text-transform: uppercase; }</style><p id="p">hi</p>`, htmlterm.Options{Width: 40})
		if err != nil {
			t.Fatalf("ParseDocument: %v", err)
		}
		before, err := doc.Render()
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if before != "hi\n\n" {
			t.Fatalf("Render() before mutation = %q, want %q", before, "hi\n\n")
		}
		doc.GetElementByID("p").SetAttribute("class", "hot")
		after, err := doc.Render()
		if err != nil {
			t.Fatalf("Render (after mutation): %v", err)
		}
		if after != "HI\n\n" {
			t.Fatalf("Render() after class mutation = %q, want %q", after, "HI\n\n")
		}
	})

	t.Run("inline style= mutation still applies on top of cached stylesheet rules", func(t *testing.T) {
		doc, err := document.ParseDocument(`<style>p { text-transform: uppercase; }</style><p id="p">hi</p>`, htmlterm.Options{Width: 40})
		if err != nil {
			t.Fatalf("ParseDocument: %v", err)
		}
		if _, err := doc.Render(); err != nil {
			t.Fatalf("Render: %v", err)
		}
		doc.GetElementByID("p").SetAttribute("style", "text-transform: lowercase")
		after, err := doc.Render()
		if err != nil {
			t.Fatalf("Render (after mutation): %v", err)
		}
		if after != "hi\n\n" {
			t.Fatalf("Render() after inline style mutation = %q, want %q (inline style should win over the cached stylesheet rule)", after, "hi\n\n")
		}
	})

	t.Run("checked mutation changes which cached :checked rule matches", func(t *testing.T) {
		doc, err := document.ParseDocument(`<style>input:checked { display: none; }</style><input type="checkbox" id="cb">`, htmlterm.Options{Width: 40})
		if err != nil {
			t.Fatalf("ParseDocument: %v", err)
		}
		before, err := doc.Render()
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if before == "" {
			t.Fatal("Render() before check = \"\", want a visible checkbox")
		}
		doc.GetElementByID("cb").SetChecked(true)
		after, err := doc.Render()
		if err != nil {
			t.Fatalf("Render (after mutation): %v", err)
		}
		if after != "" {
			t.Errorf("Render() after SetChecked(true) = %q, want \"\" (hidden by the cached :checked rule)", after)
		}
	})

	t.Run("Width/Height changes via SetSize are never cached stale", func(t *testing.T) {
		doc, err := document.ParseDocument(`<div>hello world this is a test</div>`, htmlterm.Options{Width: 40})
		if err != nil {
			t.Fatalf("ParseDocument: %v", err)
		}
		if _, err := doc.Render(); err != nil {
			t.Fatalf("Render: %v", err)
		}
		doc.SetSize(10, htmlterm.SizeAutomatic)
		after, err := doc.Render()
		if err != nil {
			t.Fatalf("Render (after resize): %v", err)
		}
		want := "hello\nworld this\nis a test\n"
		if after != want {
			t.Errorf("Render() after SetSize(10, ...) = %q, want %q", after, want)
		}
	})

	t.Run("repeated renders with no mutation are stable, including ::before pseudo-elements on multiple nodes sharing one cached rule", func(t *testing.T) {
		// Two elements matching the same "p::before" rule in one render pass
		// (and again across repeated Render calls) is the regression case
		// for cascade.go's pseudoElemDecls: it clears the cached rule's
		// trailing pseudoElem marker to match a real element, and must copy
		// rather than mutate the shared, cached rl.parts slice — otherwise
		// the first match corrupts it for the second element (in the same
		// pass) or the next Render call.
		doc, err := document.ParseDocument(`<style>p::before { content: "> "; }</style><p id="a">one</p><p id="b">two</p>`, htmlterm.Options{Width: 40})
		if err != nil {
			t.Fatalf("ParseDocument: %v", err)
		}
		want := "> one\n\n> two\n\n"
		for i := 0; i < 3; i++ {
			got, err := doc.Render()
			if err != nil {
				t.Fatalf("Render() call %d: %v", i, err)
			}
			if got != want {
				t.Fatalf("Render() call %d = %q, want %q", i, got, want)
			}
		}
	})
}

func TestDocumentSetSizeAndSize(t *testing.T) {
	doc, err := document.ParseDocument(`<p>hi</p>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if w, h := doc.Size(); w != 40 || h != htmlterm.SizeAutomatic {
		t.Fatalf("Size() = (%d, %d), want (40, SizeAutomatic)", w, h)
	}

	doc.SetSize(20, 3)
	if w, h := doc.Size(); w != 20 || h != 3 {
		t.Fatalf("Size() after SetSize(20, 3) = (%d, %d), want (20, 3)", w, h)
	}

	got, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stripANSI(got), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("Render() after SetSize(20, 3) has %d lines, want 3: %q", len(lines), got)
	}

	doc.SetSize(20, htmlterm.SizeNatural)
	got, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	r, err := htmlterm.New(htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want, err := r.Render(`<p>hi</p>`)
	if err != nil {
		t.Fatalf("Render (baseline): %v", err)
	}
	if stripANSI(got) != stripANSI(want) {
		t.Errorf("Render() after SetSize(20, SizeNatural) = %q, want %q (unconstrained baseline)", got, want)
	}
}

func TestDocumentElementIsStableHandle(t *testing.T) {
	// DocumentElement is the dispatch target Loop uses for the "resize"
	// event (there's no separate window-level concept in this package) —
	// verify it's a usable AddEventListener target and consistently
	// resolves to the same underlying node across calls (see
	// TestDocumentElementResizeDispatch, an internal test, for the
	// dispatch itself — "resize" is only ever fired by Loop/dispatch,
	// which is unexported).
	doc, err := document.ParseDocument(`<p>hi</p>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	doc.AddEventListener(doc.DocumentElement(), "resize", false, func(e *document.Event) {})

	// A second, separately-obtained handle must resolve to the same
	// underlying node — same "throwaway handle, stable identity" contract
	// as any other Element (see element.go).
	doc.DocumentElement().SetAttribute("data-test", "marker")
	if !doc.DocumentElement().HasAttribute("data-test") {
		t.Fatal("DocumentElement() should consistently resolve to the same node across calls")
	}
}

func TestScrollOverflowAutoSlicesContent(t *testing.T) {
	htmlStr := `<div id="pane" style="height:3;overflow:auto">line1<br>line2<br>line3<br>line4<br>line5</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line3") || strings.Contains(got, "line4") {
		t.Fatalf("initial render (offset 0) = %q, want line1-line3 visible, not line4/5", got)
	}

	pane := doc.GetElementByID("pane")
	if top, ok := doc.ScrollTop(pane); !ok || top != 0 {
		t.Errorf("ScrollTop(pane) after first render = (%d, %v), want (0, true)", top, ok)
	}

	doc.SetScrollTop(pane, 100) // beyond max; must clamp on next Render
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got = stripANSI(out)
	if !strings.Contains(got, "line3") || !strings.Contains(got, "line5") || strings.Contains(got, "line1") {
		t.Fatalf("render after over-scrolling = %q, want line3-line5 visible (clamped), not line1", got)
	}
	if top, ok := doc.ScrollTop(pane); !ok || top != 2 {
		t.Errorf("ScrollTop(pane) after clamp = (%d, %v), want (2, true) [max offset = 5-3]", top, ok)
	}
}

func TestScrollTopStaleWhenOverflowRemoved(t *testing.T) {
	htmlStr := `<div id="pane" style="height:3;overflow:auto">line1<br>line2<br>line3<br>line4<br>line5</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	doc.SetScrollTop(pane, 1)
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, ok := doc.ScrollTop(pane); !ok {
		t.Fatal("ScrollTop(pane) ok = false while overflow:auto is still set, want true")
	}

	pane.SetAttribute("style", "height:3") // drop overflow:auto
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, ok := doc.ScrollTop(pane); ok {
		t.Error("ScrollTop(pane) ok = true after overflow:auto removed, want false (stale entry pruned)")
	}
}

func TestDispatchWheelScrollsNearestScrollableAncestor(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto"><span id="inner">line1<br>line2<br>line3<br>line4</span></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	pane := doc.GetElementByID("pane")
	rect, ok := pane.Rect()
	if !ok {
		t.Fatal("Rect(pane) ok = false, want true")
	}

	if got := doc.DispatchWheel(rect.Row, rect.Col, 1); !got {
		t.Fatal("DispatchWheel over pane = false, want true")
	}
	if top, ok := doc.ScrollTop(pane); !ok || top <= 0 {
		t.Errorf("ScrollTop(pane) after wheel-down = (%d, %v), want a positive offset", top, ok)
	}

	if got := doc.DispatchWheel(9999, 9999, 1); got {
		t.Error("DispatchWheel at an unhit coordinate = true, want false")
	}
}

func TestDispatchKeyPageAndArrowScroll(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto">line1<br>line2<br>line3<br>line4<br><button id="btn">Go</button></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	btn := doc.GetElementByID("btn")
	if !btn.Focus() {
		t.Fatal("Focus(btn) = false, want true")
	}
	// Focus's own scroll-into-view already jumped the pane to make btn
	// visible; reset to the top so ArrowDown/PageDown below have room to
	// move (this test is about DispatchKey's own scrolling, not Focus's).
	pane := doc.GetElementByID("pane")
	doc.SetScrollTop(pane, 0)
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	before, _ := doc.ScrollTop(pane)
	if before != 0 {
		t.Fatalf("ScrollTop(pane) after reset = %d, want 0", before)
	}

	doc.DispatchKey("ArrowDown")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	afterArrow, _ := doc.ScrollTop(pane)
	if afterArrow <= before {
		t.Errorf("ScrollTop(pane) after ArrowDown = %d, want > %d", afterArrow, before)
	}

	doc.DispatchKey("PageDown")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	afterPage, _ := doc.ScrollTop(pane)
	if afterPage <= afterArrow {
		t.Errorf("ScrollTop(pane) after PageDown = %d, want > %d", afterPage, afterArrow)
	}

	doc.DispatchKey("PageUp")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	afterPageUp, _ := doc.ScrollTop(pane)
	if afterPageUp >= afterPage {
		t.Errorf("ScrollTop(pane) after PageUp = %d, want < %d", afterPageUp, afterPage)
	}
}

func TestFocusScrollsIntoView(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto">line1<br>line2<br>line3<br>line4<br><button id="btn">Go</button></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	pane := doc.GetElementByID("pane")
	if top, ok := doc.ScrollTop(pane); !ok || top != 0 {
		t.Fatalf("ScrollTop(pane) before focus = (%d, %v), want (0, true)", top, ok)
	}

	btn := doc.GetElementByID("btn")
	btn.Focus()
	if top, ok := doc.ScrollTop(pane); !ok || top == 0 {
		t.Errorf("ScrollTop(pane) after focusing an off-screen button = (%d, %v), want a positive offset", top, ok)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "Go") {
		t.Errorf("render after focusing btn = %q, want btn scrolled into view", out)
	}
}

// TestFocusScrollsIntoViewWithBorderAndPadding pins a real bug caught by
// driving cmd/htmlterm-tui in a live pty (tmux): scrollIntoView must use the
// container's resolved content-box viewport (Document.scrollViewport), not
// its Rect (the CSS border box, which also includes border/padding rows) —
// using Rect directly under-scrolled by exactly the border+padding row
// count whenever a scroll container had either set.
func TestFocusScrollsIntoViewWithBorderAndPadding(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto;border-style:solid;padding-top:1">` +
		`line1<br>line2<br>line3<br>line4<br><button id="btn">Go</button></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	btn := doc.GetElementByID("btn")
	btn.Focus()
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(stripANSI(out), "Go") {
		t.Errorf("render after focusing btn (bordered/padded pane) = %q, want btn scrolled into view", out)
	}
}

// TestScrollVisibleTracksScrollPosition pins the fix for a real bug caught
// interactively (driving cmd/htmlterm-tui in a live pty): scrolling a pane
// via DispatchKey/DispatchWheel while focus stays on a control inside it
// left Loop's terminal cursor parked at that control's stale, now
// off-screen Rect — visibly drifting away from (and past the edge of) the
// pane instead of tracking what's actually visible. ScrollVisible is what
// focusCursorPos (loop.go) now checks before placing the cursor there.
func TestScrollVisibleTracksScrollPosition(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto"><button id="btn">Go</button><br>line2<br>line3<br>line4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	pane := doc.GetElementByID("pane")
	btn := doc.GetElementByID("btn")
	if !doc.ScrollVisible(btn) {
		t.Fatal("ScrollVisible(btn) at offset 0 = false, want true (btn is the pane's first line)")
	}

	doc.SetScrollTop(pane, 2) // max offset (4 lines - height 2); scrolls btn off the top
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if doc.ScrollVisible(btn) {
		t.Error("ScrollVisible(btn) after scrolling it off the top = true, want false")
	}

	doc.SetScrollTop(pane, 0)
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !doc.ScrollVisible(btn) {
		t.Error("ScrollVisible(btn) after scrolling back to offset 0 = false, want true")
	}
}

func TestScrollShiftsDescendantPositions(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto">line1<br><input id="inp" value="x"><br>line3<br>line4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	inp := doc.GetElementByID("inp")

	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	paneRect, ok := pane.Rect()
	if !ok {
		t.Fatal("Rect(pane) ok = false, want true")
	}
	inpRect, ok := inp.Rect()
	if !ok {
		t.Fatal("Rect(inp) ok = false, want true")
	}
	if got := inpRect.Row - paneRect.Row; got != 1 {
		t.Fatalf("input's row relative to pane at offset 0 = %d, want 1", got)
	}

	doc.SetScrollTop(pane, 2)
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	paneRect2, ok := pane.Rect()
	if !ok {
		t.Fatal("Rect(pane) ok = false after scroll, want true")
	}
	inpRect2, ok := inp.Rect()
	if !ok {
		t.Fatal("Rect(inp) ok = false after scroll, want true (kept, not deleted, even though scrolled out of view)")
	}
	if got := inpRect2.Row - paneRect2.Row; got != -1 {
		t.Errorf("input's row relative to pane at offset 2 = %d, want -1 (scrolled above the visible range)", got)
	}
}

func TestNestedScrollableRegions(t *testing.T) {
	htmlStr := `<div id="outer" style="height:2;overflow:auto">` +
		`o1<br>` +
		`<div id="inner" style="height:2;overflow:auto">i1<br>i2<br>i3<br>i4</div><br>` +
		`o3<br>o4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	outer := doc.GetElementByID("outer")
	inner := doc.GetElementByID("inner")
	if _, ok := doc.ScrollTop(outer); !ok {
		t.Fatal("ScrollTop(outer) ok = false, want true")
	}
	if _, ok := doc.ScrollTop(inner); !ok {
		t.Fatal("ScrollTop(inner) ok = false, want true (nested scroll container tracked independently)")
	}

	doc.SetScrollTop(inner, 2)
	doc.SetScrollTop(outer, 1)
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render after scrolling both nested containers: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "i3") || strings.Contains(got, "i1") {
		t.Errorf("render with inner scrolled to offset 2 = %q, want i3 visible, not i1", got)
	}
}

// TestScrollContainerWithoutFocusableChildIsFocusable mirrors real browsers'
// keyboard-accessible scroll containers: a scrollable region with no
// button/input inside it would otherwise be unreachable via Tab, leaving
// keyboard users no way to scroll it (mouse wheel would still work, but
// that's not a keyboard-accessible story).
func TestScrollContainerWithoutFocusableChildIsFocusable(t *testing.T) {
	htmlStr := `<p>before</p><div id="pane" style="height:2;overflow:auto">line1<br>line2<br>line3<br>line4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	next := doc.FocusNext()
	if next == nil || next.ID() != "pane" {
		id := ""
		if next != nil {
			id = next.ID()
		}
		t.Fatalf("FocusNext() landed on id=%q, want %q (the childless scroll container itself)", id, "pane")
	}

	pane := doc.GetElementByID("pane")
	before, ok := doc.ScrollTop(pane)
	if !ok {
		t.Fatal("ScrollTop(pane) ok = false, want true")
	}
	doc.DispatchKey("ArrowDown")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	after, _ := doc.ScrollTop(pane)
	if after <= before {
		t.Errorf("ScrollTop(pane) after ArrowDown while pane itself is focused = %d, want > %d", after, before)
	}
}

// TestScrollContainerWithFocusableChildIsNotDoubleTabStop ensures a scroll
// container that already has a reachable focusable descendant (e.g. the
// outer log pane in cmd/htmlterm-tui, which has a button) isn't *also* made
// its own separate tab stop — Tab should reach it via that descendant only.
func TestScrollContainerWithFocusableChildIsNotDoubleTabStop(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto">line1<br><button id="btn">Go</button></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	first := doc.FocusNext()
	if first == nil || first.ID() != "btn" {
		id := ""
		if first != nil {
			id = first.ID()
		}
		t.Fatalf("FocusNext() landed on id=%q, want %q", id, "btn")
	}
	second := doc.FocusNext()
	if second == nil || second.ID() != "btn" {
		id := ""
		if second != nil {
			id = second.ID()
		}
		t.Fatalf("second FocusNext() (wrapping around, only 1 focusable element) landed on id=%q, want %q again", id, "btn")
	}
}

// TestOverflowYScrollDrawsGutterIndicator covers docs/SCROLLING.md's "Scrollbar
// gutter and indicator": overflow-y:scroll reserves a column and draws a
// track/thumb, unconditionally (regardless of whether content overflows).
func TestOverflowYScrollDrawsGutterIndicator(t *testing.T) {
	htmlStr := `<div id="pane" style="height:3;overflow-y:scroll">line1<br>line2<br>line3<br>line4<br>line5</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "█") {
		t.Errorf("render with overflow-y:scroll = %q, want a thumb character (█)", got)
	}
	if !strings.Contains(got, "│") {
		t.Errorf("render with overflow-y:scroll = %q, want at least one track character (│)", got)
	}
}

// TestOverflowYAutoDrawsNoGutter locks in that overflow-y:auto (and plain
// overflow:auto, its shorthand equivalent) get no gutter/indicator at all —
// a deliberate docs/SCROLLING.md design choice (see "Why auto gets no indicator,
// deliberately"), and also a regression guard that this feature didn't
// change auto's existing, already-shipped rendering.
func TestOverflowYAutoDrawsNoGutter(t *testing.T) {
	for _, style := range []string{
		"height:3;overflow-y:auto",
		"height:3;overflow:auto",
	} {
		htmlStr := `<div id="pane" style="` + style + `">line1<br>line2<br>line3<br>line4<br>line5</div>`
		doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
		if err != nil {
			t.Fatalf("ParseDocument (%s): %v", style, err)
		}
		out, err := doc.Render()
		if err != nil {
			t.Fatalf("Render (%s): %v", style, err)
		}
		got := stripANSI(out)
		if strings.ContainsAny(got, "█│") {
			t.Errorf("render with style=%q = %q, want no gutter/indicator characters", style, got)
		}
	}
}

// TestOverflowYOverridesShorthandPerAxis confirms overflow-y (longhand)
// overrides just the y-axis of an already-set overflow (shorthand) —
// exercising expandShorthand's overflow case plus the normal per-property
// cascade merge (cascade.go's directDecls), the mechanism this feature
// reuses instead of a bespoke runtime fallback.
func TestOverflowYOverridesShorthandPerAxis(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow:auto;overflow-y:scroll">line1<br>line2<br>line3<br>line4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "█") {
		t.Errorf("render with overflow:auto + overflow-y:scroll = %q, want overflow-y to win (a thumb character)", got)
	}
}

// TestOverflowYOverridesShorthandPerAxisViaStylesheet is the same check as
// TestOverflowYOverridesShorthandPerAxis but via two same-specificity
// <style> rules rather than a single inline style attribute, exercising
// directDecls' source-order tie-break (cascade.go) rather than
// parseInlineDecls' sequential-commit merge.
func TestOverflowYOverridesShorthandPerAxisViaStylesheet(t *testing.T) {
	htmlStr := `<style>#pane { overflow: auto; } #pane { overflow-y: scroll; }</style>` +
		`<div id="pane" style="height:2">line1<br>line2<br>line3<br>line4</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "█") {
		t.Errorf("render with later #pane{overflow-y:scroll} rule = %q, want it to win (a thumb character)", got)
	}
}

// TestScrollbarThumbTracksScrollPosition verifies the thumb glyph moves to
// track SetScrollTop, not just that it's drawn once statically.
func TestScrollbarThumbTracksScrollPosition(t *testing.T) {
	htmlStr := `<div id="pane" style="height:2;overflow-y:scroll">AAAA<br>BBBB<br>CCCC<br>DDDD<br>EEEE</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "AAAA") || !strings.Contains(got, "BBBB") {
		t.Fatalf("render at offset 0 = %q, want AAAA and BBBB present", got)
	}
	if !regexp.MustCompile(`AAAA +█`).MatchString(got) || !regexp.MustCompile(`BBBB +│`).MatchString(got) {
		t.Fatalf("render at offset 0 = %q, want thumb on the first visible line (AAAA...█) and track on the second (BBBB...│), each padded to a straight gutter column", got)
	}

	pane := doc.GetElementByID("pane")
	doc.SetScrollTop(pane, 3) // max offset (5 lines - height 2)
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got = stripANSI(out)
	if !regexp.MustCompile(`DDDD +│`).MatchString(got) || !regexp.MustCompile(`EEEE +█`).MatchString(got) {
		t.Errorf("render at max offset = %q, want track on the first visible line (DDDD...│) and thumb on the second (EEEE...█), each padded to a straight gutter column", got)
	}
}

// TestOverflowYScrollGutterDroppedWhenNoRoom covers the "silently drop the
// gutter rather than collapse content" edge case named in docs/SCROLLING.md: a
// box too narrow to spare a column for the gutter renders its content with
// no gutter/indicator at all, rather than corrupting it.
func TestOverflowYScrollGutterDroppedWhenNoRoom(t *testing.T) {
	htmlStr := `<div id="pane" style="width:1;height:2;overflow-y:scroll">a<br>b<br>c</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if strings.ContainsAny(got, "█│") {
		t.Errorf("render with width:1 (no room for a gutter) = %q, want no gutter/indicator characters", got)
	}
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("render with width:1 = %q, want real content (a, b) preserved, not corrupted", got)
	}
}

func TestSetInnerHTMLReplacesChildren(t *testing.T) {
	htmlStr := `<div id="list"><p>stale</p></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	list := doc.GetElementByID("list")
	if list == nil {
		t.Fatal("GetElementByID(\"list\") = nil")
	}

	if err := doc.SetInnerHTML(list, `<p>fresh</p><p>content</p>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if strings.Contains(got, "stale") {
		t.Errorf("Render() = %q, still contains replaced content %q", got, "stale")
	}
	if !strings.Contains(got, "fresh") || !strings.Contains(got, "content") {
		t.Errorf("Render() = %q, want new content (fresh, content)", got)
	}
}

func TestSetInnerHTMLTableFragmentInTableContext(t *testing.T) {
	htmlStr := `<table id="tbl"></table>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	tbl := doc.GetElementByID("tbl")
	if err := doc.SetInnerHTML(tbl, `<tr><td>a</td><td>b</td></tr>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("Render() = %q, want table cells a, b", got)
	}
}

func TestSetInnerHTMLClearsFocusOnRemovedDescendant(t *testing.T) {
	htmlStr := `<div id="pane"><input id="name" type="text"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	input := doc.GetElementByID("name")
	if !input.Focus() {
		t.Fatal("Focus(input) = false, want true")
	}
	if doc.FocusedElement() == nil {
		t.Fatal("FocusedElement() = nil after Focus, want input")
	}

	if err := doc.SetInnerHTML(pane, `<p>replaced</p>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}

	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() != nil after focused element's container was replaced, want nil")
	}
}

func TestSetInnerHTMLPreservesFocusOutsideReplacedSubtree(t *testing.T) {
	htmlStr := `<div id="pane"><p>old</p></div><input id="name" type="text">`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	input := doc.GetElementByID("name")
	input.Focus()

	if err := doc.SetInnerHTML(pane, `<p>new</p>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}

	got := doc.FocusedElement()
	if got == nil || got.ID() != "name" {
		t.Errorf("FocusedElement() after unrelated SetInnerHTML = %v, want \"name\" to remain focused", got)
	}
}

func TestSetInnerHTMLNilElement(t *testing.T) {
	doc, err := document.ParseDocument(`<div></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if err := doc.SetInnerHTML(nil, `<p>x</p>`); err == nil {
		t.Error("SetInnerHTML(nil, ...) = nil error, want error")
	}
}

// TestSetInnerHTMLSanitizesEmbeddedANSI is the security-boundary regression
// guard for SetPreRendered's whole premise: ordinary SetInnerHTML content
// (however it's spelled, including literal escape bytes an attacker-
// controlled email might contain) must still be sanitized. Only a RawNode —
// which SetInnerHTML/html.Parse can never produce — bypasses that.
func TestSetInnerHTMLSanitizesEmbeddedANSI(t *testing.T) {
	htmlStr := `<div id="pane"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	if err := doc.SetInnerHTML(pane, "\x1b[1mBOLD\x1b[m plain"); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "\x1b[1m") {
		t.Errorf("Render() = %q, want embedded ESC sequence stripped by SetInnerHTML's sanitizer", out)
	}
	if got := stripANSI(out); !strings.Contains(got, "BOLD plain") {
		t.Errorf("Render() visible text = %q, want the literal text content preserved", got)
	}
}

// TestSetPreRenderedBypassesSanitization is SetPreRendered's core contract:
// content it inserts is exempt from the sanitization
// TestSetInnerHTMLSanitizesEmbeddedANSI just confirmed applies to ordinary
// content, because it's carried by an html.RawNode (never producible by
// html.Parse/ParseFragment) rather than a TextNode.
func TestSetPreRenderedBypassesSanitization(t *testing.T) {
	htmlStr := `<div id="pane"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	doc.SetPreRendered(pane, "\x1b[1mBOLD\x1b[m plain")

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "\x1b[1mBOLD\x1b[m") {
		t.Errorf("Render() = %q, want embedded SGR bold sequence preserved verbatim", out)
	}
}

// TestSetPreRenderedSurvivesScrollClip is the actual bug this API was added
// to fix: a mail-repl message pane pre-rendered once and re-embedded via
// SetPreRendered lost all styling once scrolled, because scroll clipping
// doesn't re-run sanitization — the styling was gone from the very first,
// unscrolled render already (SetInnerHTML's sanitizer stripped it before
// scrolling ever entered the picture). This exercises both an off-screen
// (scrolled-away) and on-screen line to confirm the fix holds for content
// that must survive overflow-y's lines[offset:offset+height] slicing.
func TestSetPreRenderedSurvivesScrollClip(t *testing.T) {
	pre := "\x1b[1mBOLD line1\x1b[m\nplain line2\nplain line3\nplain line4\n\x1b[4mUNDER line5\x1b[m"
	htmlStr := `<div id="pane" style="height:3;overflow:auto"><pre id="content"></pre></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	content := doc.GetElementByID("content")
	doc.SetPreRendered(content, pre)

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "\x1b[1mBOLD line1\x1b[m") {
		t.Errorf("initial render = %q, want line1's bold SGR sequence intact", out)
	}

	pane := doc.GetElementByID("pane")
	doc.SetScrollTop(pane, 2) // line5 (index 4) now the last of the 3 visible lines
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if !strings.Contains(got, "UNDER line5") {
		t.Fatalf("scrolled render = %q, want line5 visible", got)
	}
	if !strings.Contains(out, "\x1b[4mUNDER line5\x1b[m") {
		t.Errorf("scrolled render = %q, want line5's underline SGR sequence intact after scroll clip", out)
	}
}

// TestSetPreRenderedReplacesExistingChildren mirrors
// TestSetInnerHTMLReplacesChildren: a second call fully replaces the first,
// no leftover content from the prior call remains.
func TestSetPreRenderedReplacesExistingChildren(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="pane"><p>stale</p></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	doc.SetPreRendered(pane, "first")
	doc.SetPreRendered(pane, "second")

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := stripANSI(out)
	if strings.Contains(got, "stale") || strings.Contains(got, "first") {
		t.Errorf("Render() = %q, want no trace of earlier content", got)
	}
	if !strings.Contains(got, "second") {
		t.Errorf("Render() = %q, want %q", got, "second")
	}
}

// TestSetPreRenderedClearsFocusOnRemovedDescendant mirrors
// TestSetInnerHTMLClearsFocusOnRemovedDescendant: replacing a subtree that
// contains the focused element must clear focus rather than leave it
// dangling on a detached node.
func TestSetPreRenderedClearsFocusOnRemovedDescendant(t *testing.T) {
	htmlStr := `<div id="pane"><input id="name" type="text"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	pane := doc.GetElementByID("pane")
	input := doc.GetElementByID("name")
	if !input.Focus() {
		t.Fatal("Focus(input) = false, want true")
	}

	doc.SetPreRendered(pane, "replaced")

	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() != nil after focused element's container was replaced, want nil")
	}
}

func TestSetPreRenderedNilElement(t *testing.T) {
	doc, err := document.ParseDocument(`<div></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	// Must not panic.
	doc.SetPreRendered(nil, "x")
}

func TestElementTreeNavigation(t *testing.T) {
	htmlStr := `<div id="parent">` +
		`<span id="one">1</span>` +
		`<span id="two">2</span>` +
		`<span id="three">3</span>` +
		`</div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	parent := doc.GetElementByID("parent")
	one := doc.GetElementByID("one")
	two := doc.GetElementByID("two")
	three := doc.GetElementByID("three")

	if got := parent.FirstElementChild(); got == nil || got.ID() != "one" {
		t.Errorf("FirstElementChild() = %v, want id=one", got)
	}
	if got := parent.LastElementChild(); got == nil || got.ID() != "three" {
		t.Errorf("LastElementChild() = %v, want id=three", got)
	}
	if got := one.NextElementSibling(); got == nil || got.ID() != "two" {
		t.Errorf("one.NextElementSibling() = %v, want id=two", got)
	}
	if got := three.PreviousElementSibling(); got == nil || got.ID() != "two" {
		t.Errorf("three.PreviousElementSibling() = %v, want id=two", got)
	}
	if one.PreviousElementSibling() != nil {
		t.Error("one.PreviousElementSibling() != nil, want nil (first child)")
	}
	if three.NextElementSibling() != nil {
		t.Error("three.NextElementSibling() != nil, want nil (last child)")
	}
	if got := two.Parent(); got == nil || got.ID() != "parent" {
		t.Errorf("two.Parent() = %v, want id=parent", got)
	}
	if doc.DocumentElement().Parent() != nil {
		t.Error("DocumentElement().Parent() != nil, want nil (document root)")
	}

	children := parent.Children()
	if len(children) != 3 || children[0].ID() != "one" || children[1].ID() != "two" || children[2].ID() != "three" {
		t.Errorf("Children() = %v, want [one two three]", children)
	}

	// This markup has no whitespace between tags, so plain (non-Element-
	// filtered) sibling/child navigation lines up with the element-only
	// view too.
	if got := two.FirstChild(); got == nil || got.TextContent() != "2" {
		t.Errorf("two.FirstChild().TextContent() = %v, want \"2\"", got)
	}
	if got := two.NextSibling(); got == nil || got.ID() != "three" {
		t.Errorf("two.NextSibling() = %v, want id=three", got)
	}
	if got := two.PreviousSibling(); got == nil || got.ID() != "one" {
		t.Errorf("two.PreviousSibling() = %v, want id=one", got)
	}
}

func TestElementCreateAndAppendChild(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="container"></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")

	span := doc.CreateElement("span")
	span.SetAttribute("id", "created")
	span.AppendChild(doc.CreateTextNode("hello"))
	container.AppendChild(span)

	got := doc.GetElementByID("created")
	if got == nil {
		t.Fatal("GetElementByID(\"created\") = nil after AppendChild")
	}
	if got.TextContent() != "hello" {
		t.Errorf("TextContent() = %q, want %q", got.TextContent(), "hello")
	}
	if p := got.Parent(); p == nil || p.ID() != "container" {
		t.Error("created span's Parent() should be the container")
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("Render() = %q, want it to contain the appended content %q", out, "hello")
	}
}

func TestElementInsertBefore(t *testing.T) {
	htmlStr := `<div id="container"><span id="last">last</span></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")
	last := doc.GetElementByID("last")

	first := doc.CreateElement("span")
	first.SetAttribute("id", "first")
	container.InsertBefore(first, last)

	if got := container.FirstElementChild(); got == nil || got.ID() != "first" {
		t.Errorf("FirstElementChild() = %v, want id=first (inserted before last)", got)
	}

	end := doc.CreateElement("span")
	end.SetAttribute("id", "end")
	container.InsertBefore(end, nil) // nil oldChild appends at the end
	if got := container.LastElementChild(); got == nil || got.ID() != "end" {
		t.Errorf("LastElementChild() = %v, want id=end (nil oldChild appends)", got)
	}
}

func TestElementRemoveChild(t *testing.T) {
	htmlStr := `<div id="container"><span id="a">a</span><span id="b">b</span></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")
	a := doc.GetElementByID("a")

	removed := container.RemoveChild(a)
	if removed.ID() != "a" {
		t.Errorf("RemoveChild returned id=%q, want a", removed.ID())
	}
	if removed.Parent() != nil {
		t.Error("removed child's Parent() != nil, want nil (detached)")
	}
	if got := container.Children(); len(got) != 1 || got[0].ID() != "b" {
		t.Errorf("container.Children() after removal = %v, want [b]", got)
	}
	if doc.GetElementByID("a") != nil {
		t.Error("GetElementByID(\"a\") still finds the removed element")
	}

	// A detached node can be re-attached elsewhere.
	container.AppendChild(removed)
	if got := container.Children(); len(got) != 2 || got[1].ID() != "a" {
		t.Errorf("container.Children() after re-appending removed child = %v, want [b a]", got)
	}
}

func TestElementRemoveChildPanicsOnNonChild(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="a"></div><div id="b"></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	a := doc.GetElementByID("a")
	b := doc.GetElementByID("b")

	defer func() {
		if recover() == nil {
			t.Error("RemoveChild(non-child) did not panic")
		}
	}()
	a.RemoveChild(b)
}

func TestElementRemoveChildClearsFocusOnDescendant(t *testing.T) {
	htmlStr := `<div id="container"><input id="name" type="text"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")
	input := doc.GetElementByID("name")
	if !input.Focus() {
		t.Fatal("Focus(input) = false, want true")
	}

	container.RemoveChild(input)

	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() != nil after focused element was removed, want nil")
	}
}

func TestElementReplaceChild(t *testing.T) {
	htmlStr := `<div id="container"><span id="old">old</span></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")
	oldChild := doc.GetElementByID("old")

	newChild := doc.CreateElement("span")
	newChild.SetAttribute("id", "new")

	returned := container.ReplaceChild(newChild, oldChild)
	if returned.ID() != "old" {
		t.Errorf("ReplaceChild returned id=%q, want old", returned.ID())
	}
	if returned.Parent() != nil {
		t.Error("replaced-out child's Parent() != nil, want nil (detached)")
	}
	if got := container.Children(); len(got) != 1 || got[0].ID() != "new" {
		t.Errorf("container.Children() after ReplaceChild = %v, want [new]", got)
	}
	if doc.GetElementByID("old") != nil {
		t.Error("GetElementByID(\"old\") still finds the replaced-out element")
	}
}

func TestElementReplaceChildClearsFocusOnOldChild(t *testing.T) {
	htmlStr := `<div id="container"><input id="name" type="text"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	container := doc.GetElementByID("container")
	input := doc.GetElementByID("name")
	if !input.Focus() {
		t.Fatal("Focus(input) = false, want true")
	}

	container.ReplaceChild(doc.CreateElement("span"), input)

	if doc.FocusedElement() != nil {
		t.Error("FocusedElement() != nil after focused element was replaced out, want nil")
	}
}

func TestElementReplaceChildPanicsOnNonChild(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="a"></div><div id="b"></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	a := doc.GetElementByID("a")
	b := doc.GetElementByID("b")

	defer func() {
		if recover() == nil {
			t.Error("ReplaceChild(_, non-child) did not panic")
		}
	}()
	a.ReplaceChild(doc.CreateElement("span"), b)
}

func TestElementCloneNodeDeep(t *testing.T) {
	htmlStr := `<div id="orig" class="row"><span>hello</span></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	orig := doc.GetElementByID("orig")

	clone := orig.CloneNode(true)
	if clone.Parent() != nil {
		t.Error("CloneNode result has a Parent(), want detached (nil)")
	}
	if clone.ID() != "orig" || !clone.ClassList().Contains("row") {
		t.Errorf("clone attributes = id:%q class-has-row:%v, want id:orig class-has-row:true", clone.ID(), clone.ClassList().Contains("row"))
	}
	if clone.TextContent() != "hello" {
		t.Errorf("deep clone TextContent() = %q, want %q (children copied)", clone.TextContent(), "hello")
	}

	// Mutating the clone must not affect the original.
	clone.SetAttribute("id", "clone")
	if orig.ID() != "orig" {
		t.Errorf("orig.ID() = %q after mutating clone, want unchanged \"orig\"", orig.ID())
	}

	container := doc.GetElementByID("orig").Parent()
	container.AppendChild(clone)
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Count(out, "hello") != 2 {
		t.Errorf("Render() = %q, want \"hello\" to appear twice (original + attached deep clone)", out)
	}
}

func TestElementCloneNodeShallowExcludesChildren(t *testing.T) {
	doc, err := document.ParseDocument(`<div id="orig"><span>hello</span></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	orig := doc.GetElementByID("orig")

	clone := orig.CloneNode(false)
	if clone.TextContent() != "" {
		t.Errorf("shallow clone TextContent() = %q, want \"\" (no children copied)", clone.TextContent())
	}
	if clone.ID() != "orig" {
		t.Errorf("shallow clone ID() = %q, want \"orig\" (attributes still copied)", clone.ID())
	}
}

func TestElementCloneNodeDoesNotCopyFocusState(t *testing.T) {
	htmlStr := `<div id="container"><input id="a"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	a := doc.GetElementByID("a")
	if !a.Focus() {
		t.Fatal("Focus(a) = false, want true")
	}

	clone := a.CloneNode(false)
	clone.SetAttribute("id", "clone")
	doc.GetElementByID("container").AppendChild(clone)

	focused := doc.QuerySelectorAll(":focus")
	if len(focused) != 1 || focused[0].ID() != "a" {
		t.Errorf("QuerySelectorAll(:focus) after cloning the focused element = %v, want exactly [a] (clone must not carry focus state)", focused)
	}
}

func TestElementMatches(t *testing.T) {
	htmlStr := `<ul><li id="a" class="row warn">a</li><li id="b">b</li></ul>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	a := doc.GetElementByID("a")
	b := doc.GetElementByID("b")

	if !a.Matches(".row") {
		t.Error("a.Matches(\".row\") = false, want true")
	}
	if !a.Matches("li.warn") {
		t.Error("a.Matches(\"li.warn\") = false, want true")
	}
	if a.Matches(".missing") {
		t.Error("a.Matches(\".missing\") = true, want false")
	}
	if b.Matches(".row") {
		t.Error("b.Matches(\".row\") = true, want false")
	}
	// Comma-separated selector groups: matches if any branch matches.
	if !b.Matches(".row, #b") {
		t.Error("b.Matches(\".row, #b\") = false, want true (second branch matches)")
	}
}

func TestElementClosest(t *testing.T) {
	htmlStr := `<div id="outer" class="row"><div id="inner"><span id="leaf">x</span></div></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	leaf := doc.GetElementByID("leaf")
	inner := doc.GetElementByID("inner")

	if got := leaf.Closest(".row"); got == nil || got.ID() != "outer" {
		t.Errorf("leaf.Closest(\".row\") = %v, want id=outer", got)
	}
	// Closest checks the element itself first, before walking up.
	if got := inner.Closest("#inner"); got == nil || got.ID() != "inner" {
		t.Errorf("inner.Closest(\"#inner\") = %v, want id=inner (matches itself)", got)
	}
	if got := leaf.Closest(".missing"); got != nil {
		t.Errorf("leaf.Closest(\".missing\") = %v, want nil", got)
	}
	if got := doc.DocumentElement().Closest("*"); got != nil {
		t.Errorf("DocumentElement().Closest(\"*\") = %v, want nil (document root is not an element)", got)
	}
}

func TestElementContains(t *testing.T) {
	htmlStr := `<div id="outer"><div id="inner"><span id="leaf">x</span></div></div><div id="other"></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	outer := doc.GetElementByID("outer")
	leaf := doc.GetElementByID("leaf")
	other := doc.GetElementByID("other")

	if !outer.Contains(leaf) {
		t.Error("outer.Contains(leaf) = false, want true")
	}
	if !outer.Contains(outer) {
		t.Error("outer.Contains(outer) = false, want true (inclusive of itself)")
	}
	if outer.Contains(other) {
		t.Error("outer.Contains(other) = true, want false")
	}
	if outer.Contains(nil) {
		t.Error("outer.Contains(nil) = true, want false")
	}
}
