# document

`github.com/client9/htmlterm/document` is `htmlterm`'s mutable DOM and event layer for interactive terminal applications.

Where `Renderer.Render` (the root package) parses HTML, renders it once, and discards the tree, `Document` parses once and can be queried, mutated, and re-rendered repeatedly — the basis for a host-driven interactive loop such as a form whose fields update in response to keystrokes. It depends only on `internal/render` and `internal/cssengine`; it has no dependency on a terminal library. That's [`tui`](../tui)'s job.

## Install

```bash
go get github.com/client9/htmlterm/document
```

## Usage

```go
doc, err := document.ParseDocument(`
    <form id="f">
      <label>Name: <input id="name"></label>
      <button type="submit">Submit</button>
    </form>`, document.Options{Width: 40})

name := doc.GetElementByID("name")
name.Focus()

doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *document.Event) {
    fmt.Println("submitted:", name.Value())
})

doc.DispatchKey("h", document.Modifiers{})
doc.DispatchKey("i", document.Modifiers{})
doc.DispatchKey("Enter", document.Modifiers{}) // fires "submit" — Enter on a focused text field is an implicit submit

out, _ := doc.Render()
fmt.Print(out)
```

`document.Options` is a type alias for `htmlterm.Options`, so anything documented for the root package's `Options` (CSS, `Width`/`Height` including the `SizeAutomatic`/`SizeNatural` sentinels, `Profile`, `NoOSC8Links`, `MaxBlankLines`) applies here too. The one exception: `Document.Render` never applies `Options.StripHiddenInline` — that option permanently deletes elements from the tree, which is appropriate for one-shot sanitization of untrusted HTML but would be destructive against a tree a host intends to keep mutating.

## API surface

