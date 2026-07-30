package document

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
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

// SetID sets the element's id attribute to v — mirroring the DOM's
// Element.id setter. Equivalent to SetAttribute("id", v).
func (e *Element) SetID(v string) {
	e.SetAttribute("id", v)
}

// OwnerDocument returns the Document e belongs to, or nil if e was
// constructed outside a Document (e.g. a zero-value or var-declared
// *Element) — mirroring the DOM's Node.ownerDocument.
func (e *Element) OwnerDocument() *Document {
	return e.doc
}

// ClassName returns the element's class attribute as a raw,
// whitespace-separated string — mirroring the DOM's Element.className. See
// ClassList for a token-oriented view of the same attribute.
func (e *Element) ClassName() string {
	return nodeAttr(e.node, "class")
}

// SetClassName sets the element's class attribute to v — mirroring the
// DOM's Element.className setter. Equivalent to SetAttribute("class", v).
func (e *Element) SetClassName(v string) {
	e.SetAttribute("class", v)
}

// TextContent returns the concatenated text of all descendant text nodes.
func (e *Element) TextContent() string {
	return rawContent(e.node)
}

// SetTextContent replaces all of e's children with a single text node
// carrying text, or removes all children if text is "" — mirroring the
// DOM's Node.textContent setter (which pushes nothing for an empty string
// rather than an empty text node, per spec's algorithm).
func (e *Element) SetTextContent(text string) {
	if text == "" {
		e.ReplaceChildren()
		return
	}
	e.ReplaceChildren(&Element{node: &html.Node{Type: html.TextNode, Data: text}, doc: e.doc})
}

// NodeValue returns e's own text, for a text or comment node — mirroring
// the DOM's Node.nodeValue. Returns "" for an element node (matching
// spec's null there), or any other node type this package barely models.
func (e *Element) NodeValue() string {
	if e.node.Type == html.TextNode || e.node.Type == html.CommentNode {
		return e.node.Data
	}
	return ""
}

// IsSameNode reports whether e and other refer to the exact same
// underlying node — mirroring the DOM's Node.isSameNode(). Two separate
// *Element handles obtained for the same node (e.g. from two different
// QuerySelector calls) are still the "same node". There is no
// isEqualNode() equivalent (deep structural comparison of two distinct
// nodes) — see docs/DOM_API.md.
func (e *Element) IsSameNode(other *Element) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.node == other.node
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

// HasAttributes reports whether e has any attributes at all — mirroring
// the DOM's Node.hasAttributes().
func (e *Element) HasAttributes() bool {
	return len(e.node.Attr) > 0
}

