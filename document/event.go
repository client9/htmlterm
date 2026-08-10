package document

import (
	"slices"

	"golang.org/x/net/html"
)

// focusAttr is the reserved marker attribute Document.Focus and Blur set and
// clear to record the currently focused node, and that matchPseudo checks
// for ":focus". See docs/INTERACTIVE.md's "Focus manager + live
// pseudo-classes" section. It is namespaced beyond plain "data-*" so it can't
// collide with a host's own data attributes. Future :hover and :active
// support, not part of this phase (see docs/INTERACTIVE.md's events section),
// would follow the same pattern with sibling attribute names such as
// "data-htmlterm-hover".
const focusAttr = "data-htmlterm-focus"

// selectionStartAttr and selectionEndAttr are the reserved marker attributes
// (see focusAttr) Document.setSelection sets and clears together on a text
// entry, mirroring its current d.selections state onto the tree itself.
// internal/render has no access to Document's own state, so this is how it
// learns which [start, end) rune range, if any, to render under the
// ::selection highlight (see internal/render/formcontrol.go's selectionRange
// and docs/proposals/CARET_SELECTION.md). Both are cleared together whenever
// the selection collapses, so their presence alone already means "there is an
// active range" without reading the values first.
const (
	selectionStartAttr = "data-htmlterm-selection-start"
	selectionEndAttr   = "data-htmlterm-selection-end"
)

// listenerID identifies one registered listener within a Document, for
// RemoveEventListener. Go func values aren't comparable, so a listener
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

// Modifiers records which keyboard modifier keys were held during an input
// event, mirroring real DOM events' ctrlKey, shiftKey, altKey, and metaKey
// booleans. The host, such as tui.Loop, owns translating raw terminal
// modifier reporting into this struct. htmlterm never reads a terminal
// directly outside of tui.Loop.
type Modifiers struct {
	Shift bool
	Ctrl  bool
	Alt   bool
	Meta  bool
}

// Event is passed to listeners registered via Document.AddEventListener,
// modeled on the DOM Event interface's capture, target, and bubble phases and
// its preventDefault and stopPropagation controls. See docs/INTERACTIVE.md's
// "Next: events" section.
type Event struct {
	// Type is the event name, such as "click", "keydown", "focus", or "blur".
	Type string
	// Target is the element the event was dispatched at, meaning the
	// hit-tested or focused element. It is constant throughout all three
	// phases.
	Target *Element
	// Key is set for "keydown", "keyup", and "input" events: either a single
	// printable rune as a UTF-8 string, or a named key such as "Enter",
	// "Backspace", "Tab".
	Key string
	// ShiftKey, CtrlKey, AltKey, MetaKey mirror the modifier keys held when
	// the event was dispatched, set from the Modifiers passed to
	// DispatchClick and DispatchKey. They are false for event types that
	// don't carry one, such as "resize", "submit", "focus", "blur", and
	// "change", matching real DOM's UIEvent-only modifier fields.
	ShiftKey bool
	CtrlKey  bool
	AltKey   bool
	MetaKey  bool
	// ClipboardData carries the event's plain-text clipboard payload,
	// mirroring DOM's ClipboardEvent.clipboardData, simplified to a single
	// string with no DataTransfer or MIME-type machinery, matching this
	// package's existing text-only value model. It is set for "paste", the
	// text being pasted in, supplied by the host (see DispatchPaste), and for
	// "cut", the text about to be removed from the focused field,
	// pre-populated before dispatch. A listener may read or overwrite it
	// exactly like calling clipboardData.getData or setData in a real
	// handler, and the possibly rewritten value is what each event's default
	// action then acts on. Empty for every other event type.
	ClipboardData string
	// Detail is an arbitrary caller-defined payload, mirroring
	// CustomEvent.detail. htmlterm never inspects it. It is nil for every
	// built-in event type.
	Detail any
	// Bubbles reports whether this event's dispatch runs the bubble phase
	// after target, mirroring Event.bubbles. For a built-in event type, set
	// by defaultBubbles (see newEvent): true for every type except
	// "toggle". Real spec's own per-type table isn't uniform either, since
	// focus, blur, and resize don't bubble there, but defaultBubbles only
	// special-cases "toggle" today, so focus/blur/resize still bubble here,
	// unlike spec (see COMPATIBILITY.md). For custom events Bubbles is set
	// via CustomEventInit.Bubbles, defaulting to false per spec.
	Bubbles bool
	// Cancelable reports whether PreventDefault has any effect on this
	// event, mirroring Event.cancelable. Set per type for built-in events
	// (see defaultCancelable) to match which Dispatch* methods already gate
	// their default action on DefaultPrevented(). For custom events, set
	// via CustomEventInit.Cancelable, defaulting to false per spec.
	Cancelable bool

	doc              *Document
	current          *html.Node
	stopped          bool
	stoppedImmediate bool
	defaultPrevented bool
	dispatching      bool
}

