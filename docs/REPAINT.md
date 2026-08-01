# Timers + Efficient Repaint: Design Notes

## Motivation

INTERACTIVE.md's event system covers user-driven updates end to end: click,
keyboard, focus, submit. `Loop` (originally loop.go, now tcell_loop.go; see
INTERACTIVE.md's terminal I/O section for that migration) already repaints
after every dispatch. What was missing is *periodic* updates not triggered by
user input at all, such as a spinner, a progress bar, or a live clock or
metric. Those need something like `setTimeout` and `setInterval` calling into
Go code that mutates a `Document` and triggers a repaint.

Building that surfaced a real inefficiency, at the time. `Loop.paint()`
always did a full `Document.Render()`, a whole-tree CSS and box recompute, and
a full terminal clear-and-rewrite (`\x1b[J` plus the entire frame), even when
only one small text node changed. A spinner glyph cycling every ~100ms
shouldn't cost a full-document repaint on every tick.

## Status: Phase 1 (timers) landed; Phase 2 (diff repaint) superseded, not implemented as planned

Phase 1 (timers) shipped as described below and is unaffected by the later
`Loop` migration. `timer.go`'s delivery mechanism changed, since timer fires
are now sent on `tcell.Screen`'s `EventQ()` channel rather than a private
`timerCh`, but `SetInterval`, `SetTimeout`, `ClearInterval`, and
`ClearTimeout`'s public behavior is identical; see `tui/timer.go`.

Phase 2 as planned below, hand-rolled line-level diffing inside `paint()`, was
never built. `Loop` was later rearchitected onto a
`github.com/gdamore/tcell/v3` `Screen`, whose own `Show()` does real line- and
cell-level diffing and scroll-region optimization, a materially *more* capable
diff engine than what Phase 2 sketched (see `tui/cellbridge.go` and
`tui/README.md`). That was arrived at for unrelated reasons, namely offloading the
terminal I/O layer generally, not repaint efficiency specifically. The design
reasoning below is kept for historical context, since it correctly identified
the inefficiency and the shape of a fix, but nothing past "Phase 1 (done)"
should be read as a live plan anymore.

## Phase 1 (done): timer-driven updates (the demonstrating vehicle)

`timer.go` provides `Loop.SetInterval`, `SetTimeout`, `ClearInterval`, and
`ClearTimeout`, mirroring `window.setInterval` and `setTimeout`: a
`TimerHandle` in, a plain `func()` callback, since timers aren't part of the
DOM capture and bubble model the way `event.go`'s listeners are. It was built
exactly as this section originally proposed. Each timer owns a `time.Ticker`,
for an interval, or a `time.Timer`, for a timeout, on its own forwarding
goroutine, which relays fires as a lightweight `timerFire{id}` onto one
shared, unbuffered `Loop.timerCh`, the channel-based wakeup this doc called
out as preferred.

`Loop.Run` (loop.go) now reads terminal input on its own goroutine into an
`inputCh` instead of blocking directly on `decodeEvent`, so its main loop
`select`s between `inputCh` and `timerCh`. Every `Document` mutation and every
`paint()` call still happens on that one goroutine, exactly as planned, with
no mutex anywhere in the package. A canceled timer's map entry is deleted
synchronously by `ClearInterval` and `ClearTimeout`, so a fire already in
flight on `timerCh` at cancellation time is dropped as stale by
`handleTimerFire` rather than needing a generation counter or other
bookkeeping.

`cmd/htmlterm-tui` now has the demo this section asked for: a Braille
spinner (`#spinner`, cycling every 100ms) and a live `HH:MM:SS` clock
(`#clock`, updating every second), both driven by `SetInterval` and both
rendered via the same "attribute holds the value, `::before { content:
attr(...) }` displays it" pattern the form's `#result` element already used.
No new `Element.SetTextContent` was needed. `paint()` itself is unmodified, as
required. Verified end to end in a real pty (tmux): spinner glyph and clock
both advance with zero keyboard or mouse input, while Tab, typing, checkbox
toggle, and submit all continue to work exactly as before, and Ctrl-C still
exits cleanly.

## Phase 2 (superseded, not built): line-level diff repaint

Phase 2 as originally scoped would have added hand-rolled line-level diffing
inside `paint()`: track the previous frame's lines, rewrite only the lines
that changed, and fall back to a full repaint above some differing-line
threshold. Cell-level, per-column diffing was explicitly named as a further,
larger step out of scope even for that plan.

None of it was built. `Loop`'s later migration onto a
`github.com/gdamore/tcell/v3` `Screen` gave `paint()` real line- and
cell-level diffing via `Screen.Show()` for free. That was arrived at for
unrelated reasons, offloading the terminal I/O layer generally, but strictly
supersedes what this design would have delivered. See INTERACTIVE.md's
terminal I/O section, and `tui/cellbridge.go`, for what shipped instead.

## Layering note

Phase 1 lives in `timer.go`. No changes were made to `Document`, `Element`,
`Render`, or the box model: timers are a `Loop`-level concern only, consistent
with the one-directional layering established for the terminal I/O work,
`Loop → Document → Renderer.Render` (see INTERACTIVE.md's "Layering,
deliberately unchanged by living in one package" note). A caller who never
touches timers is unaffected.
