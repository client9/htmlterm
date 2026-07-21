# tui

`github.com/client9/htmlterm/tui` drives a [`document.Document`](../document) interactively against a real terminal, using [`tcell`](https://github.com/gdamore/tcell) for raw-mode input, mouse reporting, and screen diffing.

This is the only part of `htmlterm` that depends on `tcell`. That's a deliberate boundary: a consumer that only wants one-shot rendering (`htmlterm.New`/`Render`) or a mutate-and-rerender `Document` never has to pull `tcell` — and its raw-mode/ioctl/terminfo dependencies — into their binary. `tui` in turn touches nothing of `document` but its exported API (`Document`, `Element`, `DispatchResize`, `Element.IsTextEntry`, `Document.ContentOffset`, etc.).

## Usage

```go
doc, err := document.ParseDocument(formHTML, document.Options{
    Width:  htmlterm.SizeAutomatic, // track terminal width live
    Height: htmlterm.SizeNatural,   // never clip/pad height
})

loop, err := tui.NewLoop(doc)

loop.SetInterval(500*time.Millisecond, func() {
    // update a spinner element, e.g. via doc.GetElementByID(...).SetAttribute(...)
})

if err := loop.Run(); err != nil {
    log.Fatal(err)
}
```

See [`cmd/htmlterm-tui`](../cmd/htmlterm-tui) for a complete runnable example: a form, a `SetInterval`-driven spinner and live clock, and a long paragraph that reflows live as the terminal is resized.

## API surface

```go
func NewLoop(doc *document.Document) (*Loop, error)
func (l *Loop) Run() error
func (l *Loop) Quit()

func (l *Loop) SetInterval(d time.Duration, fn func()) TimerHandle
func (l *Loop) SetTimeout(d time.Duration, fn func()) TimerHandle
func (l *Loop) ClearInterval(h TimerHandle)
func (l *Loop) ClearTimeout(h TimerHandle)
```

- **`NewLoop`** builds a `Loop` backed by a new `tcell.Screen` for the process's controlling terminal. Timers may be registered any time after construction, including before `Run` is called.
- **`Run`** initializes the screen (raw mode, mouse reporting) and repaints `doc` after every keyboard/mouse event, after every fired timer, and after every terminal resize, until Ctrl-C is read, `Quit` is called, or the screen's event stream ends. The terminal is always restored to its original state before `Run` returns, even on error. `tcell.Screen` owns the whole terminal from `Init` onward — this is a full-screen-owning TUI, with no inline/preserve-scrollback mode.
- **`Quit`** is the programmatic equivalent of the user pressing Ctrl-C — for a host that wants its own typed/dispatched quit command (e.g. a `Document` event listener reacting to a "quit" command) to end `Run` without requiring the raw Ctrl-C key sequence. Like timer callbacks, it's meant to be called from `Run`'s own goroutine; `Run` returns after the event currently being handled finishes, skipping the final repaint.
- **`SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout`** mirror `window.setInterval`/`setTimeout` for periodic updates (a spinner, a clock) that aren't triggered by user input at all. Callbacks run on `Run`'s own goroutine — the same goroutine that dispatches input events — so they may freely mutate the `Document` or register/cancel further timers.

Keyboard, mouse, and resize events read from `tcell.Screen.EventQ` are translated into calls on `Document`'s public dispatch API (`DispatchKey`/`DispatchClick`/`DispatchWheel`/`SetSize`+`DispatchResize`); a repaint follows each one. Timer fires are delivered as synthetic events on that same queue (see `timer.go`'s `timerFireEvent`), so they're handled by the same single-goroutine event loop rather than a separate channel `Run` has to select on.

## Package files

- `tcell_loop.go` — `Loop`: drives a `Document` interactively against a real terminal via `tcell.Screen`; translates screen events into `Document` dispatch calls and repaints via `cellbridge.go`.
- `cellbridge.go` — the ANSI-string-to-cell bridge between `Document.Render`'s output and `tcell.Screen`: `paintLines` decodes each already-rendered ANSI line (via `github.com/charmbracelet/x/ansi`, a general-purpose ANSI library `internal/render` also depends on — not a boundary crossing) and writes it into the screen cell by cell, blanking every column/row a given call doesn't touch.
- `timer.go` — `Loop.SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout` and their internal ticker/timer bookkeeping.

## Key invariants

- **No locking in this layer.** `Loop.Run`'s main goroutine is the only place that ever mutates `doc` or calls `paint()`; timer callbacks and dispatched event handlers all run there too, reached only via `tcell.Screen`'s single `EventQ()` channel. Nothing in `Loop` is safe to call from another goroutine.
- **Every paint redraws the whole frame.** `cellbridge.go`'s `paintLines` must explicitly blank every column past a line's own content, and every row past the frame's own line count, up to the screen's full size — there is no line-level diff repaint yet (see [`REPAINT.md`](../docs/REPAINT.md)).
- **POSIX-oriented.** Automatic resize tracking relies on `syscall.SIGWINCH`, which doesn't exist on Windows — a compile-time constraint there, not just an unverified one.

## Testing

- `tcell_loop_test.go` — event dispatch, repaint triggering, resize handling, and terminal lifecycle, against a `tcell` screen backed by `vt.MockTerm` rather than a real terminal.
- `cellbridge_test.go` — ANSI decoding and cell-writing correctness, including the hyperlink-id workaround described in `cellbridge.go`.
- `timer_test.go` — `SetInterval`/`SetTimeout` firing, cancellation, and interaction with `Run`'s event loop.

## See also

- [`document`](../document) — the DOM/event layer this package drives; read its README first for what a `Document` can do independent of any terminal.
- [`INTERACTIVE.md`](../docs/INTERACTIVE.md), [`REPAINT.md`](../docs/REPAINT.md), [`SCROLLING.md`](../docs/SCROLLING.md) — design history and rationale for the interactive/rendering-engine work; read these before extending this package.
