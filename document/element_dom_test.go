package document_test

import (
	"reflect"
	"testing"

	"github.com/client9/htmlterm/document"
)

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

func mustOptions() document.Options {
	return document.Options{Width: 20}
}
