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
type Element struct {
	node *html.Node
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
	return &Element{node: e.node.Parent}
}

// NextSibling returns e's next sibling node, which may be a text node rather
// than an element — see NextElementSibling for an element-only view — or nil
// if e is its parent's last child.
func (e *Element) NextSibling() *Element {
	if e.node.NextSibling == nil {
		return nil
	}
	return &Element{node: e.node.NextSibling}
}

// PreviousSibling is NextSibling's counterpart, toward e's parent's first
// child.
func (e *Element) PreviousSibling() *Element {
	if e.node.PrevSibling == nil {
		return nil
	}
	return &Element{node: e.node.PrevSibling}
}

// FirstChild returns e's first child node, which may be a text node rather
// than an element — see FirstElementChild for an element-only view — or nil
// if e has no children.
func (e *Element) FirstChild() *Element {
	if e.node.FirstChild == nil {
		return nil
	}
	return &Element{node: e.node.FirstChild}
}

// LastChild is FirstChild's counterpart.
func (e *Element) LastChild() *Element {
	if e.node.LastChild == nil {
		return nil
	}
	return &Element{node: e.node.LastChild}
}

// NextElementSibling returns e's next sibling that is itself an element,
// skipping any intervening text/comment nodes, or nil if there is none.
func (e *Element) NextElementSibling() *Element {
	for n := e.node.NextSibling; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n}
		}
	}
	return nil
}

// PreviousElementSibling is NextElementSibling's counterpart.
func (e *Element) PreviousElementSibling() *Element {
	for n := e.node.PrevSibling; n != nil; n = n.PrevSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n}
		}
	}
	return nil
}

// FirstElementChild returns e's first child that is itself an element, or
// nil if it has none.
func (e *Element) FirstElementChild() *Element {
	for n := e.node.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n}
		}
	}
	return nil
}

// LastElementChild is FirstElementChild's counterpart.
func (e *Element) LastElementChild() *Element {
	for n := e.node.LastChild; n != nil; n = n.PrevSibling {
		if n.Type == html.ElementNode {
			return &Element{node: n}
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
			out = append(out, &Element{node: n})
		}
	}
	return out
}

// AppendChild adds child as e's last child — mirroring the DOM's
// Node.appendChild. child must be freshly created (e.g. via
// Document.CreateElement/CreateTextNode) and not already attached anywhere
// in the tree; like the underlying golang.org/x/net/html.Node.AppendChild,
// it panics otherwise. There is no removeChild/replaceChild yet to detach an
// already-attached node first (planned separately), so this only covers
// building up new content, not relocating existing nodes.
func (e *Element) AppendChild(child *Element) {
	e.node.AppendChild(child.node)
}

// InsertBefore inserts newChild immediately before oldChild among e's
// children, or appends it as the last child if oldChild is nil — mirroring
// the DOM's Node.insertBefore. newChild must be freshly created and not
// already attached anywhere in the tree, same as AppendChild.
func (e *Element) InsertBefore(newChild, oldChild *Element) {
	var old *html.Node
	if oldChild != nil {
		old = oldChild.node
	}
	e.node.InsertBefore(newChild.node, old)
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
