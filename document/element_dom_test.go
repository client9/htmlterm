package document_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/client9/htmlterm/document"
)

func TestElementOuterHTML(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d" class="a"><span>hi &amp; bye</span></div>`)
	el := doc.GetElementByID("d")
	got, err := el.OuterHTML()
	if err != nil {
		t.Fatalf("OuterHTML: %v", err)
	}
	want := `<div id="d" class="a"><span>hi &amp; bye</span></div>`
	if got != want {
		t.Errorf("OuterHTML() = %q, want %q", got, want)
	}
}

func TestElementInnerHTML(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">a<span>b</span>c</div>`)
	el := doc.GetElementByID("d")
	got, err := el.InnerHTML()
	if err != nil {
		t.Fatalf("InnerHTML: %v", err)
	}
	want := `a<span>b</span>c`
	if got != want {
		t.Errorf("InnerHTML() = %q, want %q", got, want)
	}
}

func TestElementInnerHTMLEmpty(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d"></div>`)
	el := doc.GetElementByID("d")
	got, err := el.InnerHTML()
	if err != nil {
		t.Fatalf("InnerHTML: %v", err)
	}
	if got != "" {
		t.Errorf("InnerHTML() = %q, want \"\"", got)
	}
}

func TestElementOuterInnerHTMLVoidElement(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">a<br>b</div>`)
	el := doc.GetElementByID("d")
	got, err := el.InnerHTML()
	if err != nil {
		t.Fatalf("InnerHTML: %v", err)
	}
	want := `a<br/>b`
	if got != want {
		t.Errorf("InnerHTML() = %q, want %q", got, want)
	}
}

func TestElementOuterHTMLStripsFocusMarker(t *testing.T) {
	doc := mustParseDoc(t, `<input id="in" type="text">`)
	in := doc.GetElementByID("in")
	if !in.Focus() {
		t.Fatal("Focus() returned false")
	}
	got, err := in.OuterHTML()
	if err != nil {
		t.Fatalf("OuterHTML: %v", err)
	}
	if strings.Contains(got, "data-htmlterm-focus") {
		t.Errorf("OuterHTML() = %q, leaked internal focus marker attribute", got)
	}
}

func TestElementSetID(t *testing.T) {
	doc := mustParseDoc(t, `<div>x</div>`)
	el := doc.QuerySelector("div")
	el.SetID("foo")
	if got, want := el.ID(), "foo"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
	if doc.GetElementByID("foo") == nil {
		t.Error("GetElementByID(foo) = nil after SetID")
	}
}

func TestElementOwnerDocument(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">x</div>`)
	el := doc.GetElementByID("d")
	if el.OwnerDocument() != doc {
		t.Error("OwnerDocument() did not return the owning Document")
	}

	fresh := doc.CreateElement("span")
	if fresh.OwnerDocument() != doc {
		t.Error("OwnerDocument() on a CreateElement result did not return the owning Document")
	}
}

func TestElementClassName(t *testing.T) {
	doc := mustParseDoc(t, `<div class="a b">x</div>`)
	el := doc.QuerySelector("div")
	if got, want := el.ClassName(), "a b"; got != want {
		t.Errorf("ClassName() = %q, want %q", got, want)
	}
	el.SetClassName("c d")
	if got, want := el.ClassName(), "c d"; got != want {
		t.Errorf("after SetClassName: ClassName() = %q, want %q", got, want)
	}
	if !el.ClassList().Contains("c") {
		t.Error("SetClassName should be reflected in ClassList")
	}
}

func TestElementHasAttributesAndGetAttributeNames(t *testing.T) {
	doc := mustParseDoc(t, `<div>x</div>`)
	el := doc.QuerySelector("div")
	if el.HasAttributes() {
		t.Error("HasAttributes() = true for an element with no attributes")
	}
	if got := el.GetAttributeNames(); len(got) != 0 {
		t.Errorf("GetAttributeNames() = %v, want empty", got)
	}

	el.SetAttribute("id", "x")
	el.SetAttribute("data-foo", "bar")
	if !el.HasAttributes() {
		t.Error("HasAttributes() = false after SetAttribute")
	}
	want := []string{"id", "data-foo"}
	if got := el.GetAttributeNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetAttributeNames() = %v, want %v", got, want)
	}
}

