package document

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// Element is a DOM-like handle onto a single element node in a Document's
// tree. Mutations made through an Element are visible immediately in the
// underlying tree, including in any Element obtained separately for the
// same node, and take effect the next time Document.Render is called.
//
// doc is the owning Document, threaded through every Element constructor in
// this package (GetElementByID, QuerySelector[All], the tree-navigation
// methods below, Event.Target/CurrentTarget, etc.) so that Focus/Blur/Rect —
// which need Document-level state (the focused node, the last Render's
// position map) — can be called directly on the element, matching the DOM's
// HTMLElement.focus()/blur()/getBoundingClientRect() shape instead of taking
// an *Element parameter on Document. It is nil only for a zero-value or
// var-declared *Element a caller constructs itself outside this package;
// Focus/Blur/Rect degrade to safe no-ops/false in that case rather than
// panicking.
type Element struct {
	node *html.Node
	doc  *Document
}

// TagName returns the element's tag name, e.g. "div" or "input".
func (e *Element) TagName() string {
	return e.node.Data
}

// ID returns the element's id attribute, or "" if unset.
func (e *Element) ID() string {
	return nodeAttr(e.node, "id")
}

// TextContent returns the concatenated text of all descendant text nodes.
func (e *Element) TextContent() string {
	return rawContent(e.node)
}

// GetAttribute returns the named attribute's value and whether it is
// present.
func (e *Element) GetAttribute(name string) (string, bool) {
	for _, a := range e.node.Attr {
		if a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

// HasAttribute reports whether the named attribute is present.
func (e *Element) HasAttribute(name string) bool {
	_, ok := e.GetAttribute(name)
	return ok
}

// SetAttribute sets the named attribute, adding it if not already present.
func (e *Element) SetAttribute(name, value string) {
	setAttr(e.node, name, value)
}

// RemoveAttribute removes the named attribute, if present.
func (e *Element) RemoveAttribute(name string) {
	removeAttr(e.node, name)
}

// Value returns the element's value attribute, e.g. for a text input, or —
// for a <select> — the currently selected option's value, falling back to
// its text content if it has no value attribute (mirroring the DOM's
// HTMLSelectElement.value; see selectValue).
func (e *Element) Value() string {
	if isSelectControl(e.node) {
		return selectValue(e.node)
	}
	return nodeAttr(e.node, "value")
}

// SetValue sets the element's value attribute, or — for a <select> — marks
// the option whose value matches v as selected, leaving it unchanged if none
// matches (mirroring the DOM's HTMLSelectElement.value setter; see
// setSelectValue).
func (e *Element) SetValue(v string) {
	if isSelectControl(e.node) {
		setSelectValue(e.node, v)
		return
	}
	e.SetAttribute("value", v)
}

// Checked reports whether the element's checked attribute is present, e.g.
// for a checkbox or radio input.
func (e *Element) Checked() bool {
	return e.HasAttribute("checked")
}

// SetChecked sets or clears the element's checked attribute.
func (e *Element) SetChecked(v bool) {
	if v {
		e.SetAttribute("checked", "")
	} else {
		e.RemoveAttribute("checked")
	}
}

// Parent returns e's parent node, or nil if e has none (e.g. the document
// root — see Document.DocumentElement).
func (e *Element) Parent() *Element {
	if e.node.Parent == nil {
		return nil
	}
	return &Element{node: e.node.Parent, doc: e.doc}
}

// NextSibling returns e's next sibling node, which may be a text node rather
// than an element — see NextElementSibling for an element-only view — or nil
// if e is its parent's last child.
func (e *Element) NextSibling() *Element {
	if e.node.NextSibling == nil {
		return nil
	}
	return &Element{node: e.node.NextSibling, doc: e.doc}
}

// PreviousSibling is NextSibling's counterpart, toward e's parent's first
// child.
func (e *Element) PreviousSibling() *Element {
	if e.node.PrevSibling == nil {
		return nil
	}
	return &Element{node: e.node.PrevSibling, doc: e.doc}
}

// FirstChild returns e's first child node, which may be a text node rather
// than an element — see FirstElementChild for an element-only view — or nil
// if e has no children.
func (e *Element) FirstChild() *Element {
	if e.node.FirstChild == nil {
		return nil
	}
	return &Element{node: e.node.FirstChild, doc: e.doc}
}

// LastChild is FirstChild's counterpart.
func (e *Element) LastChild() *Element {
	if e.node.LastChild == nil {
		return nil
	}
	return &Element{node: e.node.LastChild, doc: e.doc}
}

// NextElementSibling returns e's next sibling that is itself an element,
// skipping any intervening text/comment nodes, or nil if there is none.
func (e *Element) NextElementSibling() *Element {
	for n := e.node.NextSibling; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n, doc: e.doc}
		}
	}
	return nil
}

