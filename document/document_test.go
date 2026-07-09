package document_test

import (
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
	if !strings.Contains(stripANSI(docOut), "hidden") {
		t.Errorf("Document.Render must not honor StripHiddenInline (destructive on a persistent tree), got: %q", docOut)
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
	if _, ok := doc.Rect(doc.GetElementByID("d")); ok {
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
	if _, ok := doc.Rect(nil); ok {
		t.Error("Rect(nil): ok = true, want false")
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
	rect, ok := doc.Rect(doc.GetElementByID("bq"))
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}
	want := document.Rect{Row: 0, Col: 0, Width: 6, Height: 1} // "│ hi  "
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
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
	rect, ok := doc.Rect(doc.GetElementByID("in"))
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
	rect, ok := doc.Rect(doc.GetElementByID("cb"))
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
	rect, ok := doc.Rect(doc.GetElementByID("cb"))
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
	if _, ok := doc.Rect(doc.GetElementByID("cb")); ok {
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
	rect, ok := visDoc.Rect(visDoc.GetElementByID("cb2"))
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
	rect, ok := doc.Rect(doc.GetElementByID("btn"))
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
	rect, ok := doc.Rect(doc.GetElementByID("ta"))
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
	doc, err := document.ParseDocument(`<div id="d" style="border-style:normal; padding-top:2; width:100%">hi</div>`, htmlterm.Options{Width: 10})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// want: "┌────────┐\n│        │\n│        │\n│hi      │\n└────────┘\n"
	// row 0 = top border, rows 1-2 = padding-top, row 3 = content.
	rect, ok := doc.Rect(doc.GetElementByID("d"))
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
	// INTERACTIVE.md.
	doc, err := document.ParseDocument(`<p id="first">A</p><div id="target">B</div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	before, ok := doc.Rect(doc.GetElementByID("target"))
	if !ok {
		t.Fatal("Rect() ok = false, want true")
	}

	doc.GetElementByID("first").SetAttribute("style", "margin-bottom:5")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render (after mutation): %v", err)
	}
	after, ok := doc.Rect(doc.GetElementByID("target"))
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
	rect, ok := doc.Rect(pane)
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
	if !doc.Focus(btn) {
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
	doc.Focus(btn)
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
	htmlStr := `<div id="pane" style="height:2;overflow:auto;border-style:normal;padding-top:1">` +
		`line1<br>line2<br>line3<br>line4<br><button id="btn">Go</button></div>`
	doc, err := document.ParseDocument(htmlStr, htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	btn := doc.GetElementByID("btn")
	doc.Focus(btn)
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
	paneRect, ok := doc.Rect(pane)
	if !ok {
		t.Fatal("Rect(pane) ok = false, want true")
	}
	inpRect, ok := doc.Rect(inp)
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
	paneRect2, ok := doc.Rect(pane)
	if !ok {
		t.Fatal("Rect(pane) ok = false after scroll, want true")
	}
	inpRect2, ok := doc.Rect(inp)
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

// TestOverflowYScrollDrawsGutterIndicator covers SCROLLING.md's "Scrollbar
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
// a deliberate SCROLLING.md design choice (see "Why auto gets no indicator,
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
	if !strings.Contains(got, "AAAA█") || !strings.Contains(got, "BBBB│") {
		t.Fatalf("render at offset 0 = %q, want thumb on the first visible line (AAAA█) and track on the second (BBBB│)", got)
	}

	pane := doc.GetElementByID("pane")
	doc.SetScrollTop(pane, 3) // max offset (5 lines - height 2)
	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got = stripANSI(out)
	if !strings.Contains(got, "DDDD│") || !strings.Contains(got, "EEEE█") {
		t.Errorf("render at max offset = %q, want track on the first visible line (DDDD│) and thumb on the second (EEEE█)", got)
	}
}

// TestOverflowYScrollGutterDroppedWhenNoRoom covers the "silently drop the
// gutter rather than collapse content" edge case named in SCROLLING.md: a
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