// CustomEventInit configures NewCustomEvent, mirroring the DOM's
// CustomEventInit dictionary, the object literal passed as
// `new CustomEvent(type, init)`'s second argument. It is named to match spec
// rather than Go's usual "Options" convention, consistent with this
// package's existing practice of borrowing DOM vocabulary where it aids
// readability, as Modifiers, ClipboardData, and ListenerHandle do.
type CustomEventInit struct {
	// Detail is an arbitrary caller-defined payload, mirroring
	// CustomEvent.detail. htmlterm never inspects it.
	Detail any
	// Bubbles controls whether DispatchEvent runs the bubble phase after
	// target, mirroring CustomEventInit.bubbles. Defaults to false (Go's
	// zero value), matching spec's own default.
	Bubbles bool
	// Cancelable controls DispatchEvent's return value, and whether
	// PreventDefault has any effect at all. The return is false only if both
	// Cancelable is true and a listener called PreventDefault. Mirrors
	// CustomEventInit.cancelable, also defaulting to false.
	Cancelable bool
}

// NewCustomEvent constructs an Event for use with Element.DispatchEvent,
// mirroring the DOM's `new CustomEvent(type, init)` constructor.
func NewCustomEvent(typ string, init CustomEventInit) *Event {
	return &Event{
		Type:       typ,
		Detail:     init.Detail,
		Bubbles:    init.Bubbles,
		Cancelable: init.Cancelable,
	}
}

// CurrentTarget returns the element whose listener is currently running.
// During the capture and bubble phases this differs from Target, which stays
// fixed.
func (e *Event) CurrentTarget() *Element {
	return &Element{node: e.current, doc: e.doc}
}

// StopPropagation prevents the event from reaching any further ancestors, or
// descendants during capture, after the current phase's remaining listeners
// on this node have run.
func (e *Event) StopPropagation() {
	e.stopped = true
}

// StopImmediatePropagation is like StopPropagation but also skips any
// remaining listeners on the current node.
func (e *Event) StopImmediatePropagation() {
	e.stopped = true
	e.stoppedImmediate = true
}

// PreventDefault suppresses the event's built-in default action, such as a
// checkbox toggling its checked state on click. It is a no-op unless
// e.Cancelable is true, matching real DOM's preventDefault behavior.
func (e *Event) PreventDefault() {
	if e.Cancelable {
		e.defaultPrevented = true
	}
}

// DefaultPrevented reports whether PreventDefault was called.
func (e *Event) DefaultPrevented() bool {
	return e.defaultPrevented
}

// AddEventListener registers fn to run when an event of type typ is
// dispatched to el, during the capture phase if capture is true and
// otherwise during the bubble phase. The target phase runs every listener on
// the target node regardless of its capture flag. Listeners are stored on the
// Document, keyed by el's underlying node, rather than on Element itself,
// which is a throwaway handle recreated on every lookup (see element.go), so
// a listener persists across separately-obtained Elements for the same node.
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