// PreviousElementSibling is NextElementSibling's counterpart.
func (e *Element) PreviousElementSibling() *Element {
	for n := e.node.PrevSibling; n != nil; n = n.PrevSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n, doc: e.doc}
		}
	}
	return nil
}

// FirstElementChild returns e's first child that is itself an element, or
// nil if it has none.
func (e *Element) FirstElementChild() *Element {
	for n := e.node.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n, doc: e.doc}
		}
	}
	return nil
}

// LastElementChild is FirstElementChild's counterpart.
func (e *Element) LastElementChild() *Element {
	for n := e.node.LastChild; n != nil; n = n.PrevSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n, doc: e.doc}
		}
	}
	return nil
}

// Children returns e's direct element children, in document order (text and
// comment node children excluded).
func (e *Element) Children() []*Element {
	var out []*Element
	for n := e.node.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			out = append(out, &Element{node: n, doc: e.doc})
		}
	}
	return out
}

// AppendChild adds child as e's last child — mirroring the DOM's
// Node.appendChild. child must be freshly created (e.g. via
// Document.CreateElement/CreateTextNode or Element.CloneNode) or already
// removed from wherever it was (see RemoveChild); like the underlying
// golang.org/x/net/html.Node.AppendChild, it panics if child is still
// attached anywhere in the tree.
func (e *Element) AppendChild(child *Element) {
	e.node.AppendChild(child.node)
}

// InsertBefore inserts newChild immediately before oldChild among e's
// children, or appends it as the last child if oldChild is nil — mirroring
// the DOM's Node.insertBefore. newChild must not already be attached
// anywhere in the tree, same as AppendChild.
func (e *Element) InsertBefore(newChild, oldChild *Element) {
	var old *html.Node
	if oldChild != nil {
		old = oldChild.node
	}
	e.node.InsertBefore(newChild.node, old)
}

// RemoveChild removes child from e's children and returns it, now detached
// (no parent, no siblings) and safe to re-attach elsewhere via AppendChild/
// InsertBefore — mirroring the DOM's Node.removeChild. Panics if child is
// not currently a child of e, matching the underlying
// golang.org/x/net/html.Node.RemoveChild. If the currently focused element
// is child or one of its descendants, focus is silently cleared (no "blur"
// dispatched — the element is gone, not blurred), the same behavior
// SetInnerHTML/SetPreRendered already have for a wholesale subtree
// replacement (see Document.clearFocusIfDetached). Listeners registered on
// now-detached descendants become unreachable, same as SetInnerHTML — call
// Document.RemoveEventListener first if that matters.
func (e *Element) RemoveChild(child *Element) *Element {
	e.node.RemoveChild(child.node)
	if e.doc != nil {
		e.doc.clearFocusIfDetached()
	}
	return child
}

// ReplaceChild replaces oldChild with newChild among e's children and
// returns oldChild, now detached — mirroring the DOM's
// Node.replaceChild(newChild, oldChild) (note the argument order: new node
// first, matching the DOM rather than RemoveChild's single-argument shape).
// newChild must not already be attached anywhere in the tree (see
// AppendChild). Panics if oldChild is not currently a child of e. Focus/
// listener handling for oldChild's subtree is identical to RemoveChild.
func (e *Element) ReplaceChild(newChild, oldChild *Element) *Element {
	if oldChild.node.Parent != e.node {
		panic("htmlterm: ReplaceChild called for a non-child Node")
	}
	e.node.InsertBefore(newChild.node, oldChild.node)
	e.node.RemoveChild(oldChild.node)
	if e.doc != nil {
		e.doc.clearFocusIfDetached()
	}
	return oldChild
}

// CloneNode returns a detached copy of e — mirroring the DOM's
// Node.cloneNode(deep). If deep is true, e's whole subtree is copied; if
// false, only e itself (with its attributes, but no children) is copied.
// The clone must be attached via AppendChild/InsertBefore before it appears
// in Render output. Event listeners are never copied, matching the DOM
// (cloneNode never copies listeners either).
//
// The three reserved state-marker attributes this package reflects into the
// tree as real attributes — focusAttr, selectOpenAttr, selectHighlightAttr
// (see event.go/select.go) — are deliberately not copied, even though a
// real DOM clone would copy every attribute verbatim: unlike a browser,
// where focus/open-popup state is never an HTML attribute in the first
// place, those three *are* attributes here, so a literal copy would produce
// a second element that matches ":focus" (or renders as an open dropdown)
// the moment it's attached, alongside the original — clearly not what a
// caller cloning a focused input or an open <select> wants.
func (e *Element) CloneNode(deep bool) *Element {
	return &Element{node: cloneHTMLNode(e.node, deep), doc: e.doc}
}

