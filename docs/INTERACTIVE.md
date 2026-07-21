# Interactive Rendering: Design Notes

## Motivation

htmlterm renders a restricted HTML+CSS subset to terminal output. Two things
are missing: flexbox/grid layout (not addressed here), and interactivity
(forms, live updates). This document covers the interactivity track. Goal:
stay as close to the HTML/DOM/CSS model as possible; no scripting engine —
event handling is native Go, following the same conceptual model
(addEventListener-style dispatch), not an Elm/Bubble-Tea message-passing
redesign. htmlterm is evolving from a pure rendering library into a full TUI
framework built on that same web/DOM/CSS/event model — "no scripting engine"
means you program it in Go instead of JavaScript, not that the framework
stops at rendering.

## Status: RENDERING.md landed (all 9 steps); events landed; terminal I/O landed — track complete

**RENDERING.md**, the proposal to replace the string-concatenation rendering
core with one where every element's content carries its own position/size as
first-class data (the `box` type, token-based inline wrapping), has landed in
full — see that document's own migration-step history. With it in place:

- Phase 1 (`Document`/`Element`, below) is done and unaffected — it only
  depended on `Renderer.Render`'s public string-in/string-out contract,
  which the rewrite preserved.
- Form-control rendering (below) is **done**, built directly against the new
  `box`/token machinery as planned, including `<select>` — its closed state
  renders like `<input>`/`<button>` (formcontrol.go), and its open dropdown
  is composited via the popups/z-order mechanism below.
- The hit-test map (below) is **done**: `Document.Rect(el)`.
- The popup column-splicing primitive (`spliceColumns`) is **done and
  wired**: `select_popup.go`'s `compositeOpenSelects` is its first (and so
  far only) caller, driving `<select>`'s dropdown — see RENDERING.md's
  "Popups / z-order" section.
- The event system (below) is **done**.

## Phase 1 (done): `Document` + `Element`

A persistent, mutable wrapper around the parsed tree, so a host can mutate
attributes and re-render in a loop (e.g. a hand-rolled Bubble-Tea-style
loop), instead of `Renderer.Render`'s parse-once-discard model.

- `document.go`: `Document`, `ParseDocument`, `Render`, `GetElementByID`,
  `QuerySelector`/`QuerySelectorAll` (reusing `parseSelector`/
  `matchSelector`, selector.go).
- `element.go`: `Element` (attribute get/set/remove/has, `Value`/
  `SetValue`, `Checked`/`SetChecked`) and `ClassList`.
- `setAttr`/`removeAttr` added next to `nodeAttr` in selector.go.
- **Key decision:** `Document.Render` never runs `Options.StripHiddenInline`
  — it structurally deletes nodes (`strip.go`), correct for one-shot
  sanitization of untrusted HTML via `Renderer.Render`, but destructive
  against a tree meant to be mutated further. Covered by
  `TestDocumentRenderIgnoresStripHiddenInline` in document_test.go.
- **Done:** `Document.SetInnerHTML(el, fragment)` replaces el's children with
  a freshly parsed fragment (parsed in el's own context, so e.g. a bare
  `<tr>` fragment needs el to be a `<table>`/`<tbody>`). This is the
  mechanism for splicing structural, host-controlled content — a
  freshly-fetched table, a rendered email body — into a container a running
  `Loop` is driving, since `Loop` holds one `Document` pointer for its whole
  run with no document-swap API. Superseded the `ImportHTML` idea originally
  sketched here: no sanitization step, since the caller (not untrusted
  external input parsed once at ingestion) controls what fragment goes in;
  `Options.StripHiddenInline`-style sanitization, if ever wanted for
  untrusted fragments, would layer on top rather than replace this. Does not
  invalidate `cachedRules`, so a fragment must not itself carry `<style>`
  elements — page-level CSS stays in `Options.CSS`/`Stylesheets`, set once at
  `ParseDocument` time. Clears focus (silently, no `blur` dispatched) if the
  focused element was inside the replaced subtree, since it's gone rather
  than blurred; listeners on now-detached descendants become unreachable,
  the same leak a real DOM has without `removeEventListener` first.
  `scrollOffsets`/`scrollViewport`/`contentOffsets` need no cleanup — all
  three are rebuilt wholesale on the next `Render`. Covered by
  `TestSetInnerHTMLReplacesChildren`, `TestSetInnerHTMLTableFragmentInTableContext`,
  `TestSetInnerHTMLClearsFocusOnRemovedDescendant`,
  `TestSetInnerHTMLPreservesFocusOutsideReplacedSubtree`, and
  `TestSetInnerHTMLNilElement` in document_test.go.

