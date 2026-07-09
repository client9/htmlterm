package document

import "golang.org/x/net/html"

// focusAttr is the reserved marker attribute Document.Focus/Blur set and
// clear to record the currently focused node, and that matchPseudo checks
// for ":focus" — see INTERACTIVE.md's "Focus manager + live pseudo-classes"
// section. Namespaced beyond plain "data-*" so it can't collide with a
// host's own data attributes. Future :hover/:active support (not part of
// this phase — see INTERACTIVE.md's events section) would follow the same
// pattern with sibling attribute names, e.g. "data-htmlterm-hover".
const focusAttr = "data-htmlterm-focus"

// listenerID identifies one registered listener within a Document, for
// RemoveEventListener — Go func values aren't comparable, so a listener
// can't be removed by passing the same func back.
type listenerID uint64

type listenerEntry struct {
	id      listenerID
	typ     string
	capture bool
	fn      func(*Event)
}

// ListenerHandle identifies a listener previously registered via
// Document.AddEventListener, for later removal via RemoveEventListener.
type ListenerHandle struct {
	node *html.Node
	id   listenerID
}

// Event is passed to listeners registered via Document.AddEventListener,
// modeled on the DOM Event interface's capture/target/bubble phases and
// preventDefault/stopPropagation controls — see INTERACTIVE.md's "Next:
// events" section.
type Event struct {
	// Type is the event name, e.g. "click", "keydown", "focus", "blur".
	Type string
	// Target is the element the event was dispatched at (the hit-tested or
	// focused element) — constant throughout all three phases.
	Target *Element
	// Key is set for "keydown" events: either a single printable rune as a
	// UTF-8 string, or a named key such as "Enter", "Backspace", "Tab".
	Key string

	current          *html.Node
	stopped          bool
	stoppedImmediate bool
	defaultPrevented bool
}

// CurrentTarget returns the element whose listener is currently running —
// during capture/bubble phases this differs from Target, which stays fixed.
func (e *Event) CurrentTarget() *Element {
	return &Element{node: e.current}
}

// StopPropagation prevents the event from reaching any further ancestors
// (or descendants, during capture) after the current phase's remaining
// listeners on this node have run.
func (e *Event) StopPropagation() {
	e.stopped = true
}

// StopImmediatePropagation is like StopPropagation but also skips any
// remaining listeners on the current node.
func (e *Event) StopImmediatePropagation() {
	e.stopped = true
	e.stoppedImmediate = true
}

// PreventDefault suppresses the event's built-in default action (e.g. a
// checkbox toggling its checked state on click).
func (e *Event) PreventDefault() {
	e.defaultPrevented = true
}

// DefaultPrevented reports whether PreventDefault was called.
func (e *Event) DefaultPrevented() bool {
	return e.defaultPrevented
}

// AddEventListener registers fn to run when an event of type typ is
// dispatched to el, during the capture phase if capture is true, otherwise
// during the bubble phase (the target phase runs every listener on the
// target node regardless of its capture flag). Listeners are stored on the
// Document, keyed by el's underlying node — not on Element itself, which is
// a throwaway handle recreated on every lookup (see element.go) — so a
// listener persists across separately-obtained Elements for the same node.
func (d *Document) AddEventListener(el *Element, typ string, capture bool, fn func(*Event)) ListenerHandle {
	d.nextListenerID++
	id := d.nextListenerID
	if d.listeners == nil {
		d.listeners = make(map[*html.Node][]listenerEntry)
	}
	d.listeners[el.node] = append(d.listeners[el.node], listenerEntry{
		id:      id,
		typ:     typ,
		capture: capture,
		fn:      fn,
	})
	return ListenerHandle{node: el.node, id: id}
}

// RemoveEventListener removes a listener previously registered via
// AddEventListener. It is a no-op if the listener was already removed.
func (d *Document) RemoveEventListener(h ListenerHandle) {
	entries := d.listeners[h.node]
	for i, e := range entries {
		if e.id == h.id {
			d.listeners[h.node] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

// ancestorChain returns the path from the document root down to n
// (inclusive), for capture/bubble traversal.
func ancestorChain(n *html.Node) []*html.Node {
	var chain []*html.Node
	for cur := n; cur != nil; cur = cur.Parent {
		chain = append(chain, cur)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// dispatch runs typ-listeners registered against target and its ancestors,
// in capture order (root toward target), then target-phase order, then
// bubble order (target toward root), honoring StopPropagation/
// StopImmediatePropagation. key is copied into the returned Event's Key
// field (used for "keydown").
func (d *Document) dispatch(target *html.Node, typ, key string) *Event {
	ev := &Event{Type: typ, Target: &Element{node: target}, Key: key}
	chain := ancestorChain(target)

	runNode := func(n *html.Node, wantCapture *bool) bool {
		ev.current = n
		for _, e := range d.listeners[n] {
			if e.typ != typ {
				continue
			}
			if wantCapture != nil && e.capture != *wantCapture {
				continue
			}
			e.fn(ev)
			if ev.stoppedImmediate {
				return false
			}
		}
		return true
	}

	capture := true
	for _, n := range chain[:len(chain)-1] {
		if !runNode(n, &capture) || ev.stopped {
			return ev
		}
	}

	if !runNode(target, nil) || ev.stopped {
		return ev
	}

	bubble := false
	for i := len(chain) - 2; i >= 0; i-- {
		if !runNode(chain[i], &bubble) || ev.stopped {
			return ev
		}
	}

	return ev
}