// cloneHTMLNode is CloneNode's recursive implementation. golang.org/x/net/
// html has no built-in clone, and its Node.AppendChild panics on a node that
// already has parent/sibling pointers set, so reusing subtree nodes directly
// isn't an option — every node needs a fresh copy.
func cloneHTMLNode(n *html.Node, deep bool) *html.Node {
	clone := &html.Node{
		Type:      n.Type,
		DataAtom:  n.DataAtom,
		Data:      n.Data,
		Namespace: n.Namespace,
	}
	for _, a := range n.Attr {
		if a.Key == focusAttr || a.Key == selectOpenAttr || a.Key == selectHighlightAttr {
			continue
		}
		clone.Attr = append(clone.Attr, a)
	}
	if deep {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			clone.AppendChild(cloneHTMLNode(c, true))
		}
	}
	return clone
}

// Focus moves focus to e, setting the reserved focusAttr marker that
// ":focus" matches against and dispatching "blur"/"focus" events (neither of
// which bubbles, per DOM semantics) for the previously and newly focused
// elements — mirroring the DOM's HTMLElement.focus(). Returns false, making
// no change, if e is nil, not attached to a Document, or not focusable (a
// disabled element, or one that isn't a tab-stoppable form control or
// scroll container — see Document.isFocusable).
func (e *Element) Focus() bool {
	if e == nil || e.doc == nil {
		return false
	}
	return e.doc.focus(e)
}

// Blur removes focus from e, dispatching "blur", if e is the currently
// focused element — mirroring the DOM's HTMLElement.blur(). A no-op if e is
// nil, not attached to a Document, or not the currently focused element
// (matching a real browser, where blur() on an element that isn't focused
// does nothing).
func (e *Element) Blur() {
	if e == nil || e.doc == nil || e.doc.focused != e.node {
		return
	}
	e.doc.blur()
}

// Rect returns e's position and size as of the most recent Document.Render
// call (the CSS border box — content+padding+border, excluding margin — see
// RENDERING.md's Position tracking section for the exact semantics and its
// documented approximations), and whether a position was recorded for it at
// all — mirroring (approximately; see above) the DOM's
// getBoundingClientRect(). A position is recorded for every element that
// produces its own box during composition (block-level elements, tables,
// lists, inline-block elements including form controls, and plain inline
// elements like <span>/<label> reached via token-splicing) — see inline.go's
// and render.go's "default" dispatch cases for exactly which elements that
// covers, and their doc comments for the specific, uncommon combinations (a
// hyperlink or another inline-block wrapping a further trackable descendant)
// where a nested element's own Rect isn't tracked. ok is false if e is nil,
// not attached to a Document, Render hasn't been called yet, or e has no
// recorded position (e.g. display:none, or one of those documented gaps).
func (e *Element) Rect() (Rect, bool) {
	if e == nil || e.doc == nil {
		return Rect{}, false
	}
	return e.doc.rect(e)
}

// ClassList returns a handle for reading and mutating the element's class
// attribute as a set of whitespace-separated tokens.
func (e *Element) ClassList() *ClassList {
	return &ClassList{el: e}
}

// IsTextEntry reports whether e is a <textarea> or a text-like <input>
// (any type other than checkbox/radio/submit/button/reset/hidden) — the
// elements DispatchKey's printable-character and Backspace default actions
// act on, and the set a host's own cursor-placement logic (e.g. tui's
// focusCursorPos) should treat as having a text insertion point.
func (e *Element) IsTextEntry() bool {
	return isTextEntry(e.node)
}

// ClassList is a DOM-like handle onto an element's class attribute, tokenized
// the same way selector class matching tokenizes it (see matchPart).
type ClassList struct {
	el *Element
}

func (c *ClassList) classes() []string {
	return strings.Fields(nodeAttr(c.el.node, "class"))
}

// Contains reports whether cls is present in the class list.
func (c *ClassList) Contains(cls string) bool {
	return slices.Contains(c.classes(), cls)
}

// Add adds cls to the class list, if not already present.
func (c *ClassList) Add(cls string) {
	if c.Contains(cls) {
		return
	}
	cur := append(c.classes(), cls)
	c.el.SetAttribute("class", strings.Join(cur, " "))
}

// Remove removes cls from the class list, if present.
func (c *ClassList) Remove(cls string) {
	cur := c.classes()
	out := cur[:0]
	for _, x := range cur {
		if x != cls {
			out = append(out, x)
		}
	}
	c.el.SetAttribute("class", strings.Join(out, " "))
}

// Toggle adds cls if absent and removes it if present, returning whether it
// is present after the call.
func (c *ClassList) Toggle(cls string) bool {
	if c.Contains(cls) {
		c.Remove(cls)
		return false
	}
	c.Add(cls)
	return true
}
