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
doc.Focus(name)

doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *document.Event) {
    fmt.Println("submitted:", name.Value())
})

doc.DispatchKey("h")
doc.DispatchKey("i")
doc.DispatchKey("Enter") // fires "submit" — Enter on a focused text field is an implicit submit

out, _ := doc.Render()
fmt.Print(out)
```

`document.Options` is a type alias for `htmlterm.Options`, so anything documented for the root package's `Options` (CSS, `Width`/`Height` including the `SizeAutomatic`/`SizeNatural` sentinels, `Profile`, `NoOSC8Links`, `MaxBlankLines`) applies here too. The one exception: `Document.Render` never applies `Options.StripHiddenInline` — that option permanently deletes elements from the tree, which is appropriate for one-shot sanitization of untrusted HTML but would be destructive against a tree a host intends to keep mutating.

## API surface

**`Document`/`Element`** — `ParseDocument`, `GetElementByID`, `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove (`GetAttribute`/`SetAttribute`/`RemoveAttribute`/`HasAttribute`), `ClassList` (`Contains`/`Add`/`Remove`/`Toggle`), `Value`/`SetValue`, `Checked`/`SetChecked`, `TagName`/`ID`/`TextContent`. Mutations are reflected the next time `Render` is called.

**Rendering and sizing** — `Document.Render() (string, error)`; `SetSize(width, height)`/`Size() (width, height int)` for live resize (see the root README's "Sizing and resize" section for `SizeAutomatic`/`SizeNatural` semantics); `DispatchResize()` re-renders at the current size and fires a `"resize"` event on `DocumentElement()` — there's no separate window-level concept in this package, so the document root doubles as that event's target.

**Events** — `Document.AddEventListener(el, typ, capture, fn) ListenerHandle` / `RemoveEventListener(h)`, modeled on the DOM `Event` interface: capture → target → bubble dispatch phases, `Event.StopPropagation`/`StopImmediatePropagation`/`PreventDefault`/`DefaultPrevented`. `Document.DispatchClick(row, col)`, `DispatchWheel(row, col, delta)`, and `DispatchKey(key)` hit-test/route input, run built-in default actions (checkbox/radio toggle, focus traversal, text entry, implicit form submit on Enter), and dispatch the corresponding native event.

**Focus** — `Document.Focus(el)`/`Blur()`/`FocusNext()`/`FocusPrev()`/`FocusedElement()` manage a single focused element matched by the `:focus` pseudo-class; focusable elements are form controls and other elements the built-in default actions know how to drive.

**Hit-testing and geometry** — `Document.Rect(el) (Rect, bool)` returns an element's on-screen position and size (the CSS border box) as of the most recent `Render` call — recorded for free as a byproduct of rendering, and the basis for translating real mouse coordinates into `DispatchClick` calls. `ContentOffset(el)`, `ScrollTop(el)`/`SetScrollTop(el, offset)`, and `ScrollVisible(el)` support scroll containers (`overflow: scroll|auto` with a resolved height).

**Form controls** — `<input>` (text, checkbox, radio, submit/button/reset, hidden), `<button>`, `<textarea>`, `<form>`/`<fieldset>`/`<legend>` render with terminal approximations (`[value]`, `☐`/`☑`, `○`/`●`, `[ Label ]`) driven entirely by attributes, so `Element.SetValue`/`SetChecked` are reflected on the next `Render()`. `Element.IsTextEntry()` reports whether an element accepts direct keystroke input (used by `DispatchKey` and by `tui`'s cursor placement). `<select>` is not yet supported — no dropdown-rendering concept exists.

## Package files

- `document.go` — `Document`, `ParseDocument`, `Render`, `Rect`, focus management, event dispatch, live sizing, `DispatchResize`, `ContentOffset`, scrolling.
- `element.go` — `Element`, attribute/value helpers, `ClassList`, `IsTextEntry`.
- `event.go` — the native event model: `Event`, listener registration/removal, capture/target/bubble dispatch, focus marker plumbing.
- `attrs.go` — attribute helpers shared by `Document` and `Element`.

## Testing

- `document_test.go` — `Document`/`Element` API tests: `ParseDocument`/`Render` parity with `Renderer.Render`, `GetElementByID`, `QuerySelector(All)`, attribute/`ClassList`/`Value`/`Checked` mutation reflected in subsequent `Render` output, `SetSize`/`Size` round-tripping, `DocumentElement` handle stability.
- `event_test.go` — capture/target/bubble dispatch order, `StopPropagation`/`StopImmediatePropagation`/`PreventDefault`, click hit-testing and default actions (checkbox/radio toggle, submit), keydown default actions, focus/blur, `:focus`/`:checked`/`:disabled`/`:required` selector matching.
- `helpers_internal_test.go` — package-internal coverage for `setAttr`/`removeAttr`/`nodeAttr`, plus `TestDocumentElementResizeDispatch` (pinning `DocumentElement`/`dispatch`'s plumbing for `"resize"`, since `Loop.Run`, the only public path that fires it for real, needs a real terminal).
- `helpers_test.go` — shared `stripANSI` test helper used by the two black-box test files above.

## Design notes

`Document` assumes a single-goroutine-mutates-the-tree contract: nothing in its public API is safe to call concurrently on the same `Document`, and nothing in the codebase ever does — [`tui.Loop`](../tui)'s main goroutine is the only place that mutates a `Document` in an interactive program. See [`INTERACTIVE.md`](../INTERACTIVE.md) for the full design history behind this package (focus manager, event model, form controls) and [`SCROLLING.md`](../SCROLLING.md) for the scroll-container support.

## See also

- [`tui`](../tui) — drives a `Document` against a real terminal.
- [`internal/render`](../internal/render) — the layout engine `Document.Render` calls into.