// hasListener reports whether the listener with id id is still registered on
// n. It is runDispatch's per-listener recheck against the live list, so a
// listener that another listener removed earlier in the same dispatch isn't
// called off the snapshot anyway, matching real DOM's "if listener's removed
// is true, continue".
func (d *Document) hasListener(n *html.Node, id listenerID) bool {
	return slices.ContainsFunc(d.listeners[n], func(e listenerEntry) bool {
		return e.id == id
	})
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

// newEvent constructs the Event that dispatch and runDispatch share, without
// running any listeners yet. It is the seam DispatchPaste and DispatchCut use
// to set ClipboardData between construction and the capture, target, and
// bubble walk, the same way a real ClipboardEvent's clipboardData is already
// populated by the time a listener sees it.
func (d *Document) newEvent(target *html.Node, typ, key string, mods Modifiers) *Event {
	return &Event{
		Type:       typ,
		Target:     &Element{node: target, doc: d},
		Key:        key,
		ShiftKey:   mods.Shift,
		CtrlKey:    mods.Ctrl,
		AltKey:     mods.Alt,
		MetaKey:    mods.Meta,
		Bubbles:    defaultBubbles(typ),
		Cancelable: defaultCancelable(typ),
		doc:        d,
	}
}

// defaultBubbles reports whether a built-in event type typ's dispatch runs
// the bubble phase after target. True for every type but the three below:
// real spec's own per-type table isn't uniform either, since focus, blur,
// and resize don't bubble there, but this package has no general per-type
// bubbling suppression (see COMPATIBILITY.md's "focus"/"blur" deviation),
// so those three bubble here anyway.
//
// The exceptions are the ones kept spec-accurate because nothing here ever
// depended on the simpler, bubbling behavior: HTMLDetailsElement's "toggle",
// and HTMLDialogElement's "close" and "cancel". A dialog's two events not
// bubbling matters more than a details' does, since dialogs nest: a "close"
// that bubbled would reach an outer dialog's own listener.
func defaultBubbles(typ string) bool {
	switch typ {
	case "toggle", "close", "cancel":
		return false
	default:
		return true
	}
}

// defaultCancelable reports whether a built-in event type typ's own
// Dispatch* method already gates its default action on DefaultPrevented().
// click, keydown, paste, cut, submit, reset, and cancel do, since each
// already has default-action code it skips when prevented ("reset" via
// triggerReset, which skips its own restore walk; "cancel" via DispatchKey's
// Escape handling, which skips the close that would otherwise follow).
// input, change, close, focus, blur, resize, and
// keyup don't, since nothing gates on them today, matching real spec for
// input, change, close, focus, and blur, and simply having nothing to skip
// for resize and keyup (DispatchKeyUp runs no default action at all; see its
// own doc comment). Any other, custom type is false here: custom events get
// their Cancelable from CustomEventInit instead.
func defaultCancelable(typ string) bool {
	switch typ {
	case "click", "keydown", "paste", "cut", "submit", "reset", "cancel":
		return true
	default:
		return false
	}
}

// dispatch builds a fresh Event via newEvent and runs it. It is the shape
// every caller other than DispatchPaste and DispatchCut uses, those two being
// the ones that need to set a field between construction and the listener
// walk.
func (d *Document) dispatch(target *html.Node, typ, key string, mods Modifiers) *Event {
	ev := d.newEvent(target, typ, key, mods)
	return d.runDispatch(ev, target, ev.Bubbles)
}

// runDispatch runs ev's typ-listeners registered against target and its
// ancestors, in capture order from root toward target, then target-phase
// order, then, if bubbles is true, bubble order from target toward root,
// honoring StopPropagation and StopImmediatePropagation.
//
// It guards against reentrant dispatch of the same Event object, as when a
// listener calls DispatchEvent on the very Event it's currently handling: a
// call while ev.dispatching is already true is a no-op, returning ev
// unchanged. It also centralizes ev's per-dispatch state, meaning the Target
// and doc assignment and the reset of stopped and stoppedImmediate, inside
// that same guarded block, so a blocked reentrant call can't corrupt an
// outer, in-flight dispatch's state before the guard runs. defaultPrevented
// is deliberately not reset, since real spec never resets it on redispatch;
// see docs/proposals/CUSTOM_EVENTS.md.
func (d *Document) runDispatch(ev *Event, target *html.Node, bubbles bool) *Event {
	if ev.dispatching {
		return ev
	}
	ev.dispatching = true
	defer func() { ev.dispatching = false }()

	ev.doc = d
	ev.Target = &Element{node: target, doc: d}
	ev.stopped = false
	ev.stoppedImmediate = false

	typ := ev.Type
	chain := ancestorChain(target)

	runNode := func(n *html.Node, wantCapture *bool) bool {
		ev.current = n
		// Iterate over a snapshot, not d.listeners[n] itself: a listener is
		// free to call AddEventListener or RemoveEventListener during its own
		// dispatch, a one-shot handler removing itself being the common case,
		// and RemoveEventListener shifts the live slice in place. Ranging
		// over it directly would skip the entry after a removed one and run
		// the last entry twice. Real DOM specifies the same thing: the
		// listener list is snapshotted per node before it's walked, so
		// additions during dispatch aren't called for this event either.
		for _, e := range slices.Clone(d.listeners[n]) {
			if e.typ != typ {
				continue
			}
			if wantCapture != nil && e.capture != *wantCapture {
				continue
			}
			if !d.hasListener(n, e.id) {
				continue // removed by an earlier listener in this same dispatch
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

	if bubbles {
		bubble := false
		for i := len(chain) - 2; i >= 0; i-- {
			if !runNode(chain[i], &bubble) || ev.stopped {
				return ev
			}
		}
	}

	return ev
}