**`Document`/`Element`** — `ParseDocument`, `GetElementByID`, `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove (`GetAttribute`/`SetAttribute`/`RemoveAttribute`/`HasAttribute`), `ClassList` (`Contains`/`Add`/`Remove`/`Toggle`), `Value`/`SetValue`, `Checked`/`SetChecked`, `TagName`/`ID`/`TextContent`. Mutations are reflected the next time `Render` is called.

**Rendering and sizing** — `Document.Render() (string, error)`; `SetSize(width, height)`/`Size() (width, height int)` for live resize (see the root README's "Sizing and resize" section for `SizeAutomatic`/`SizeNatural` semantics); `DispatchResize()` re-renders at the current size and fires a `"resize"` event on `DocumentElement()` — there's no separate window-level concept in this package, so the document root doubles as that event's target.

**Events** — `Document.AddEventListener(el, typ, capture, fn) ListenerHandle` / `RemoveEventListener(h)`, modeled on the DOM `Event` interface: capture → target → bubble dispatch phases, `Event.StopPropagation`/`StopImmediatePropagation`/`PreventDefault`/`DefaultPrevented`; `Event` also carries `ShiftKey`/`CtrlKey`/`AltKey`/`MetaKey`, mirroring a real DOM event's modifier-key booleans (zero for event types with no real modifier-key concept, e.g. `"submit"`/`"focus"`/`"blur"`/`"change"`; `"input"` carries them like `"keydown"` does). `Document.DispatchClick(row, col, mods Modifiers)`, `DispatchWheel(row, col, deltaX, deltaY int)`, and `DispatchKey(key string, mods Modifiers)` hit-test/route input, run built-in default actions (checkbox/radio toggle, focus traversal, text entry, implicit form submit on Enter, form reset on a reset control), and dispatch the corresponding native event. A focused text `<input>`/`<textarea>`'s `DispatchKey`-driven value mutations fire `"input"` on every keystroke and `"change"` once on commit (Enter, or losing focus) if the value differs from its value when focus was gained. Activating a reset control (`<input type="reset">`, `<button type="reset">`, by click or Enter) fires `"reset"` on the nearest ancestor `<form>` and, unless a listener calls `PreventDefault`, restores every descendant control to the value, checkedness, or selected option it had at `ParseDocument` time — see `COMPATIBILITY.md`'s "Form controls are attribute-driven, not stateful" for what that snapshot does and doesn't cover. `DispatchWheel`'s `deltaX` scrolls the nearest `overflow-x:scroll|auto` ancestor horizontally, independently of `deltaY`'s vertical target — see `ScrollLeft`/`SetScrollLeft` below and the root `COMPATIBILITY.md`'s CSS "At a Glance" entry for `overflow-x`.

**Focus** — `Element.Focus()`/`Blur()` (mirroring the DOM's `HTMLElement.focus`/`blur`) plus `Document.FocusNext()`/`FocusPrev()`/`FocusedElement()`, which need whole-document state, manage a single focused element matched by the `:focus` pseudo-class; focusable elements are form controls and other elements the built-in default actions know how to drive.

**Hit-testing and geometry** — `Element.Rect() (Rect, bool)` returns an element's on-screen position and size (the CSS border box) as of the most recent `Render` call — recorded for free as a byproduct of rendering, and the basis for translating real mouse coordinates into `DispatchClick` calls. `ContentOffset(el)`/`ContentOffsetX(el)` return the row/column shift from that border box to the element's first content cell (its border-top/padding-top and border-left/padding-left) — what a caret inside a bordered, padded `<textarea>` has to be placed against. `ScrollTop(el)`/`SetScrollTop(el, offset)` and `ScrollVisible(el)` support vertical scroll containers (`overflow-y: scroll|auto` with a resolved height); `ScrollLeft(el)`/`SetScrollLeft(el, offset)` are their horizontal counterparts (`overflow-x: scroll|auto` with an explicit width) — `ScrollVisible` checks both axes. `Element.Focus()` auto-scrolls a focused descendant into view on both axes if it falls outside a scrollable ancestor's visible row/column range.

**Form controls** — `<input>` (text, checkbox, radio, submit/button/reset, hidden), `<button>`, `<textarea>`, `<form>`/`<fieldset>`/`<legend>` render with terminal approximations (`[value]`, `☐`/`☑`, `○`/`●`, `[ Label ]`) driven entirely by attributes, so `Element.SetValue`/`SetChecked` are reflected on the next `Render()`. `Element.IsTextEntry()` reports whether an element accepts direct keystroke input (used by `DispatchKey` and by `tui`'s cursor placement). `<select>` is supported, with a composited dropdown popup — see `docs/SELECT.md`: clicking the control or pressing Enter/Space opens it, `ArrowUp`/`ArrowDown` browse (arrow-key highlighting is separate from the committed value until confirmed), Enter/Space/click-on-option commits and fires `"change"`, and Escape or a click elsewhere closes it without committing.

## Package files

- `document.go` — `Document`, `ParseDocument`, `Render`, `Rect`, focus management, event dispatch, live sizing, `DispatchResize`, `ContentOffset`/`ContentOffsetX`, scrolling.
- `element.go` — `Element`, attribute/value helpers, `ClassList`, `Dataset`, `Style`, `IsTextEntry`.
- `event.go` — the native event model: `Event`, listener registration/removal, capture/target/bubble dispatch, focus marker plumbing.
- `attrs.go` — attribute helpers shared by `Document` and `Element`.
- `select.go` — `<select>` dropdown popup state: open/close, arrow-key highlight vs. committed value, option confirm/click handling.

## Testing

- `document_test.go` — `Document`/`Element` API tests: `ParseDocument`/`Render` parity with `Renderer.Render`, `GetElementByID`, `QuerySelector(All)`, attribute/`ClassList`/`Value`/`Checked` mutation reflected in subsequent `Render` output, `SetSize`/`Size` round-tripping, `DocumentElement` handle stability.
- `element_dom_test.go` — `Element`'s tree-navigation and mutation API: parent/sibling/child traversal, `AppendChild`/`InsertBefore`/`RemoveChild`/`ReplaceChild`, `CloneNode`, `OuterHTML`/`InnerHTML`.
- `event_test.go` — capture/target/bubble dispatch order, `StopPropagation`/`StopImmediatePropagation`/`PreventDefault`, click hit-testing and default actions (checkbox/radio toggle, submit, reset), keydown default actions, focus/blur, `:focus`/`:checked`/`:disabled`/`:required` selector matching.
- `dialog_test.go` — `<dialog>` behavior: `Show`/`ShowModal`/`Close`/`CloseWith`/`ReturnValue`, non-bubbling `"close"`/`"cancel"`, `<form method="dialog">` submission, and, for modals, centering, focus trapping, click swallowing outside, Escape unwinding (including ahead of a nested `<select>` popup), nesting order, and `::backdrop`.
- `datalist_test.go` — `<input list>` suggestion popup: prefix filtering as you type, ArrowDown-to-open, arrow browsing, Enter/click to pick (firing `"input"` then `"change"`), Escape and click-outside and blur dismissal, disabled options, and Escape ordering against a modal `<dialog>`.
- `select_test.go` — `<select>` dropdown popup behavior: open/close, arrow-key highlight vs. commit, `"change"` dispatch, disabled option/optgroup handling.
- `style_test.go` — `Element.Style()`'s inline-style `GetPropertyValue`/`SetProperty`/`RemoveProperty`/`CSSText`/`SetCSSText`.
- `table_separate_test.go`/`table_collapse_test.go` — `<table>` rendering through the `Document`/`Element` API for both `border-collapse` modes.
- `helpers_internal_test.go` — package-internal coverage for `setAttr`/`removeAttr`/`nodeAttr`, plus `TestDocumentElementResizeDispatch` (pinning `DocumentElement`/`dispatch`'s plumbing for `"resize"`, since `Loop.Run`, the only public path that fires it for real, needs a real terminal).
- `helpers_test.go` — shared `stripANSI` test helper used by the black-box test files above.

## Design notes

`Document` assumes a single-goroutine-mutates-the-tree contract: nothing in its public API is safe to call concurrently on the same `Document`, and nothing in the codebase ever does — [`tui.Loop`](../tui)'s main goroutine is the only place that mutates a `Document` in an interactive program. See [`INTERACTIVE.md`](../docs/INTERACTIVE.md) for the full design history behind this package (focus manager, event model, form controls) and [`SCROLLING.md`](../docs/SCROLLING.md) for the scroll-container support.

## See also

- [`tui`](../tui) — drives a `Document` against a real terminal.
- [`internal/render`](../internal/render) — the layout engine `Document.Render` calls into.
