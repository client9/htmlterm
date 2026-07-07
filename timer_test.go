package htmlterm

import (
	"io"
	"os"
	"testing"
	"time"
)

// newTestLoop returns a Loop suitable for exercising timer.go directly,
// without ever calling Run — none of SetInterval/SetTimeout/ClearInterval/
// ClearTimeout/handleTimerFire touch l.doc, l.in, or l.out, so a nil
// Document and unused in/out are fine here.
func newTestLoop() *Loop {
	return NewLoop(nil, os.Stdin, io.Discard)
}

// recvFire waits up to 1s for a fire on l.timerCh, failing the test on
// timeout instead of hanging forever if the timer mechanism is broken.
func recvFire(t *testing.T, l *Loop) timerFire {
	t.Helper()
	select {
	case fire := <-l.timerCh:
		return fire
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a timer fire")
		return timerFire{}
	}
}

func TestSetIntervalRepeats(t *testing.T) {
	l := newTestLoop()
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
	l := newTestLoop()
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
	l := newTestLoop()
	fired := false
	h := l.SetInterval(time.Hour, func() { fired = true }) // long enough it never fires on its own

	// Simulate a fire already having been forwarded (in flight on l.timerCh)
	// at the moment Clear runs — l.timerCh is unbuffered, so this send
	// blocks until read below, meaning ClearInterval is guaranteed to run
	// first regardless of goroutine scheduling.
	go func() { l.timerCh <- timerFire(h) }()

	l.ClearInterval(h)
	fire := <-l.timerCh
	l.handleTimerFire(fire)

	if fired {
		t.Fatal("callback ran for a timer canceled before its fire was processed")
	}
	if _, ok := l.timers[h.id]; ok {
		t.Fatal("timer state should have been removed by ClearInterval")
	}
}

func TestClearTimeoutCancelsBeforeFiring(t *testing.T) {
	l := newTestLoop()
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