func TestElementChildElementCount(t *testing.T) {
	doc := mustParseDoc(t, `<div>text<span>a</span>more<span>b</span></div>`)
	el := doc.QuerySelector("div")
	if got, want := el.ChildElementCount(), 2; got != want {
		t.Errorf("ChildElementCount() = %d, want %d", got, want)
	}
}

func TestElementQuerySelectorScopedToSubtree(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><span class="hit">a</span></div><span class="hit">b</span>`)
	outer := doc.GetElementByID("outer")

	got := outer.QuerySelector(".hit")
	if got == nil {
		t.Fatal("QuerySelector(.hit) = nil")
	}
	if got.TextContent() != "a" {
		t.Errorf("QuerySelector found %q, want the descendant inside #outer", got.TextContent())
	}

	all := outer.QuerySelectorAll(".hit")
	if len(all) != 1 {
		t.Errorf("QuerySelectorAll(.hit) found %d elements, want 1 (scoped to #outer only)", len(all))
	}
}

func TestElementQuerySelectorNeverMatchesSelf(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer" class="hit"><span class="hit">a</span></div>`)
	outer := doc.GetElementByID("outer")

	got := outer.QuerySelector(".hit")
	if got == nil {
		t.Fatal("QuerySelector(.hit) = nil")
	}
	if got.TagName() != "span" {
		t.Errorf("QuerySelector(.hit) matched %q, want it to skip the context node itself and find the span descendant", got.TagName())
	}
}

func TestElementGetElementsByClassNameAndTagName(t *testing.T) {
	doc := mustParseDoc(t, `<div id="outer"><p class="a b">1</p><p class="a">2</p><span class="a b">3</span></div>`)
	outer := doc.GetElementByID("outer")

	if got := outer.GetElementsByClassName("a b"); len(got) != 2 {
		t.Errorf("GetElementsByClassName(\"a b\") found %d, want 2", len(got))
	}
	if got := outer.GetElementsByClassName("a"); len(got) != 3 {
		t.Errorf("GetElementsByClassName(\"a\") found %d, want 3", len(got))
	}
	if got := outer.GetElementsByTagName("p"); len(got) != 2 {
		t.Errorf("GetElementsByTagName(\"p\") found %d, want 2", len(got))
	}
}

func TestDocumentGetElementsByClassNameTagNameName(t *testing.T) {
	doc := mustParseDoc(t, `<p class="a b">1</p><p class="a">2</p><input name="who" value="x">`)

	if got := doc.GetElementsByClassName("a b"); len(got) != 1 {
		t.Errorf("GetElementsByClassName(\"a b\") found %d, want 1", len(got))
	}
	if got := doc.GetElementsByClassName("a"); len(got) != 2 {
		t.Errorf("GetElementsByClassName(\"a\") found %d, want 2", len(got))
	}
	if got := doc.GetElementsByTagName("p"); len(got) != 2 {
		t.Errorf("GetElementsByTagName(\"p\") found %d, want 2", len(got))
	}
	if got := doc.GetElementsByName("who"); len(got) != 1 {
		t.Errorf("GetElementsByName(\"who\") found %d, want 1", len(got))
	}
}

func TestDocumentGetElementsByNameWithQuoteInName(t *testing.T) {
	// GetElementsByName builds its selector via fmt.Sprintf("[name=%q]",
	// name), which backslash-escapes a quote character in name — the
	// selector-value parser must unescape that back to a literal quote, or a
	// name containing one can never match.
	doc := mustParseDoc(t, `<input name="a&quot;b" value="x"><input name="ab" value="y">`)

	if got := doc.GetElementsByName(`a"b`); len(got) != 1 {
		t.Errorf(`GetElementsByName("a\"b") found %d, want 1`, len(got))
	} else if v := got[0].Value(); v != "x" {
		t.Errorf(`GetElementsByName("a\"b")[0].Value() = %q, want "x"`, v)
	}
}