## Done: render actual form controls

`<input>`/`<button>`/`<textarea>`/`<select>` now have UA-stylesheet entries
and attribute-driven rendering, in `formcontrol.go` plus UA rules in
`htmlterm.go` and CSS.md's element table. `<select>`'s open dropdown is a
separate compositing step — see `select_popup.go` and the "Popups / z-order"
section below.

- UA stylesheet: `input, button, select { display: inline-block; }`;
  `textarea { display: block; border-style: solid; padding-left: 1;
  padding-right: 1; }` (a real bordered box, via the normal block pipeline,
  not a special case); `form`/`fieldset`/`legend` also get sensible block
  defaults (fieldset bordered, legend bold on its own line — a simplified
  terminal approximation of browsers' border-straddling placement).
- `inputDisplayText` (`formcontrol.go`) synthesizes `<input>`'s box from
  **attributes**, not children (it has none): text-like → `[value or
  placeholder]`; checkbox → `☐`/`☑` (`checked` attribute); radio → `○`/`●`;
  submit/reset/button → `[ Label ]` (falling back to a type-appropriate
  default label); hidden → nothing. `<button>` keeps rendering its children
  normally, wrapped in brackets via UA `::before`/`::after` content — no
  synthesis needed since it already has real children to render.
  `<textarea>` shows its `value` attribute if set, else falls back to its
  child text (HTML's real default-value rule, one leading newline ignored).
  `selectDisplayText` synthesizes a closed `<select>`'s box the same way:
  the selected option's label (first `<option selected>`, else the first
  `<option>`), bracketed with a disclosure indicator.
- Composes with Phase 1 automatically: `Element.SetValue`/`SetChecked`
  already mutate the exact attributes this reads from.
- Written directly against RENDERING.md's `box`/token machinery (e.g.
  `<textarea>`'s content reuses `appendText`'s existing newline-to-`brk`
  splitting so the standard block wrap/border/padding pipeline needs no
  special-casing beyond "where do the tokens come from").

## Done: hit-test map

`Document.Rect(el) (Rect, bool)` (document.go) is a lookup into the
position map `Render()` already builds — no synthetic markers, no post-hoc
string scanning, superseding the original OSC-marker design outright. One
thing changed from the original design during implementation: **there is no
opt-in.** The original plan sketched something like
`Document.TrackForHitTest(el)`, scoped to form-control-shaped elements as
the primary motivating case. That turned out unnecessary — tracking a
position is a free byproduct of composition for *any* element that produces
its own box (every block-level element, table, list, inline-block element
including all form controls, and plain inline elements like `<span>`/
`<label>` via token-splicing — see wraptoken.go's `Rect` doc comment and
RENDERING.md's Position tracking section), so `Document.Rect` works on any
element, not just ones explicitly marked. An open `<select>`'s popup extends
this with synthetic, non-box-produced `Rect`s for its `<option>` children —
see `select_popup.go`'s `compositeSelectPopup`.

Known, documented gaps (see wraptoken.go's `Rect` and inline.go's/render.go's
matching "default" dispatch case comments): a trackable element nested
inside an `<a>` or inside another `display:inline-block` element doesn't
get its own position — both those wrapper kinds stay string-based rather
than token-spliced, since whole-string OSC8 hyperlink wrapping and
inline-block's atomic-unit contract don't have a cheap token-level
equivalent, and neither case realistically nests a *further* trackable
element (e.g. a second form control) in practice. Horizontal margin is also
not inset out of a tracked element's own `Rect` (a documented
simplification — see RENDERING.md) for elements that set
`margin-left`/`margin-right` explicitly, which form controls essentially
never do.

## Done: events

Go-native `AddEventListener`-style dispatch with capture/target/bubble
phases and default actions (space/click toggles a checkbox, typed
characters append to a focused input's value unless prevented), building on
the hit-test map for mouse click routing and Phase 1's attribute API for the
actual mutation. New file `event.go`; focus/hit-test/keyboard entry points
added to `document.go`.

- Listener storage lives on `Document` (`listeners map[*html.Node][]listenerEntry`),
  not `Element` — `Element` is a throwaway handle reconstructed on every
  lookup (Phase 1), so keying by the underlying `*html.Node` (the same
  identity `positions` already uses) is what makes a listener survive across
  separately-obtained `Element`s for the same node.
  `Document.AddEventListener(el, typ, capture, fn) ListenerHandle` /
  `RemoveEventListener(handle)` — a handle, not the func value, since Go
  funcs aren't comparable.
- `Document.dispatch` builds the root→target ancestor chain and runs
  capture-phase listeners, then the target's own listeners, then (unless
  stopped) bubble-phase listeners, honoring `Event.StopPropagation`/
  `StopImmediatePropagation`/`PreventDefault`.
