package document

import (
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

// Value returns the element's value attribute, e.g. for a text input.
func (e *Element) Value() string {
	return nodeAttr(e.node, "value")
}

// SetValue sets the element's value attribute.
func (e *Element) SetValue(v string) {
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
	for _, x := range c.classes() {
		if x == cls {
			return true
		}
	}
	return false
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