func TestDocumentBodyHeadTitle(t *testing.T) {
	doc := mustParseDoc(t, `<html><head><title>Hello</title></head><body><p>x</p></body></html>`)

	if doc.Body() == nil || doc.Body().TagName() != "body" {
		t.Error("Body() did not return the <body> element")
	}
	if doc.Head() == nil || doc.Head().TagName() != "head" {
		t.Error("Head() did not return the <head> element")
	}
	if got, want := doc.Title(), "Hello"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
}

func TestDocumentTitleEmptyWhenMissing(t *testing.T) {
	doc := mustParseDoc(t, `<p>x</p>`)
	if got := doc.Title(); got != "" {
		t.Errorf("Title() = %q, want \"\" when there is no <title>", got)
	}
}

func TestElementRemove(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span id="child">x</span></div>`)
	child := doc.GetElementByID("child")
	child.Remove()
	if doc.GetElementByID("child") != nil {
		t.Error("element still findable after Remove()")
	}

	// Removing an already-detached (or parentless) element is a no-op, not
	// a panic.
	child.Remove()
}

func TestElementBeforeAfter(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span id="mid">mid</span></div>`)
	parent := doc.GetElementByID("parent")
	mid := doc.GetElementByID("mid")

	before := doc.CreateElement("b")
	before.SetID("before")
	mid.Before(before)

	after := doc.CreateElement("i")
	after.SetID("after")
	mid.After(after)

	var order []string
	for c := parent.FirstElementChild(); c != nil; c = c.NextElementSibling() {
		order = append(order, c.ID())
	}
	want := []string{"before", "mid", "after"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("child order = %v, want %v", order, want)
	}
}

func TestElementReplaceWith(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span id="old">x</span></div>`)
	parent := doc.GetElementByID("parent")
	old := doc.GetElementByID("old")

	replacement := doc.CreateElement("b")
	replacement.SetID("new")
	old.ReplaceWith(replacement)

	if doc.GetElementByID("old") != nil {
		t.Error("old element still findable after ReplaceWith")
	}
	if parent.FirstElementChild().ID() != "new" {
		t.Error("replacement element not spliced into parent's children")
	}
}

func TestElementReplaceChildren(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span>1</span><span>2</span></div>`)
	parent := doc.GetElementByID("parent")

	a := doc.CreateElement("b")
	a.SetAttribute("id", "a")
	b := doc.CreateElement("i")
	b.SetAttribute("id", "b")
	parent.ReplaceChildren(a, b)

	if got, want := parent.ChildElementCount(), 2; got != want {
		t.Fatalf("ChildElementCount() = %d, want %d", got, want)
	}
	if parent.FirstElementChild().TagName() != "b" || parent.LastElementChild().TagName() != "i" {
		t.Error("ReplaceChildren did not install newChildren in order")
	}
}

func TestElementClick(t *testing.T) {
	doc := mustParseDoc(t, `<input type="checkbox" id="cb">`)
	cb := doc.GetElementByID("cb")
	if cb.Checked() {
		t.Fatal("checkbox unexpectedly checked before Click")
	}
	if !cb.Click() {
		t.Fatal("Click() returned false")
	}
	if !cb.Checked() {
		t.Error("Click() did not toggle the checkbox")
	}
}

func TestElementClickUnrendered(t *testing.T) {
	// A freshly created, unattached element has no Rect, so Click is a
	// documented no-op rather than a panic.
	docNoRender, err := document.ParseDocument(`<div id="d">x</div>`, mustOptions())
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	el := docNoRender.GetElementByID("d")
	if el.Click() {
		t.Error("Click() = true before Render has ever run, want false")
	}
}

func TestDocumentCreateComment(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">a</div>`)
	el := doc.GetElementByID("d")
	el.AppendChild(doc.CreateComment("hidden note"))

	got, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "hidden note") {
		t.Errorf("Render() = %q, comment text leaked into output", got)
	}

	nodes := el.ChildNodes()
	if len(nodes) != 2 {
		t.Fatalf("ChildNodes() = %d nodes, want 2 (text + comment)", len(nodes))
	}
	if got, want := nodes[1].NodeValue(), "hidden note"; got != want {
		t.Errorf("comment NodeValue() = %q, want %q", got, want)
	}
}