- `Document.elementAt(row, col)` hit-tests the position map from `Rect`,
  picking the innermost (deepest-in-tree) match when Rects overlap (e.g. a
  `<label>` wrapping an `<input>`) — tree depth, not rect area, breaks ties.
  `DispatchClick` wraps hit-test + dispatch + default action (checkbox
  toggle; radio check-and-clear-siblings, scoped to the nearest ancestor
  `<form>` or the whole document if none; toggling an open/closed `<select>`
  dropdown, or selecting a clicked `<option>` within one — see
  `applySelectClick`, document/select.go).
- **Focus manager + live pseudo-classes:** `:checked`/`:disabled`/
  `:required` fall out of attribute-presence matching in `matchPseudo`, as
  planned — pure additions, no new state. `:focus` uses a reserved marker
  attribute (`data-htmlterm-focus`) that `Document.Focus`/`Blur` set/clear
  and `matchPseudo` checks — exactly the design this section originally
  sketched, avoiding any change to `matchSelector`/cascade's signatures.
  `Document.FocusNext`/`FocusPrev` walk focusable elements (non-disabled
  `input`/`button`/`textarea`/`select`, `input[type=hidden]` excluded) in
  document order, rebuilt fresh each call. `:hover`/`:active` are **not**
  implemented — no mouse-move/press tracking exists yet — but reserve the
  same attribute pattern (`data-htmlterm-hover`/`-active`) for whenever that
  work happens.
- `Document.DispatchKey(key string)` dispatches `"keydown"` to the focused
  element; default actions: printable rune → append to a focused text-like
  `<input>`/`<textarea>`'s value, `"Backspace"` → drop the last rune, `" "`
  on a focused checkbox/radio → the same toggle as click's default action,
  `"Tab"` → `FocusNext()`, `"Enter"`/`" "` on a focused `<select>` → toggle
  its dropdown open/closed, `"Escape"` on a focused `<select>` → close its
  dropdown without changing the selection, `"ArrowUp"`/`"ArrowDown"` on a
  focused `<select>` → move its selection to the previous/next option
  (clamped, not wrapping) whether the dropdown is open or closed, matching a
  real `<select>`'s keyboard behavior — see `moveSelectSelection`,
  document/select.go. `key` follows a small fixed vocabulary (a single
  printable rune, or a named key like `"Enter"`/`"Backspace"`/`"Tab"`) —
  decoding real terminal bytes into that vocabulary is `Loop`'s job (below),
  not `DispatchKey`'s.
- Not in scope for this phase, and not accidental gaps: full HTML form
  submission (navigation/POST — htmlterm has no network/navigation concept).
