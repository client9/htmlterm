package document

import (
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
	"golang.org/x/net/html"
)

// dialogReturnAttr is the reserved marker attribute (see focusAttr) a
// <dialog> carries to hold its return value, the string close() was given
// and returnValue reports back. Real DOM keeps returnValue as an IDL
// property with no reflecting content attribute at all. This package has no
// separate property layer to put it in (see COMPATIBILITY.md's "Form
// controls are attribute-driven" entry), so it lives in an attribute like
// every other piece of control state here, and is stripped by cloneHTMLNode
// so it never reaches OuterHTML or CloneNode. Unlike selectOpenAttr it is
// never read by the renderer, so it needs no renderOptions threading.
const dialogReturnAttr = "data-htmlterm-dialog-return"

// dialogModalAttr is the reserved marker attribute (see focusAttr) a <dialog>
// carries while it is open as a modal, meaning opened by ShowModal rather
// than Show. It is threaded through via renderOptions' DialogModalAttr the
// same way focusAttr is, and reaches layout only through the cascade, as the
// attribute ":modal" matches. That is what makes the UA stylesheet's own
// `dialog:modal` rule apply, giving a modal dialog the position, z-index, and
// centering that put it in the top layer, with no dedicated compositing code
// of the kind <select>'s popup needs. Rendering reads it directly in exactly
// one place, to decide whether to paint a ::backdrop.
//
// It is also this package's own source of truth for "is this dialog modal",
// read by the focus trap and the click-containment check. See docs/DIALOG.md.
const dialogModalAttr = "data-htmlterm-dialog-modal"

// selectorMarkers is the marker set every selector match in this package
// runs with. Focus and Modal are this package's own reserved attributes, so
// :focus and :modal both work in QuerySelector, Matches, and Closest. Hover
// is deliberately empty: it means "the arrow-key-highlighted <option> in an
// open <select> popup", which is rendering state rather than something a
// caller querying the DOM should be able to select on.
func selectorMarkers() cssengine.Markers {
	return cssengine.Markers{Focus: focusAttr, Modal: dialogModalAttr}
}

// isDialog reports whether n is a <dialog> element.
func isDialog(n *html.Node) bool {
	return n != nil && n.Type == html.ElementNode && strings.EqualFold(n.Data, "dialog")
}

// nearestDialog returns n's nearest ancestor-or-self <dialog>, or nil if it
// has none. Shared by the form method="dialog" default action and, once a
// modal is open, the focus and click containment checks.
func nearestDialog(n *html.Node) *html.Node {
	for p := n; p != nil; p = p.Parent {
		if isDialog(p) {
			return p
		}
	}
	return nil
}

// Show opens e as a non-modal dialog, setting its open attribute, mirroring
// HTMLDialogElement.show(). It fires no event, matching spec: only close()
// and the Escape-key cancel path do. Returns false, making no change, if e
// isn't an attached <dialog>, or is already open.
//
// A non-modal dialog is an ordinary in-flow block box. Nothing about it is
// special-cased in Go: the UA stylesheet's dialog/dialog:not([open]) pair
// does the whole job, the same way <details> is styled rather than coded.
// See ShowModal for the modal counterpart, which is a different layout mode
// entirely.
func (e *Element) Show() bool {
	if e == nil || e.doc == nil || !isDialog(e.node) || nodeHasAttr(e.node, "open") {
		return false
	}
	setAttr(e.node, "open", "")
	return true
}

// ShowModal opens e as a modal dialog, mirroring
// HTMLDialogElement.showModal(). It sets the open attribute and the reserved
// modal marker, then moves focus to the first focusable element inside the
// dialog, or leaves focus alone if it has none. Returns false, making no
// change, if e isn't an attached <dialog>, or is already open.
//
// A modal differs from Show's non-modal dialog in three ways, all of which
// follow from the marker rather than from dedicated code here. The UA
// stylesheet's `dialog:modal` rule gives it position:fixed, a maximal
// z-index, and auto margins, so it leaves normal flow, centers itself, and
// paints above everything. Everything outside it stops being focusable (see
// hiddenByModal) and stops receiving clicks. And Escape closes it, firing
// "cancel" first (see DispatchKey).
//
// Nesting works: a second ShowModal while one is already open makes the new
// dialog the topmost, and closing it hands control back to the one below.
func (e *Element) ShowModal() bool {
	if e == nil || e.doc == nil || !isDialog(e.node) || nodeHasAttr(e.node, "open") {
		return false
	}
	setAttr(e.node, "open", "")
	// One past whatever is already topmost, so nesting orders by ShowModal
	// call order rather than by document order. See modalDepth.
	depth := 1
	if top := e.doc.topmostModal(); top != nil {
		depth = modalDepth(top) + 1
	}
	setAttr(e.node, dialogModalAttr, strconv.Itoa(depth))
	e.doc.openModals++
	e.doc.modalCache = nil
	// Remembered before focus moves inside, so closing can put it back. See
	// restoreFocusAfterClose.
	if e.doc.focused != nil {
		if e.doc.modalPrevFocus == nil {
			e.doc.modalPrevFocus = map[*html.Node]*html.Node{}
		}
		e.doc.modalPrevFocus[e.node] = e.doc.focused
	}
	e.doc.focusFirstInModal(e.node)
	return true
}

