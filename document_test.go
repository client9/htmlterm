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