- **`submit` event (added after the initial events pass, prompted by
  wanting a more interesting `cmd/htmlterm-tui` demo than per-field
  mutation):** `isSubmitControl`/`nearestForm` (document.go) — clicking an
  `input[type=submit]` or a `<button>` whose type is unset or `"submit"`
  (HTML's default button type), or pressing `"Enter"` while focused on such
  a control or on a text entry, dispatches `"submit"` on the nearest
  ancestor `<form>` (capture/target/bubble, same as any other event type —
  no new dispatch mechanism needed). `clearRadioSiblings` was refactored to
  share `nearestForm` rather than duplicating the ancestor walk. As
  before, there is no default action beyond firing the event — htmlterm has
  no navigation/network concept, so there's nothing for a listener to call
  `PreventDefault` against. `cmd/htmlterm-tui`'s form now has an
  `AddEventListener(form, "submit", ...)` handler that reveals a `#result`
  element via `ClassList().Add("visible")` plus `SetAttribute("data-name",
  …)`/`content: attr(data-name)` — demonstrating attribute-driven display
  (the same pattern `inputDisplayText` already uses) rather than adding a
  new `Element.SetTextContent` API that a static demo doesn't otherwise
  need.

## Done, then replaced: terminal I/O (`Loop`)

The first implementation of this section was fully hand-rolled
(`terminal.go`/`input.go`/`loop.go`: raw mode via `golang.org/x/term`, a
restricted byte-prefix input decoder, a DSR-cursor-query/`originRow`-based
paint loop that rendered *inline*, preserving whatever was already in the
terminal's scrollback above it). That design is described in git history
if needed, but has since been **replaced wholesale**, not incrementally
patched: `Loop` now drives a `github.com/gdamore/tcell/v3` `Screen`
instead. The rationale (why hand-rolling this layer stopped being worth
it, why tcell specifically, and why full-screen ownership was accepted as
a deliberate trade rather than preserved) isn't re-derivable from the code
or a diff — it came out of a long back-and-forth evaluating `ultraviolet`,
`containerd/console`, and `tcell` against what this package actually
needed (the CSS/DOM engine, not the terminal wiring, is htmlterm's
differentiated part) — see this repo's session history/PR description for
that evaluation if it's ever in question again; the short version is
captured in CLAUDE.md's `tcell_loop.go`/`cellbridge.go` entries.

**Current state:**
- `tcell_loop.go`'s `Loop` translates `tcell.Screen`'s `EventQ()` events
  (`EventKey`/`EventMouse`/`EventResize`, plus `timer.go`'s posted
  `timerFireEvent`) into `Document`'s existing public dispatch API
  (`DispatchKey`/`DispatchClick`/`DispatchWheel`/`SetSize`) — unchanged
  from what the original hand-rolled decoder called into. `Screen.Init`/
  `Fini` own raw mode entirely; there's no more hand-written byte decoder,
  no DSR cursor-position query, and no `originRow` — `Screen` owns the
  whole terminal from `(0,0)`, so mouse/paint coordinates are already in
  `Document`'s own coordinate space.
- `cellbridge.go` bridges `Document.Render()`'s ANSI-string output (still
  produced exactly as before — the CSS/box/wraptoken engine is completely
  untouched by this migration) into `Screen.SetContent` calls, since tcell
  has no such bridge itself. See CLAUDE.md's `cellbridge.go` entry for the
  width-1-per-rune consistency requirement, the full-row/column blanking
  requirement (a real bug, found via the same kind of live-pty testing
  this section's original gotchas were found by), and a confirmed tcell
  v3.4.0 rendering bug around repeated hyperlink styles across rows.
- This is now a **full-screen-owning TUI**, not an inline/scrollback-
  preserving one: `Screen.Init()` takes over the whole terminal, and
  `Loop` forces `Document`'s width *and* height to the real terminal size
  on every resize (not just whichever axis was `SizeAutomatic`, as the
  original design did) — content taller than the terminal is clipped, not
  left to overflow into scrollback. This was a deliberate, explicitly
  confirmed trade, not a side effect: it's the only way to get tcell's
  `Screen` as the host at all (its `mainLoop` redraws its `CellBuffer`
  unconditionally on resize, which forced choosing between fighting that
  or embracing it).
- **Non-goals from the original pass are now closed, not still open:**
  terminal resize, full mouse vocabulary (buttons/drag/motion — though
  `Loop` still only *acts on* primary-button clicks and wheel, matching
  `Document`'s existing dispatch surface), Kitty keyboard protocol,
  bracketed paste, and Windows terminal support all come from tcell
  directly now, rather than being hand-rolled gaps to close later.
- **REPAINT.md's "Phase 2" (line-level diff repaint) is superseded, not
  implemented as planned:** `Screen.Show()` does tcell's own line/cell-level
  diffing and scroll-region optimization; `Loop.paint()` no longer does a
  full clear+rewrite on every frame. See REPAINT.md for the updated status.

## Also deferred, not on the critical path

- **Chat/REPL-style rendering:** an append-only transcript doesn't need
  `Document` at all for the log — each turn is rendered independently via
  plain `Renderer.Render` and frozen once complete (written straight to
  real stdout, preserving native terminal scrollback/copy-paste). A
  still-streaming turn is repainted in place by re-rendering the
  accumulated-so-far fragment from scratch and moving the cursor up by
  `strings.Count(prevOutput, "\n")` — no new API needed. `Document` is only
  needed for the one small live input line. CSS whose truth depends on
  future siblings (`:last-child`, sibling combinators, `counter.go`'s
  whole-document `buildCounterMap` prepass) is incompatible with "freeze
  and never revisit" content and should be avoided there; turn numbering
  should be plain Go string interpolation instead of CSS counters. This is
  unaffected by RENDERING.md and can proceed independently at any time.
- **Popups / z-order beyond `<select>`:** `<select>`'s dropdown is now
  implemented — see RENDERING.md's "Popups / z-order" section and
  `select_popup.go`. Any *other* floating/overlay use case (a tooltip, a
  context menu, a modal) would reuse the same `spliceColumns`-based
  compositing pattern, but none of those has a driving use case yet, so
  nothing beyond `<select>` is built.