// Close closes e with an empty return value, mirroring
// HTMLDialogElement.close() called with no argument. See CloseWith.
func (e *Element) Close() bool {
	return e.CloseWith("")
}

// CloseWith closes e and sets its return value to rv, mirroring
// HTMLDialogElement.close(returnValue). It clears the open attribute and
// dispatches a non-bubbling, non-cancelable "close" event. Returns false,
// making no change and firing nothing, if e isn't an attached <dialog>, or
// is already closed, matching real DOM, where close() on a closed dialog
// does nothing.
//
// Go has no optional arguments, so the spec's one overloaded method is two
// here, the same split Element's other value-taking setters use.
func (e *Element) CloseWith(rv string) bool {
	if e == nil || e.doc == nil || !isDialog(e.node) || !nodeHasAttr(e.node, "open") {
		return false
	}
	e.doc.closeDialog(e.node, rv)
	return true
}

// ReturnValue reports e's return value, the string last passed to CloseWith
// or SetReturnValue, mirroring HTMLDialogElement.returnValue. Empty for a
// dialog that has never been closed with one.
func (e *Element) ReturnValue() string {
	if e == nil {
		return ""
	}
	return nodeAttr(e.node, dialogReturnAttr)
}

// SetReturnValue sets e's return value without closing it, mirroring
// assignment to HTMLDialogElement.returnValue. CloseWith sets it too, which
// is the usual way it gets one.
func (e *Element) SetReturnValue(rv string) {
	if e == nil || !isDialog(e.node) {
		return
	}
	setAttr(e.node, dialogReturnAttr, rv)
}

// closeDialog is the shared close path behind CloseWith, the Escape-key
// cancel default action, and a method="dialog" form submission: record the
// return value, drop the open and modal attributes, then dispatch "close".
// The event fires last so a listener reading Open() or ReturnValue() sees the
// settled state, matching real DOM's own ordering.
func (d *Document) closeDialog(dialog *html.Node, rv string) {
	setAttr(dialog, dialogReturnAttr, rv)
	removeAttr(dialog, "open")
	if nodeHasAttr(dialog, dialogModalAttr) {
		removeAttr(dialog, dialogModalAttr)
		d.openModals = max(0, d.openModals-1)
	}
	d.modalCache = nil
	d.restoreFocusAfterClose(dialog)
	d.dispatch(dialog, "close", "", Modifiers{})
}

// restoreFocusAfterClose moves focus off a dialog that has just closed. The
// dialog is display:none from this point on, so leaving focus inside it would
// keep delivering every keystroke to an element rendering nothing, with the
// terminal cursor and :focus styling nowhere to put themselves.
//
// Focus goes back to whatever held it when ShowModal ran, mirroring real
// DOM's own "previously focused element" restore, provided that element is
// still focusable: it may have been removed, disabled, or hidden while the
// dialog was up, and it is unreachable anyway if an outer modal is still
// open. Failing that, focus is simply dropped. Focus that was already
// outside the dialog is left alone.
//
// Called before "close" dispatches, so a listener sees settled focus along
// with the settled open state and return value.
func (d *Document) restoreFocusAfterClose(dialog *html.Node) {
	prev, hadPrev := d.modalPrevFocus[dialog]
	delete(d.modalPrevFocus, dialog)

	if d.focused == nil || !isDescendant(dialog, d.focused) {
		return
	}
	if hadPrev && prev != nil && isDescendant(d.doc, prev) && d.isFocusable(prev) {
		d.focus(&Element{node: prev, doc: d})
		return
	}
	d.blur()
}

// modalDepth reports n's position in the top layer, the value its modal
// marker attribute carries, or 0 if it isn't an open modal at all. Depths
// start at 1, assigned in ShowModal call order, so a higher number is nearer
// the top.
//
// The ordering lives in the attribute rather than in a stack field on
// Document deliberately. The tree is then the only source of truth: a modal
// detached from the document, or force-closed by writing its attributes
// directly, simply stops being found by topmostModal's walk, with no separate
// bookkeeping to prune or resync. It is the same reason selectOpenAttr holds
// <select> popup state on the node rather than beside it.
func modalDepth(n *html.Node) int {
	if !nodeHasAttr(n, "open") {
		return 0
	}
	v := nodeAttr(n, dialogModalAttr)
	if v == "" {
		return 0
	}
	depth, err := strconv.Atoi(v)
	if err != nil || depth < 1 {
		return 0
	}
	return depth
}

