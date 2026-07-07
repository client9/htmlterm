package htmlterm_test

import (
	"strings"
	"testing"

	"github.com/client9/htmlterm"
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

	doc, err := htmlterm.ParseDocument(htmlStr, opts)
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

	doc, err := htmlterm.ParseDocument(htmlStr, opts)
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40, CSS: css})
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
	doc, err := htmlterm.ParseDocument(htmlStr, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(`<div id="d">hi</div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, ok := doc.Rect(doc.GetElementByID("d")); ok {
		t.Error("Rect() before any Render() call: ok = true, want false")
	}
}

func TestDocumentRectNilElement(t *testing.T) {
	doc, err := htmlterm.ParseDocument(`<div>hi</div>`, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(`<blockquote id="bq">hi</blockquote>`, htmlterm.Options{Width: 40})
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
	want := htmlterm.Rect{Row: 0, Col: 0, Width: 6, Height: 1} // "│ hi  "
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectFormControlInsideLabel(t *testing.T) {
	// The realistic pattern this feature exists for: an <input> wrapped in
	// a <label>, a plain inline (non-inline-block, non-anchor) element —
	// verifies token-splicing (not string-flattening) preserves the
	// input's own trackable position through the label's boundary.
	doc, err := htmlterm.ParseDocument(`<label>Name: <input type="text" value="Bob" id="in"></label>`, htmlterm.Options{Width: 40})
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
	want := htmlterm.Rect{Row: 0, Col: 6, Width: 5, Height: 1} // "[Bob]" after "Name: "
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectRootLevelInlineBlock(t *testing.T) {
	// A root-level, single-line inline-block element (no embedded "\n")
	// must still be tracked — regression test for a bug where render.go's
	// root dispatch only boxed inline-block content when it happened to
	// contain a newline, silently leaving single-line content as an
	// untracked plain text token.
	doc, err := htmlterm.ParseDocument(`<button id="btn">Click</button>`, htmlterm.Options{Width: 40})
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
	want := htmlterm.Rect{Row: 0, Col: 0, Width: 9, Height: 1} // "[ Click ]"
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectMultiLineBox(t *testing.T) {
	doc, err := htmlterm.ParseDocument("<div><textarea id=\"ta\">line one\nline two</textarea></div>", htmlterm.Options{Width: 40})
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
	want := htmlterm.Rect{Row: 0, Col: 0, Width: 40, Height: 4} // top border + 2 lines + bottom border
	if rect != want {
		t.Errorf("Rect() = %+v, want %+v (rendered: %q)", rect, want, out)
	}
}

func TestDocumentRectRowShiftsForPaddingAndBorder(t *testing.T) {
	// padding-top and a top border rule both prepend rows before the
	// content's own wrapped position — verifies renderBlockContentBox's
	// row-shift calculation (pt, plus one more if a top rule is drawn).
	doc, err := htmlterm.ParseDocument(`<div id="d" style="border-style:normal; padding-top:2; width:100%">hi</div>`, htmlterm.Options{Width: 10})
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
	doc, err := htmlterm.ParseDocument(`<p id="first">A</p><div id="target">B</div>`, htmlterm.Options{Width: 40})
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

func TestDocumentSetSizeAndSize(t *testing.T) {
	doc, err := htmlterm.ParseDocument(`<p>hi</p>`, htmlterm.Options{Width: 40})
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
	doc, err := htmlterm.ParseDocument(`<p>hi</p>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	doc.AddEventListener(doc.DocumentElement(), "resize", false, func(e *htmlterm.Event) {})

	// A second, separately-obtained handle must resolve to the same
	// underlying node — same "throwaway handle, stable identity" contract
	// as any other Element (see element.go).
	doc.DocumentElement().SetAttribute("data-test", "marker")
	if !doc.DocumentElement().HasAttribute("data-test") {
		t.Fatal("DocumentElement() should consistently resolve to the same node across calls")
	}
}