func TestDocumentFormsImagesLinksScriptsStyleSheets(t *testing.T) {
	doc := mustParseDoc(t, `
		<form id="f"></form>
		<img id="i" src="x.png">
		<a id="a1" href="/one">one</a>
		<a id="a2">no href</a>
		<area href="/two">
		<script id="s">1</script>
		<style id="st">p{color:red}</style>
	`)
	if got := doc.Forms(); len(got) != 1 || got[0].ID() != "f" {
		t.Errorf("Forms() = %v, want [f]", ids(got))
	}
	if got := doc.Images(); len(got) != 1 || got[0].ID() != "i" {
		t.Errorf("Images() = %v, want [i]", ids(got))
	}
	if got := doc.Links(); len(got) != 2 {
		t.Errorf("Links() found %d, want 2 (a[href] and area[href], excluding the href-less <a>)", len(got))
	}
	if got := doc.Scripts(); len(got) != 1 || got[0].ID() != "s" {
		t.Errorf("Scripts() = %v, want [s]", ids(got))
	}
	if got := doc.StyleSheets(); len(got) != 1 || got[0].ID() != "st" {
		t.Errorf("StyleSheets() = %v, want [st]", ids(got))
	}
}

func ids(els []*document.Element) []string {
	out := make([]string, len(els))
	for i, e := range els {
		out[i] = e.ID()
	}
	return out
}

func TestElementNodeValue(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">hello</div>`)
	el := doc.GetElementByID("d")
	if got := el.NodeValue(); got != "" {
		t.Errorf("NodeValue() on element = %q, want \"\"", got)
	}
	text := el.FirstChild()
	if got, want := text.NodeValue(), "hello"; got != want {
		t.Errorf("NodeValue() on text node = %q, want %q", got, want)
	}
}

func TestElementSetTextContent(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d"><span>old</span></div>`)
	el := doc.GetElementByID("d")
	el.SetTextContent("new text")
	if got, want := el.TextContent(), "new text"; got != want {
		t.Errorf("TextContent() = %q, want %q", got, want)
	}
	if got, want := el.ChildElementCount(), 0; got != want {
		t.Errorf("ChildElementCount() = %d, want %d (old <span> should be gone)", got, want)
	}

	el.SetTextContent("")
	if got := len(el.ChildNodes()); got != 0 {
		t.Errorf("ChildNodes() after SetTextContent(\"\") = %d, want 0", got)
	}
}

func TestElementChildNodesIncludesText(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">a<span>b</span>c</div>`)
	el := doc.GetElementByID("d")
	nodes := el.ChildNodes()
	if len(nodes) != 3 {
		t.Fatalf("ChildNodes() = %d, want 3 (text, span, text)", len(nodes))
	}
	if nodes[0].NodeValue() != "a" || nodes[2].NodeValue() != "c" {
		t.Errorf("ChildNodes() text values = %q, %q, want \"a\", \"c\"", nodes[0].NodeValue(), nodes[2].NodeValue())
	}
	if nodes[1].TagName() != "span" {
		t.Errorf("ChildNodes()[1].TagName() = %q, want \"span\"", nodes[1].TagName())
	}
	// Children() should still filter to elements only.
	if got := len(el.Children()); got != 1 {
		t.Errorf("Children() = %d, want 1", got)
	}
}

func TestElementIsSameNode(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">x</div><span id="s">y</span>`)
	d1 := doc.GetElementByID("d")
	d2 := doc.GetElementByID("d")
	s := doc.GetElementByID("s")

	if !d1.IsSameNode(d2) {
		t.Error("two handles for the same node should be IsSameNode")
	}
	if d1.IsSameNode(s) {
		t.Error("distinct nodes should not be IsSameNode")
	}
}