// topmostModal returns the open modal <dialog> nearest the top of the top
// layer, or nil if none is open. That is the one, and the only one, that
// currently holds focus and input: see hiddenByModal and DispatchClick.
//
// Two fast paths guard the walk, because isFocusable calls this for every
// candidate node and focusableList calls isFocusable for every node in the
// document, so a walk on each would make one Tab press quadratic in document
// size. openModals skips it entirely for the overwhelmingly common case of no
// modal at all. modalCache skips it when the answer is already known.
//
// Neither is trusted blindly. ShowModal and closeDialog clear the cache, so
// it can't outlive a change in which modal is topmost, and a cached node is
// re-checked against the tree before it is returned, since it can still be
// detached or force-closed by writing its attributes directly. Either check
// failing falls back to the walk, which remains the only thing that decides
// which modal is topmost.
func (d *Document) topmostModal() *html.Node {
	if d.openModals <= 0 {
		return nil
	}
	if n := d.modalCache; n != nil && modalDepth(n) > 0 && isDescendant(d.doc, n) {
		return n
	}
	var best *html.Node
	bestDepth := 0
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if isDialog(n) {
			if depth := modalDepth(n); depth > bestDepth {
				best, bestDepth = n, depth
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
	d.modalCache = best
	return best
}

// hiddenByClosedDialog reports whether n sits inside a <dialog> that has no
// open attribute. The UA stylesheet's `dialog:not([open]) { display: none }`
// rule makes that whole subtree render nothing, so nothing in it can be
// focused, exactly as for content hidden inside a closed <details> (see
// hiddenByClosedDetails, which this mirrors). Without this, Tab would stop on
// an invisible control inside an unopened dialog.
//
// Unlike the <details> version there is no exempt child to walk past: a
// closed dialog hides itself entirely, summary and all.
func hiddenByClosedDialog(n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if isDialog(p) && !nodeHasAttr(p, "open") {
			return true
		}
	}
	return false
}

// hiddenByModal reports whether n is outside the topmost open modal
// <dialog>, and so inert: not focusable, and not clickable. It is the
// terminal counterpart of a browser making the rest of the document inert
// while a modal is open.
//
// isFocusable checks this first, exactly where it checks hiddenByClosedDetails
// (see that function), so one hook covers Tab order, Element.Focus, and
// click-to-focus together. With no modal open it is false for everything, so
// a document that never calls ShowModal is unaffected.
func (d *Document) hiddenByModal(n *html.Node) bool {
	modal := d.topmostModal()
	if modal == nil {
		return false
	}
	for p := n; p != nil; p = p.Parent {
		if p == modal {
			return false
		}
	}
	return true
}

// focusFirstInModal moves focus into a dialog that has just been opened as a
// modal, mirroring showModal()'s own focusing steps.
//
// It needs no subtree filtering of its own: the modal marker is already set
// by the time this runs, so hiddenByModal has made everything outside the
// dialog unfocusable and focusableList's ordinary whole-document walk yields
// only candidates inside it. When there are none, focus is dropped rather
// than left where it was, so it can't sit on an element the user can no
// longer see or reach. Real DOM focuses the dialog itself in that case; a
// <dialog> isn't focusable here without a tabindex, and leaving focus outside
// the modal would put a hole in the trap.
func (d *Document) focusFirstInModal(dialog *html.Node) {
	if list := d.focusableList(); len(list) > 0 {
		d.focus(&Element{node: list[0], doc: d})
		return
	}
	if d.focused != nil {
		d.blur()
	}
}

// dismissTopmostModal runs the Escape-key default action for an open modal:
// dispatch a cancelable "cancel", and unless a listener prevents it, close
// the dialog with an empty return value. Mirrors HTML's own
// "request to close" steps. Reports whether a modal was open to act on.
func (d *Document) dismissTopmostModal() bool {
	modal := d.topmostModal()
	if modal == nil {
		return false
	}
	if ev := d.dispatch(modal, "cancel", "", Modifiers{}); ev.DefaultPrevented() {
		return true
	}
	d.closeDialog(modal, "")
	return true
}

// isDialogMethodForm reports whether form is a <form method="dialog">, the
// markup-only idiom for closing a dialog with a return value and no
// scripting. Real HTML treats the method attribute case-insensitively.
func isDialogMethodForm(form *html.Node) bool {
	return strings.EqualFold(nodeAttr(form, "method"), "dialog")
}

// triggerSubmit dispatches "submit" on form and then runs the one default
// action a submission has here: a <form method="dialog"> inside a <dialog>
// closes that dialog, taking the submitter control's own value as the return
// value, per HTML's form-submission algorithm. Every other form is left
// alone, since this package never navigates or sends a request (see
// COMPATIBILITY.md); dispatching the event is the whole behavior there.
//
// Shared by DispatchClick's submit-control default action and DispatchKey's
// Enter, so the two stay in step. The "submit" event is cancelable, and a
// listener that prevents it suppresses the close, matching a browser.
func (d *Document) triggerSubmit(form, submitter *html.Node) {
	ev := d.dispatch(form, "submit", "", Modifiers{})
	if ev.DefaultPrevented() || !isDialogMethodForm(form) {
		return
	}
	dialog := nearestDialog(form)
	if dialog == nil || !nodeHasAttr(dialog, "open") {
		return
	}
	rv := ""
	if submitter != nil {
		rv = nodeAttr(submitter, "value")
	}
	d.closeDialog(dialog, rv)
}
