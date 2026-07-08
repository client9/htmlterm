package htmlterm

import (
	"testing"
	"time"
)

// newTestLoop returns a Loop suitable for exercising timer.go directly,
// without ever calling Run — none of SetInterval/SetTimeout/ClearInterval/
// ClearTimeout/handleTimerFire touch l.doc, so a nil Document is fine
// here. Backed by a real vt.MockTerm-driven Screen (see
// cellbridge_test.go's newTestScreen), since addTimer sends events onto
// l.screen's EventQ rather than a channel of its own.
func newTestLoop(t *testing.T) *Loop {
	t.Helper()
	scr, _ := newTestScreen(t, 20, 5) // already Init'd, with Fini registered via t.Cleanup
	return newLoopWithScreen(nil, scr)
}

// recvFire waits up to 1s for a *timerFireEvent on l.screen's EventQ,
// skipping over any other event kind that arrives first (e.g. Screen.Init
// posts an initial *tcell.EventResize to report starting dimensions,
// which a real Run loop would simply handle and move past) — failing the
// test on overall timeout instead of hanging forever if the timer
// mechanism is broken.
func recvFire(t *testing.T, l *Loop) timerID {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-l.screen.EventQ():
			if fire, ok := ev.(*timerFireEvent); ok {
				return fire.id
			}
		case <-deadline:
			t.Fatal("timed out waiting for a timer fire")
			return 0
		}
	}
}

func TestSetIntervalRepeats(t *testing.T) {
	l := newTestLoop(t)
	count := 0
	h := l.SetInterval(5*time.Millisecond, func() { count++ })
	defer l.ClearInterval(h)

	l.handleTimerFire(recvFire(t, l))
	l.handleTimerFire(recvFire(t, l))

	if count != 2 {
		t.Fatalf("count = %d, want 2 (interval should keep firing)", count)
	}
	if _, ok := l.timers[h.id]; !ok {
		t.Fatal("interval timer's state was removed after firing, want it to remain registered")
	}
}

func TestSetTimeoutFiresOnce(t *testing.T) {
	l := newTestLoop(t)
	count := 0
	h := l.SetTimeout(5*time.Millisecond, func() { count++ })

	l.handleTimerFire(recvFire(t, l))

	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if _, ok := l.timers[h.id]; ok {
		t.Fatal("one-shot timer's state was not removed after firing")
	}
}

func TestClearTimerDropsStaleFire(t *testing.T) {
	l := newTestLoop(t)
	fired := false
	h := l.SetInterval(time.Hour, func() { fired = true }) // long enough it never fires on its own

	// Simulate a fire already sitting on the event queue at the moment
	// ClearInterval runs.
	l.postTimerFire(h.id)

	l.ClearInterval(h)
	id := recvFire(t, l)
	l.handleTimerFire(id)

	if fired {
		t.Fatal("callback ran for a timer canceled before its fire was processed")
	}
	if _, ok := l.timers[h.id]; ok {
		t.Fatal("timer state should have been removed by ClearInterval")
	}
}

func TestClearTimeoutCancelsBeforeFiring(t *testing.T) {
	l := newTestLoop(t)
	fired := false
	h := l.SetTimeout(time.Hour, func() { fired = true })

	l.ClearTimeout(h)

	if _, ok := l.timers[h.id]; ok {
		t.Fatal("timer state should have been removed by ClearTimeout")
	}
	if fired {
		t.Fatal("callback should not have run")
	}
}
