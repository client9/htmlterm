package tui

import (
	"time"

	"github.com/gdamore/tcell/v3"
)

// timerID identifies one registered timer within a Loop, for ClearInterval/
// ClearTimeout — mirroring listenerID's role for event listeners (event.go).
type timerID uint64

// TimerHandle identifies a timer previously registered via Loop.SetInterval
// or Loop.SetTimeout, for later cancellation via ClearInterval/ClearTimeout —
// the Go analogue of the opaque ID window.setInterval/setTimeout return in
// JS.
type TimerHandle struct {
	id timerID
}

// timerFireEvent is posted onto Loop's tcell.Screen event queue (via
// Screen.PostEvent) when a registered timer's ticker/timer fires — just
// enough to look the timer back up in Loop.timers. Embedding
// tcell.EventTime satisfies tcell.Event's When() requirement for free.
// Delivered through the same PollEvent loop as keyboard/mouse/resize
// events (tcell_loop.go), rather than a separate channel Run has to select
// on; the callback itself still runs on Run's own goroutine, so it's safe
// to mutate the Document from it.
type timerFireEvent struct {
	tcell.EventTime
	id timerID
}

// timerState is what Loop.timers stores per registered timer. Exactly one of
// ticker/timer is set, matching whether it came from SetInterval or
// SetTimeout.
type timerState struct {
	ticker *time.Ticker
	timer  *time.Timer
	fn     func()
	done   chan struct{}
	once   bool // true for SetTimeout: removed from timers after firing once
}

// SetInterval registers fn to run every d, repeatedly, until canceled via
// ClearInterval (or ClearTimeout — see there). Like window.setInterval, fn
// runs on Run's own goroutine (once Run is executing), so it may freely
// mutate l's Document or register/cancel further timers. May be called
// before Run, so a caller can arm timers as part of setup.
func (l *Loop) SetInterval(d time.Duration, fn func()) TimerHandle {
	ticker := time.NewTicker(d)
	return l.addTimer(&timerState{ticker: ticker, fn: fn, done: make(chan struct{})}, ticker.C)
}

// SetTimeout registers fn to run once, after d elapses, until canceled via
// ClearTimeout (or ClearInterval — see there). Like window.setTimeout, fn
// runs on Run's own goroutine (once Run is executing). May be called before
// Run.
func (l *Loop) SetTimeout(d time.Duration, fn func()) TimerHandle {
	timer := time.NewTimer(d)
	return l.addTimer(&timerState{timer: timer, fn: fn, done: make(chan struct{}), once: true}, timer.C)
}

// addTimer registers st under a fresh id and starts its forwarding
// goroutine, which relays every receive off src as a timerFireEvent sent
// on l.screen's event queue (Screen.EventQ) until done is closed. src is
// ticker.C or timer.C — time.Ticker's channel already carries a 1-tick
// buffer and drops ticks that aren't read promptly, so a consumer running
// behind naturally coalesces rather than building an unbounded backlog.
func (l *Loop) addTimer(st *timerState, src <-chan time.Time) TimerHandle {
	l.nextTimerID++
	id := l.nextTimerID
	l.timers[id] = st

	go func() {
		for {
			select {
			case <-src:
				l.postTimerFire(id)
			case <-st.done:
				return
			}
		}
	}()

	return TimerHandle{id: id}
}

// postTimerFire sends a timerFireEvent for id onto l.screen's event queue.
// EventQ's own doc comment says the channel stays open until Fini() and
// that callers "must not write to this channel after Fini() is called" —
// Run stops every outstanding timer (closing its done channel) before
// calling Fini (see Run's deferred stopAllTimers), but that only narrows
// the race, since a tick can still be selected concurrently with a
// deferred close; recover guards the remaining window where a send lands
// on an already-closed channel, which would otherwise panic this
// goroutine.
func (l *Loop) postTimerFire(id timerID) {
	defer func() { _ = recover() }()
	ev := &timerFireEvent{id: id}
	ev.SetEventNow()
	l.screen.EventQ() <- ev
}

// ClearInterval cancels a timer previously registered via SetInterval. It is
// a no-op if the timer was already canceled or has already fired (for a
// one-shot SetTimeout timer). Interchangeable with ClearTimeout, matching
// JS's shared clearInterval/clearTimeout ID namespace in practice.
func (l *Loop) ClearInterval(h TimerHandle) {
	l.clearTimer(h.id)
}

// ClearTimeout cancels a timer previously registered via SetTimeout. It is a
// no-op if the timer was already canceled or has already fired. Interchangeable
// with ClearInterval — see there.
func (l *Loop) ClearTimeout(h TimerHandle) {
	l.clearTimer(h.id)
}

// stopAllTimers cancels every still-registered timer — called by Run
// (tcell_loop.go) before Screen.Fini, so no timer's forwarding goroutine
// is left trying to send to Screen.EventQ after the screen tears it down.
func (l *Loop) stopAllTimers() {
	for id := range l.timers {
		l.clearTimer(id)
	}
}

// clearTimer removes id from l.timers and stops its forwarding goroutine and
// underlying ticker/timer. Deleting the map entry here, synchronously, is
// what lets handleTimerFire treat a fire for an id no longer present as
// stale (already-canceled) rather than a bug — the forwarding goroutine may
// already have a fire in flight on l.timerCh when this runs.
func (l *Loop) clearTimer(id timerID) {
	st, ok := l.timers[id]
	if !ok {
		return
	}
	delete(l.timers, id)
	close(st.done)
	if st.ticker != nil {
		st.ticker.Stop()
	}
	if st.timer != nil {
		st.timer.Stop()
	}
}

// handleTimerFire runs the callback for a received timerFireEvent's id,
// unless its timer was canceled between being posted and being received
// here (in which case id is no longer in l.timers and this is a silent
// no-op). A one-shot SetTimeout timer is removed from l.timers after
// firing, matching JS's setTimeout running at most once.
func (l *Loop) handleTimerFire(id timerID) {
	st, ok := l.timers[id]
	if !ok {
		return
	}
	if st.once {
		delete(l.timers, id)
	}
	st.fn()
}
