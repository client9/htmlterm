# Timers + Efficient Repaint: Design Notes

## Motivation

INTERACTIVE.md's event system covers user-driven updates end to end (click,
keyboard, focus, submit), and `Loop` (originally loop.go, now tcell_loop.go
— see INTERACTIVE.md's terminal I/O section for that migration) already
repaints after every dispatch. Missing: *periodic* updates not triggered by
user input at all — a spinner, a progress bar, a live clock/metric — which
need something like `setTimeout`/`setInterval` calling into Go code that
mutates a `Document` and triggers a repaint.

Building that surfaced a real inefficiency, at the time: `Loop.paint()`
always did a full `Document.Render()` (whole-tree CSS/box recompute) and a
full terminal clear+rewrite (`\x1b[J` + the entire frame), even when only
one small text node changed — e.g. a spinner glyph cycling every ~100ms
shouldn't cost a full-document repaint on every tick.

## Status: Phase 1 (timers) landed; Phase 2 (diff repaint) superseded, not implemented as planned

Phase 1 (timers) shipped as described below and is unaffected by the later
`Loop` migration (`timer.go`'s delivery mechanism changed — timer fires are
now sent on `tcell.Screen`'s `EventQ()` channel rather than a private
`timerCh` — but `SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout`'s
public behavior is identical; see CLAUDE.md's `timer.go` entry).

Phase 2 as planned below (hand-rolled line-level diffing inside `paint()`)
was never built: `Loop` was later rearchitected onto a
`github.com/gdamore/tcell/v3` `Screen`, whose own `Show()` does real
line/cell-level diffing and scroll-region optimization — a materially
*more* capable diff engine than what Phase 2 sketched (see `cellbridge.go`'s
CLAUDE.md entry), arrived at for unrelated reasons (offloading the terminal
I/O layer generally, not repaint efficiency specifically). The design
reasoning below is kept for historical context — it correctly identified
the inefficiency and the shape of a fix — but nothing past "Phase 1 (done)"
should be read as a live plan anymore.

## Phase 1 (done): timer-driven updates (the demonstrating vehicle)

`timer.go`: `Loop.SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout`,
mirroring `window.setInterval`/`setTimeout` — a `TimerHandle` in, a plain
`func()` callback (timers aren't part of the DOM capture/bubble model,
unlike `event.go`'s listeners). Built exactly as this section originally
proposed: each timer owns a `time.Ticker` (interval) or `time.Timer`
(timeout) on its own forwarding goroutine, which relays fires as a
lightweight `timerFire{id}` onto one shared, unbuffered `Loop.timerCh` — the
channel-based wakeup this doc called out as preferred. `Loop.Run` (loop.go)
now reads terminal input on its own goroutine into an `inputCh` instead of
blocking directly on `decodeEvent`, so its main loop `select`s between
`inputCh` and `timerCh`; every `Document` mutation and every `paint()` call
still happens on that one goroutine, exactly as planned — no mutex anywhere
in the package. A canceled timer's map entry is deleted synchronously by
`ClearInterval`/`ClearTimeout`, so a fire already in flight on `timerCh` at
cancellation time is simply dropped as stale by `handleTimerFire` rather
than needing a generation counter or other bookkeeping.

`cmd/htmlterm-tui` now has the demo this section asked for: a Braille
spinner (`#spinner`, cycling every 100ms) and a live `HH:MM:SS` clock
(`#clock`, updating every second), both driven by `SetInterval` and both
rendered via the same "attribute holds the value, `::before { content:
attr(...) }` displays it" pattern the form's `#result` element already used
— no new `Element.SetTextContent` needed. `paint()` itself is unmodified, as
required. Verified end to end in a real pty (tmux): spinner glyph and clock
both advance with zero keyboard/mouse input, while Tab/typing/checkbox
toggle/submit all continue to work exactly as before and Ctrl-C still exits
cleanly.

**Not done, deliberately deferred**: the instrumentation this section
originally asked for (bytes-written-per-paint or `Document.Render()`
wall-time per tick) — Phase 2 will need it as its "before" baseline, but nothing
consumes it yet, so it wasn't added speculatively. Add it as the first step
of Phase 2, not before.

## Phase 2: line-level diff repaint

Change `Loop.paint()` (currently: CUP to `(originRow, 0)` → `\x1b[J` → write
the whole frame) to diff against the *previous* frame instead of always
doing a full repaint:

- `Loop` keeps the previous frame's lines (`prevLines []string`) as new
  state.
- Split the new frame into lines the same way; compare index by index up to
  `max(len(prevLines), len(newLines))` (treat a missing side as `""` — this
  naturally handles the frame growing or shrinking a line count between
  paints).
- Identical line → skip entirely, zero terminal writes for that row.
- Differing line → CUP to `(originRow+i, 0)`, erase-to-end-of-line
  (`\x1b[K` — the line-granularity analog of the existing `\x1b[J`), write
  just that line's content. No `\n`→`\r\n` translation needed here (unlike
  `writeFrame`'s existing full-frame path, still used by the fallback below)
  since each write is a single line preceded by an absolute CUP — nothing
  relies on a bare linefeed to advance to the next row.
- **Fallback**: if the fraction of differing lines exceeds some threshold
  (discussed as roughly 50% during design, not fixed here — pick something
  reasonable and note it's tunable), just run the existing full-repaint path
  instead. Cheaper in total operations than many small diffs when almost
  everything changed anyway. The very first paint (no `prevLines` yet)
  always takes this path too — there's nothing to diff against.

**Why this is safe without inventing a new invariant**: every line this
engine renders is already independently ANSI-self-contained — opens and
closes its own SGR/OSC8 span, never leaks style across a line boundary (see
textutil.go's `ansiCarry` design, and `wordWrapANSI`'s doc comment: "every
wrapped line of a styled/linked run remains independently styled"). That
existing guarantee is exactly what makes rewriting one line in isolation
correct, with no bleed into neighboring untouched lines. Nothing new needs
to be proven here — just relied on.

**Explicitly out of scope for this phase, not an oversight — cell-level
diffing** (per-column diffing *within* a changed line: coalescing runs of
changed cells, rewriting only the exact changed glyphs instead of the whole
line). For the motivating cases here — a spinner/progress bar/clock that's
either its own line or dominates one — line-level already captures nearly
all the benefit over full-frame repaint (unrelated lines untouched at all;
a changed line's cost is bounded by *its own* width, not the document's
total height) for a fraction of the engineering. Cell-level would require
decomposing each line into a per-column style-state grid (extending
`ansiCarry`-style tracking across *every* column, not just wrap points, the
way `spliceColumns` (textutil.go) already does for a *known, fixed*
replacement range), plus a run-coalescing diff algorithm — a real new
subsystem, not a tweak. It only matters for a case this project isn't
motivated by (many independently-updating small widgets packed into wide,
mostly-static lines). Line-level and cell-level aren't mutually exclusive as
a roadmap, though: line-level decides *which lines* to touch; a future
cell-level pass could sub-diff *within* a changed line later without
discarding this work — so this isn't a dead end if that case ever shows up.

**Verification**: reuse Phase 1's instrumentation to show a concrete
before/after — e.g. "N bytes/tick, whole frame" (Phase 1 baseline) vs. "M
bytes/tick, one line" (Phase 2), with N ≫ M for any multi-line document.

## Layering note

Both phases stay entirely inside `loop.go` (plus whatever new demo file).
No changes anticipated to `Document`/`Element`/`Render`/the box model — this
is a `Loop`-level concern only, consistent with the one-directional layering
already established for the terminal I/O work: `Loop → Document →
Renderer.Render` (see INTERACTIVE.md's "Layering, deliberately unchanged by
living in one package" note). A caller who never touches timers or the new
diff path is completely unaffected.