// GetAttributeNames returns the names of all of e's attributes, in the
// order they appear on the element — mirroring the DOM's
// Element.getAttributeNames().
func (e *Element) GetAttributeNames() []string {
	names := make([]string, len(e.node.Attr))
	for i, a := range e.node.Attr {
		names[i] = a.Key
	}
	return names
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
// setSelectValue). For a text entry, this also collapses any selection to
// the end of the new value — mirroring real spec's behavior for
// programmatic value assignment (setSelectionRange, not .value, is the way
// to set an arbitrary caret position).
func (e *Element) SetValue(v string) {
	if isSelectControl(e.node) {
		setSelectValue(e.node, v)
		return
	}
	e.SetAttribute("value", v)
	if e.doc != nil {
		e.doc.clearSelection(e.node)
	}
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

// Hidden reports whether the element's hidden attribute is present —
// mirroring the DOM's HTMLElement.hidden. See COMPATIBILITY.md: the UA
// stylesheet already maps a present hidden attribute (and
// aria-hidden="true") to display:none.
func (e *Element) Hidden() bool {
	return e.HasAttribute("hidden")
}

// SetHidden sets or clears the element's hidden attribute — mirroring the
// DOM's HTMLElement.hidden setter.
func (e *Element) SetHidden(v bool) {
	if v {
		e.SetAttribute("hidden", "")
	} else {
		e.RemoveAttribute("hidden")
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

// ChildNodes returns all of e's direct children, in document order,
// including text nodes — mirroring the DOM's Node.childNodes. Unlike the
// DOM's live NodeList, this is a snapshot slice, same as Children().
func (e *Element) ChildNodes() []*Element {
	var out []*Element
	for n := e.node.FirstChild; n != nil; n = n.NextSibling {
		out = append(out, &Element{node: n, doc: e.doc})
	}
	return out
}

// ChildElementCount returns the number of e's direct element children —
// mirroring the DOM's Element.childElementCount. Equivalent to
// len(e.Children()).
func (e *Element) ChildElementCount() int {
	return len(e.Children())
}

// QuerySelector returns the first descendant of e, in document order,
// matching sel — mirroring the DOM's Element.querySelector(). Like the
// DOM's version, e itself is never matched, only its descendants. sel
// accepts the same selector grammar as Document.QuerySelector (see CSS.md).
func (e *Element) QuerySelector(sel string) *Element {
	var found *html.Node
	walkMatchingSubtree(e.node, sel, false, func(n *html.Node) bool {
		found = n
		return false
	})
	if found == nil {
		return nil
	}
	return &Element{node: found, doc: e.doc}
}

// QuerySelectorAll returns every descendant of e, in document order,
// matching sel — mirroring the DOM's Element.querySelectorAll(). Like
// QuerySelector, e itself is never matched.
func (e *Element) QuerySelectorAll(sel string) []*Element {
	var out []*Element
	walkMatchingSubtree(e.node, sel, false, func(n *html.Node) bool {
		out = append(out, &Element{node: n, doc: e.doc})
		return true
	})
	return out
}

// GetElementsByClassName returns every descendant of e whose class
// attribute includes every token in cls (cls may itself be a
// whitespace-separated list of multiple class names) — mirroring the DOM's
// Element.getElementsByClassName().
func (e *Element) GetElementsByClassName(cls string) []*Element {
	sel := classNameSelector(cls)
	if sel == "" {
		return nil
	}
	return e.QuerySelectorAll(sel)
}

// GetElementsByTagName returns every descendant of e with the given tag
// name, in document order — mirroring the DOM's
// Element.getElementsByTagName().
func (e *Element) GetElementsByTagName(tag string) []*Element {
	return e.QuerySelectorAll(tag)
}

// Matches reports whether e matches sel — mirroring the DOM's
// Element.matches(). sel accepts the same selector grammar as CSS rules and
// Document.QuerySelector (see CSS.md), including comma-separated selector
// groups.
func (e *Element) Matches(sel string) bool {
	return cssengine.ParseSelectorGroup(sel).Match(e.node, focusAttr, "")
}

// Closest returns the nearest element in e's own inclusive ancestor chain
// (starting with e itself, then walking up through Parent) that matches
// sel, or nil if none does — mirroring the DOM's Element.closest(). Typical
// use is inside an event listener registered on a container, to find which
// specific descendant (e.g. a list row) an event's Target landed in or
// under: doc.AddEventListener(list, "click", false, func(e *Event) { row :=
// e.Target.Closest(".row"); ... }).
func (e *Element) Closest(sel string) *Element {
	group := cssengine.ParseSelectorGroup(sel)
	for n := e.node; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && group.Match(n, focusAttr, "") {
			return &Element{node: n, doc: e.doc}
		}
	}
	return nil
}

// Contains reports whether other is e itself or one of e's descendants —
// mirroring the DOM's Node.contains(). Returns false if other is nil.
func (e *Element) Contains(other *Element) bool {
	if other == nil {
		return false
	}
	return isDescendant(e.node, other.node)
}

// AppendChild adds child as e's last child — mirroring the DOM's
// Node.appendChild. child must be freshly created (e.g. via
// Document.CreateElement/CreateTextNode or Element.CloneNode) or already
// removed from wherever it was (see RemoveChild); like the underlying
// golang.org/x/net/html.Node.AppendChild, it panics if child is still
// attached anywhere in the tree.
func (e *Element) AppendChild(child *Element) {
	e.node.AppendChild(child.node)
	markRulesStale(e.doc, child.node)
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
	markRulesStale(e.doc, newChild.node)
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
	markRulesStale(e.doc, child.node)
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
	markRulesStale(e.doc, newChild.node)
	markRulesStale(e.doc, oldChild.node)
	if e.doc != nil {
		e.doc.clearFocusIfDetached()
	}
	return oldChild
}

// Remove detaches e from its parent, if it has one — mirroring the DOM's
// ChildNode.remove(). A no-op if e has no parent (e.g. the document root,
// or an already-detached element). Focus/listener handling for e's subtree
// is identical to RemoveChild.
func (e *Element) Remove() {
	p := e.Parent()
	if p == nil {
		return
	}
	p.RemoveChild(e)
}

// Before inserts newSibling immediately before e among its parent's
// children — mirroring the DOM's ChildNode.before(). A no-op if e has no
// parent. newSibling must not already be attached anywhere in the tree,
// same as AppendChild.
func (e *Element) Before(newSibling *Element) {
	p := e.Parent()
	if p == nil {
		return
	}
	p.InsertBefore(newSibling, e)
}

// After inserts newSibling immediately after e among its parent's children
// — mirroring the DOM's ChildNode.after(). A no-op if e has no parent.
// newSibling must not already be attached anywhere in the tree, same as
// AppendChild.
func (e *Element) After(newSibling *Element) {
	p := e.Parent()
	if p == nil {
		return
	}
	p.InsertBefore(newSibling, e.NextSibling())
}

// ReplaceWith replaces e with newSibling among its parent's children,
// returning e now detached — mirroring the DOM's ChildNode.replaceWith().
// A no-op returning e unchanged if e has no parent. Focus/listener handling
// for e's subtree is identical to RemoveChild/ReplaceChild.
func (e *Element) ReplaceWith(newSibling *Element) *Element {
	p := e.Parent()
	if p == nil {
		return e
	}
	return p.ReplaceChild(newSibling, e)
}

// InsertAdjacentElement inserts newEl at position relative to e — mirroring
// the DOM's Element.insertAdjacentElement(). position must be one of
// "beforebegin" (immediately before e, as a sibling), "afterbegin" (as e's
// new first child), "beforeend" (as e's new last child), or "afterend"
// (immediately after e, as a sibling); any other value returns an error and
// does nothing, matching spec's SyntaxError for an invalid position.
// "beforebegin"/"afterend" are no-ops if e has no parent, same as
// Before/After. newEl must not already be attached anywhere in the tree,
// same as AppendChild.
func (e *Element) InsertAdjacentElement(position string, newEl *Element) error {
	switch position {
	case "beforebegin":
		e.Before(newEl)
	case "afterbegin":
		e.InsertBefore(newEl, e.FirstChild())
	case "beforeend":
		e.AppendChild(newEl)
	case "afterend":
		e.After(newEl)
	default:
		return fmt.Errorf("htmlterm: invalid InsertAdjacentElement position %q", position)
	}
	return nil
}

// InsertAdjacentText inserts a new text node carrying text at position
// relative to e — mirroring the DOM's Element.insertAdjacentText(). See
// InsertAdjacentElement for the accepted position values.
func (e *Element) InsertAdjacentText(position, text string) error {
	return e.InsertAdjacentElement(position, &Element{node: &html.Node{Type: html.TextNode, Data: text}, doc: e.doc})
}

// ReplaceChildren detaches all of e's existing children and appends
// newChildren in their place, in order — mirroring the DOM's
// Element.replaceChildren(). Each of newChildren must not already be
// attached anywhere in the tree, same as AppendChild. Focus/listener
// handling for the detached children is identical to RemoveChild.
func (e *Element) ReplaceChildren(newChildren ...*Element) {
	for c := e.FirstChild(); c != nil; c = e.FirstChild() {
		e.RemoveChild(c)
	}
	for _, c := range newChildren {
		e.AppendChild(c)
	}
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

// OuterHTML serializes e, including its own tag, back to an HTML string —
// mirroring the DOM's Element.outerHTML getter. Uses golang.org/x/net/
// html's own Render, so escaping/void-element/raw-text-element (`<script>`/
// `<style>`/`<textarea>`) handling exactly matches what re-parsing the
// result would produce — see Render's own doc comment on what "best
// effort" round-tripping means for a non-well-formed tree. As with
// CloneNode, the reserved focus/select-popup state attributes are stripped
// before rendering (via the same cloneHTMLNode used by CloneNode), so
// e.g. a focused element's OuterHTML never leaks the internal
// "data-htmlterm-focus" marker. err is non-nil only if e's own subtree
// isn't well-formed enough to render at all (e.g. a void element somehow
// carrying children) — not a case this package's own mutation API can
// produce, but possible if a caller builds a malformed tree by hand.
func (e *Element) OuterHTML() (string, error) {
	var b strings.Builder
	if err := html.Render(&b, cloneHTMLNode(e.node, true)); err != nil {
		return "", err
	}
	return b.String(), nil
}

// InnerHTML serializes e's children back to an HTML string — mirroring the
// DOM's Element.innerHTML getter; see SetInnerHTML for the setter direction.
// Same rendering fidelity and reserved-attribute stripping as
// OuterHTML.
func (e *Element) InnerHTML() (string, error) {
	var b strings.Builder
	for c := e.node.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&b, cloneHTMLNode(c, true)); err != nil {
			return "", err
		}
	}
	return b.String(), nil
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

// Click synthesizes a click at e's own on-screen position (the top-left
// cell of its Rect) and runs the same hit-testing/default-action path as a
// real user click via Document.DispatchClick — mirroring the DOM's
// HTMLElement.click(). Returns false, doing nothing, if e is nil, not
// attached to a Document, or has no recorded Rect (e.g. display:none, or
// Render hasn't run yet — see Rect).
func (e *Element) Click() bool {
	if e == nil || e.doc == nil {
		return false
	}
	r, ok := e.Rect()
	if !ok {
		return false
	}
	return e.doc.DispatchClick(r.Row, r.Col, Modifiers{})
}

// DispatchEvent dispatches ev at e through the same capture/target/bubble
// walk every built-in Dispatch* method uses, mirroring EventTarget's
// dispatchEvent (implemented by both Node/Element and Document in real
// spec — Document-wide dispatch is reached here via
// doc.DocumentElement().DispatchEvent(ev), since DocumentElement() already
// returns the one Element that covers it). Skips the bubble phase entirely
// if ev.Bubbles is false. Returns false if e or ev is nil, or if e is not
// attached to a Document; false if ev is already mid-dispatch elsewhere
// (reentrancy — checked before anything else runs, so this is unconditional
// and does not depend on ev's current Cancelable/DefaultPrevented state);
// otherwise false if a listener called PreventDefault and ev.Cancelable is
// true (matching spec's dispatchEvent return value), true otherwise.
//
// There is no built-in default action for a custom event type, unlike
// DispatchClick/DispatchKey/etc. — the returned bool is the entire signal;
// it's up to the caller's own code, immediately after this call, to act on
// it (skip whatever it was about to do next), the same way application code
// built on a real dispatchEvent would. This also means dispatching a
// built-in-named type (e.g. "click") through DispatchEvent runs any
// registered listeners but no default action (checkbox toggle, submit
// forwarding, etc.) — see COMPATIBILITY.md.
func (e *Element) DispatchEvent(ev *Event) bool {
	if e == nil || e.doc == nil || ev == nil {
		return false
	}
	return e.doc.dispatchEvent(e, ev)
}

// Rect returns e's position and size as of the most recent Document.Render
// call (the CSS border box — content+padding+border, excluding margin — see
// docs/RENDERING.md's Position tracking section for the exact semantics and its
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

// ScrollTop returns e's current vertical scroll offset (in lines), and
// whether e was a scroll container (overflow:scroll|auto with a resolved
// height) as of the most recent Document.Render call — mirroring the DOM's
// element.scrollTop. ok is false if e is nil, not attached to a Document, or
// isn't a scroll container.
func (e *Element) ScrollTop() (offset int, ok bool) {
	if e == nil || e.doc == nil {
		return 0, false
	}
	return e.doc.scrollTop(e)
}

// SetScrollTop sets e's vertical scroll offset directly (e.g. to jump to
// the top of a pane, or restore a previously saved position) — mirroring the
// DOM's element.scrollTop setter. The value is clamped to the valid range on
// the next Document.Render call, the same way DispatchWheel/DispatchKey-
// driven scrolling is; a no-op if e is nil, not attached to a Document, or
// isn't (or hasn't yet been rendered as) a scroll container.
func (e *Element) SetScrollTop(offset int) {
	if e == nil || e.doc == nil {
		return
	}
	e.doc.setScrollTop(e, offset)
}

// ScrollLeft is ScrollTop's horizontal counterpart: reports e's current
// horizontal scroll offset and whether e was (as of the most recent
// Document.Render call) an overflow-x:scroll|auto container with an
// explicit width — mirroring the DOM's element.scrollLeft.
func (e *Element) ScrollLeft() (offset int, ok bool) {
	if e == nil || e.doc == nil {
		return 0, false
	}
	return e.doc.scrollLeft(e)
}

// SetScrollLeft is SetScrollTop's horizontal counterpart: sets e's
// horizontal scroll offset directly — mirroring the DOM's element.scrollLeft
// setter. The value is clamped to the valid range on the next Document.Render
// call, the same way DispatchWheel/DispatchKey-driven scrolling is; a no-op
// if e is nil, not attached to a Document, or isn't (or hasn't yet been
// rendered as) a horizontal scroll container.
func (e *Element) SetScrollLeft(offset int) {
	if e == nil || e.doc == nil {
		return
	}
	e.doc.setScrollLeft(e, offset)
}

// ScrollVisible reports whether e's Rect, as of the most recent
// Document.Render call, currently falls at least partly within the visible
// content range of every scrollable ancestor it has — the read side of what
// the DOM's Element.scrollIntoView() would otherwise trigger (there's no
// scripting engine here to fire that as a side-effecting call; this is the
// visibility check a host needs instead — e.g. Loop's terminal cursor
// placement, via focusCursorPos, so it can tell whether a focused control's
// position is actually on-screen right now rather than off-screen inside a
// container that has since scrolled past it). True for an element with no
// scrollable ancestor, e is nil, e isn't attached to a Document, or before
// the first Render. Checks both scroll axes: an element off-screen on either
// the vertical or horizontal range of any scrollable ancestor is not
// visible.
func (e *Element) ScrollVisible() bool {
	if e == nil || e.doc == nil {
		return true
	}
	return e.doc.scrollVisible(e)
}

// SetInnerHTML parses htmlStr as an HTML fragment (parsed in e's own
// context, the same rule ParseFragment uses — e.g. a fragment containing
// bare <tr>s needs e to itself be a <table>/<tbody> for the fragment parser
// to accept them) and replaces e's children with the result, discarding e's
// previous children entirely — mirroring the DOM's Element.innerHTML setter.
// This is the mechanism for injecting structural, host-controlled content —
// a freshly-fetched envelope table, a rendered email body — into a container
// that a Loop is actively driving, without needing to replace the Document
// itself (Loop holds one Document pointer for its whole run; there is no
// document-swap API, so a container declared once up front and refreshed via
// SetInnerHTML is the supported pattern for content that changes shape, as
// opposed to attribute-driven mutation for content that doesn't — see
// docs/INTERACTIVE.md's ImportHTML note, which this supersedes).
//
// A <style> element in the fragment (or removed along with e's previous
// children) does take effect: it marks Document's cachedRules stale, so the
// next Render recomputes the resolved stylesheet rule set rather than
// reusing a snapshot from before the replacement. That said, page-level CSS
// still belongs in Options.CSS/Stylesheets, set once at ParseDocument time —
// SetInnerHTML is meant for markup, not styling; a <style>-bearing fragment
// is supported, not recommended.
//
// If the currently focused element is inside the replaced subtree, focus is
// silently cleared (no "blur" dispatched — the element is gone, not
// blurred) rather than left dangling on a detached node. Any event listeners
// registered on now-detached descendants become unreachable (the same
// listener-leak behavior a real DOM has when you drop a subtree without
// removeEventListener) — call Document.RemoveEventListener first if that
// matters. Returns an error if e is nil or not attached to a Document.
func (e *Element) SetInnerHTML(htmlStr string) error {
	if e == nil || e.doc == nil {
		return fmt.Errorf("htmlterm: SetInnerHTML on nil element")
	}
	return e.doc.setInnerHTML(e, htmlStr)
}

// ClassList returns a handle for reading and mutating the element's class
// attribute as a set of whitespace-separated tokens.
func (e *Element) ClassList() *ClassList {
	return &ClassList{el: e}
}

// Dataset returns a handle for reading and mutating the element's data-*
// attributes — mirroring the DOM's HTMLElement.dataset.
func (e *Element) Dataset() *Dataset {
	return &Dataset{el: e}
}

// IsTextEntry reports whether e is a <textarea> or a text-like <input>
// (any type other than checkbox/radio/submit/button/reset/hidden) — the
// elements DispatchKey's printable-character and Backspace default actions
// act on, and the set a host's own cursor-placement logic (e.g. tui's
// focusCursorPos) should treat as having a text insertion point.
func (e *Element) IsTextEntry() bool {
	return isTextEntry(e.node)
}

// SelectionStart returns the rune offset of the start of e's current
// selection into its value — mirroring the DOM's
// HTMLInputElement.selectionStart/HTMLTextAreaElement.selectionStart.
// Defaults to the end of the value (a collapsed caret) if
// SetSelectionRange has never been called, matching this package's
// pre-existing append-at-end behavior. Returns 0 if e is nil or not
// attached to a Document.
func (e *Element) SelectionStart() int {
	if e == nil || e.doc == nil {
		return 0
	}
	return e.doc.selection(e.node).start
}

// SelectionEnd is SelectionStart's counterpart — mirroring
// HTMLInputElement.selectionEnd/HTMLTextAreaElement.selectionEnd. For a
// collapsed selection (no range), SelectionStart and SelectionEnd are
// equal — that shared value is also where a caret indicator should be
// drawn, matching real UAs, which show the blinking caret at
// selectionEnd even when a range is selected.
func (e *Element) SelectionEnd() int {
	if e == nil || e.doc == nil {
		return 0
	}
	return e.doc.selection(e.node).end
}

// SelectionDirection returns "forward", "backward", or "none" — mirroring
// HTMLInputElement.selectionDirection/HTMLTextAreaElement.selectionDirection.
// Returns "none" if e is nil or not attached to a Document.
func (e *Element) SelectionDirection() string {
	if e == nil || e.doc == nil {
		return "none"
	}
	return e.doc.selection(e.node).direction
}

// SetSelectionRange sets e's caret/selection to [start, end) (rune offsets
// into its value, clamped to the value's length and reordered if
// start > end) — mirroring the DOM's
// HTMLInputElement.setSelectionRange(start, end, direction). direction is
// optional, like the real method's third argument; passing more than one
// value uses only the first, and an omitted or unrecognized direction
// normalizes to "none". A no-op if e is nil or not attached to a Document.
func (e *Element) SetSelectionRange(start, end int, direction ...string) {
	if e == nil || e.doc == nil {
		return
	}
	dir := "none"
	if len(direction) > 0 {
		dir = direction[0]
	}
	e.doc.setSelection(e.node, start, end, dir)
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

// Dataset is a DOM-like handle onto an element's data-* attributes —
// mirroring the DOM's HTMLElement.dataset (a DOMStringMap). Unlike the
// DOM's live property bag, each method reads/writes the backing data-*
// attribute directly through the *Element rather than caching anything.
type Dataset struct {
	el *Element
}

// datasetAttrName converts a camelCase dataset key (e.g. "fooBar", matching
// how the DOM's dataset itself is addressed, e.g. dataset.fooBar) to its
// backing attribute name ("data-foo-bar") — mirroring the DOM's dataset
// name-mapping algorithm (HTML spec's "dataset DOMStringMap").
func datasetAttrName(name string) string {
	var b strings.Builder
	b.WriteString("data-")
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('-')
			b.WriteByte(c + ('a' - 'A'))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Get returns the value of the named dataset entry, and whether it is
// present — mirroring reading a property off the DOM's dataset, except
// returning an explicit ok bool instead of undefined. name is a camelCase
// dataset key, not the raw data-* attribute suffix — see datasetAttrName.
func (d *Dataset) Get(name string) (string, bool) {
	return d.el.GetAttribute(datasetAttrName(name))
}

// Set sets the named dataset entry to value, adding the backing data-*
// attribute if not already present — mirroring assigning a property on the
// DOM's dataset.
func (d *Dataset) Set(name, value string) {
	d.el.SetAttribute(datasetAttrName(name), value)
}

// Has reports whether the named dataset entry is present — mirroring an
// `in` check against the DOM's dataset.
func (d *Dataset) Has(name string) bool {
	return d.el.HasAttribute(datasetAttrName(name))
}

// Delete removes the named dataset entry, if present — mirroring `delete`
// on a property of the DOM's dataset.
func (d *Dataset) Delete(name string) {
	d.el.RemoveAttribute(datasetAttrName(name))
}

// Style is a DOM-like handle onto an element's inline "style" attribute —
// mirroring the DOM's CSSStyleDeclaration for inline styles only. There is
// no computed-style equivalent (getComputedStyle); see docs/DOM_API.md.
//
// Two real-DOM behaviors this deliberately doesn't replicate, both stemming
// from internal/cssengine.ParseDeclarations expanding shorthand properties
// into longhands at parse time and never recording that a shorthand was
// used (the cascade itself has the same limitation — nothing in this engine
// tracks "this element's margin came from one shorthand declaration"):
//
//   - Shorthands don't round-trip: SetProperty("margin", "1px") is stored as
//     margin-top/-right/-bottom/-left, so GetPropertyValue("margin")
//     afterward returns "", not "1px" — query the longhand instead.
//   - CSSText/SetCSSText don't preserve declaration order — CSSText
//     serializes properties sorted by name, not in the order they were set
//     or appeared in the original style="" text.
type Style struct {
	el *Element
}

// Style returns a handle for reading and mutating e's inline "style"
// attribute — mirroring the DOM's HTMLElement.style.
func (e *Element) Style() *Style {
	return &Style{el: e}
}

// GetPropertyValue returns the value of the named property in the element's
// inline style, or "" if not set — mirroring
// CSSStyleDeclaration.getPropertyValue. name is lowercased before lookup
// unless it's a custom property ("--name"). Shorthand properties are never
// present in isolation — see the Style doc comment.
func (s *Style) GetPropertyValue(name string) string {
	decls := cssengine.ParseDeclarations(nodeAttr(s.el.node, "style"))
	return decls[normalizeStyleProp(name)]
}

// SetProperty sets the named property to value in the element's inline
// style, adding it if not already present, or removing it if value is ""
// (matching CSSStyleDeclaration.setProperty's own empty-value behavior) —
// mirroring CSSStyleDeclaration.setProperty. A shorthand value is expanded
// into its longhand properties the same way a literal style="" attribute
// would be; the shorthand name itself is not stored — see the Style doc
// comment.
func (s *Style) SetProperty(name, value string) {
	if value == "" {
		s.RemoveProperty(name)
		return
	}
	cur := nodeAttr(s.el.node, "style")
	var b strings.Builder
	b.WriteString(cur)
	if b.Len() > 0 {
		b.WriteString(";")
	}
	b.WriteString(name)
	b.WriteString(":")
	b.WriteString(value)
	decls := cssengine.ParseDeclarations(b.String())
	s.el.SetAttribute("style", serializeStyleDecls(decls))
}

// RemoveProperty removes the named property from the element's inline
// style, if present, and returns its former value — mirroring
// CSSStyleDeclaration.removeProperty. A no-op returning "" if name isn't a
// stored longhand — including, per the Style doc comment, a shorthand name
// that was never itself stored.
func (s *Style) RemoveProperty(name string) string {
	decls := cssengine.ParseDeclarations(nodeAttr(s.el.node, "style"))
	key := normalizeStyleProp(name)
	old := decls[key]
	delete(decls, key)
	s.el.SetAttribute("style", serializeStyleDecls(decls))
	return old
}

// CSSText returns the element's inline style serialized back to
// "prop: value; ..." text, with properties sorted by name — mirroring
// CSSStyleDeclaration.cssText's getter, except without preserving
// declaration order (see the Style doc comment).
func (s *Style) CSSText() string {
	return serializeStyleDecls(cssengine.ParseDeclarations(nodeAttr(s.el.node, "style")))
}

// SetCSSText replaces the element's entire inline style with text —
// mirroring CSSStyleDeclaration.cssText's setter. text is stored verbatim
// rather than being re-parsed/re-serialized immediately; the next
// GetPropertyValue/CSSText call parses it like any other style="" value.
func (s *Style) SetCSSText(text string) {
	s.el.SetAttribute("style", text)
}

// normalizeStyleProp lowercases name for lookup/storage, except custom
// properties ("--name"), which are case-sensitive — matching
// cssengine.ParseDeclarations' own normalization.
func normalizeStyleProp(name string) string {
	if strings.HasPrefix(name, "--") {
		return name
	}
	return strings.ToLower(name)
}

// serializeStyleDecls renders decls back to "prop: value; ..." text, sorted
// by property name for deterministic output — see the Style doc comment on
// why declaration order isn't preserved.
func serializeStyleDecls(decls map[string]string) string {
	keys := make([]string, 0, len(decls))
	for k := range decls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(decls[k])
		b.WriteString(";")
	}
	return b.String()
}