func TestElementHidden(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d">x</div>`)
	el := doc.GetElementByID("d")
	if el.Hidden() {
		t.Error("Hidden() = true before SetHidden")
	}
	el.SetHidden(true)
	if !el.Hidden() {
		t.Error("Hidden() = false after SetHidden(true)")
	}
	if !el.HasAttribute("hidden") {
		t.Error("SetHidden(true) did not set the hidden attribute")
	}
	el.SetHidden(false)
	if el.Hidden() || el.HasAttribute("hidden") {
		t.Error("SetHidden(false) did not clear the hidden attribute")
	}
}

func TestElementInsertAdjacentElement(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span id="mid">mid</span></div>`)
	parent := doc.GetElementByID("parent")
	mid := doc.GetElementByID("mid")

	bb := doc.CreateElement("b")
	bb.SetID("beforebegin")
	if err := mid.InsertAdjacentElement("beforebegin", bb); err != nil {
		t.Fatalf("InsertAdjacentElement(beforebegin): %v", err)
	}

	ae := doc.CreateElement("i")
	ae.SetID("afterend")
	if err := mid.InsertAdjacentElement("afterend", ae); err != nil {
		t.Fatalf("InsertAdjacentElement(afterend): %v", err)
	}

	ab := doc.CreateElement("u")
	ab.SetID("afterbegin")
	if err := mid.InsertAdjacentElement("afterbegin", ab); err != nil {
		t.Fatalf("InsertAdjacentElement(afterbegin): %v", err)
	}

	be := doc.CreateElement("em")
	be.SetID("beforeend")
	if err := mid.InsertAdjacentElement("beforeend", be); err != nil {
		t.Fatalf("InsertAdjacentElement(beforeend): %v", err)
	}

	var order []string
	for c := parent.FirstElementChild(); c != nil; c = c.NextElementSibling() {
		order = append(order, c.ID())
	}
	want := []string{"beforebegin", "mid", "afterend"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("parent child order = %v, want %v", order, want)
	}

	var midOrder []string
	for c := mid.FirstElementChild(); c != nil; c = c.NextElementSibling() {
		midOrder = append(midOrder, c.ID())
	}
	wantMid := []string{"afterbegin", "beforeend"}
	if !reflect.DeepEqual(midOrder, wantMid) {
		t.Errorf("mid child order = %v, want %v", midOrder, wantMid)
	}

	if err := mid.InsertAdjacentElement("nonsense", bb); err == nil {
		t.Error("InsertAdjacentElement with an invalid position should return an error")
	}
}

func TestElementInsertAdjacentText(t *testing.T) {
	doc := mustParseDoc(t, `<div id="parent"><span id="mid">mid</span></div>`)
	mid := doc.GetElementByID("mid")
	if err := mid.InsertAdjacentText("beforeend", "!"); err != nil {
		t.Fatalf("InsertAdjacentText: %v", err)
	}
	if got, want := mid.TextContent(), "mid!"; got != want {
		t.Errorf("TextContent() = %q, want %q", got, want)
	}
}

func TestElementDataset(t *testing.T) {
	doc := mustParseDoc(t, `<div id="d" data-foo-bar="1">x</div>`)
	el := doc.GetElementByID("d")
	ds := el.Dataset()

	if got, ok := ds.Get("fooBar"); !ok || got != "1" {
		t.Errorf("Get(fooBar) = (%q, %v), want (1, true)", got, ok)
	}
	if !ds.Has("fooBar") {
		t.Error("Has(fooBar) = false")
	}
	if ds.Has("nope") {
		t.Error("Has(nope) = true, want false")
	}

	ds.Set("newKey", "v")
	if got, want := el.GetAttributeNames(), []string{"id", "data-foo-bar", "data-new-key"}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetAttributeNames() = %v, want %v", got, want)
	}

	ds.Delete("fooBar")
	if ds.Has("fooBar") {
		t.Error("Has(fooBar) = true after Delete")
	}
	if el.HasAttribute("data-foo-bar") {
		t.Error("data-foo-bar attribute still present after Delete")
	}
}

func mustOptions() document.Options {
	return document.Options{Width: 20}
}
