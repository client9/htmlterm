# `<dialog>`

`<dialog>` covers two quite different things behind one tag, and they are
implemented by two different mechanisms here. A **non-modal** dialog is an
ordinary block box gated on an attribute, the same shape `<details>` already
has, and needs no Go code at all. A **modal** dialog is a top-layer overlay:
centered, out of normal flow, painted above everything, with focus and input
trapped inside it.

CSS.md's own `dialog` entry is a one-line pointer here. This page is the full
reference. For the gap from a browser see `COMPATIBILITY.md`; for the API
against real DOM see `docs/DOM_API.md`.

## Quick reference

| | Non-modal (`Show()`) | Modal (`ShowModal()`) |
|---|---|---|
| In normal flow | ✅ yes | no, out of flow entirely |
| Position | wherever it sits in the document | centered in the viewport |
| Width | fills its container, like any block | shrinks to fit its content |
| Paints above other content | no | ✅ yes |
| `:modal` matches | no | ✅ yes |
| `::backdrop` | n/a | ✅ opt-in |
| Focus trapped inside | no | ✅ yes |
| Clicks/wheel outside ignored | no | ✅ yes |
| Escape closes it | no | ✅ yes, firing `"cancel"` first |

Both share the rest: `border`, `padding`, `margin`, `background-color`,
`color`, `width`, and every other block property style a dialog like any
other block box.

## The UA stylesheet

```css
dialog             { display: block; border-style: solid; padding-left: 1; padding-right: 1; }
dialog:not([open]) { display: none; }
dialog:modal       { position: fixed; z-index: 2147483647; margin: auto; }
```

That is the whole implementation of dialog layout. The border and padding are
this engine's translation of a browser UA sheet's own `border: solid;
padding: 1em`, reusing the cell counts `<textarea>` already settled on. The
second rule is the same attribute-gated `display: none` path `[hidden]` and
`<details>` use.

The third rule is the top layer, and it is worth reading closely, because
nothing else implements it. `position: fixed` hands the dialog to the
ordinary out-of-flow machinery (`internal/render/outofflow.go`), which removes
it from normal flow and anchors it to the viewport. The maximal `z-index` puts
it above every other positioned element in the same paint pass. `margin: auto`
centers it on both axes. Unlike `<select>`'s dropdown, which is composited by
dedicated Go code (`select_popup.go`), a modal dialog is not special-cased
anywhere in layout: it is a positioned box, and every rule above is one an
author could have written.

Override any of it. `dialog:modal { margin: 2 auto; }` pins a modal two rows
from the top and centers it horizontally; `dialog { border-style: none; }`
drops the border from every dialog.

## Non-modal

```html
<dialog id="d"><p>Saved.</p></dialog>
```

`Element.Show()` sets `open`; `Element.Close()` clears it. In between, the
dialog is an ordinary block box in document order, and pushes the content
after it down.

`Renderer.Render`'s one-shot rendering has no interactivity, so it renders
whatever `open` state the markup specifies, and always non-modally: there is
no `ShowModal()` call for markup to express.

## Modal

```html
<dialog id="confirm">
  <p>Delete this file?</p>
  <form method="dialog">
    <button type="submit" value="cancel">Cancel</button>
    <button type="submit" value="delete">Delete</button>
  </form>
</dialog>
```

`Element.ShowModal()` sets `open`, marks the dialog modal, and moves focus to
the first focusable element inside it. From that point until it closes:

- **Focus is trapped.** Tab and Shift+Tab cycle only within the dialog, and
  `Element.Focus()` on anything outside returns false. One hook does all of
  this, in `Document.isFocusable`, which is also what makes the initial
  focusing above need no subtree logic of its own.
- **Clicks and wheel events outside are swallowed.** `DispatchClick` and
  `DispatchWheel` both return false without dispatching, focusing, scrolling,
  or running any default action. A click on the backdrop area does nothing,
  matching HTML's default.
- **Escape closes it**, firing a cancelable `"cancel"` first. Preventing that
  event keeps the dialog open. Escape unwinds one layer per press: with a
  `<select>` popup open inside the modal, the first Escape closes the popup
  and the second closes the dialog. It works even when the dialog holds no
  focus at all, which is the case for one with nothing focusable inside;
  otherwise such a dialog could never be closed, since it swallows every
  click outside itself too.

Closing puts focus back where it was when `ShowModal()` ran, provided that
element is still focusable, and drops it otherwise. Focus is never left inside
a closed dialog, which is `display: none` and renders nothing.

Nesting works. A second `ShowModal()` while one is already open makes the new
dialog topmost, and closing it hands control back to the one beneath. The
ordering is by `ShowModal()` call order, not document order.

### `:modal`

`dialog:modal` matches only while a dialog is open modally, which is what
lets a single element style differently in its two modes:

```css
dialog        { border-style: rounded; }
dialog:modal  { border-style: double; }
```

Real CSS's `:modal` also matches fullscreen elements. There is no fullscreen
concept here, so a modal `<dialog>` is the whole set.

### `::backdrop`

```css
dialog::backdrop { background-color: #000080; }
```

Fills the whole viewport behind the dialog. `background-color` is the only
supported property.

**There is no UA default, deliberately.** The fill is opaque: it replaces the
content behind it rather than tinting it, because nothing here can re-tint an
already-serialized line of ANSI. `opacity` applies to a color at
style-resolution time, long before compositing. A browser's own default
backdrop is semi-transparent and costs nothing; an opaque default here would
blank the page behind every modal, so it is opt-in instead. Without a rule, a
modal paints over the content it covers and leaves the rest readable.

`::backdrop` applies to modal dialogs only, matching real CSS, where it
applies to top-layer elements rather than to any positioned box.

## Return values

```go
dlg.ShowModal()
// ... user clicks Delete ...
dlg.ReturnValue() // "delete"
```

`CloseWith(rv)` records `rv` and then fires `"close"`, so a listener reading
`ReturnValue()` sees the settled value. `Close()` is `CloseWith("")`.
`SetReturnValue` sets one without closing.

A `<form method="dialog">` inside a dialog closes it on submit, with the
submitting control's `value` as the return value. That is the markup-only way
to close a dialog, and needs no Go callback at all. The `"submit"` event still
fires first, and preventing it suppresses the close.

Real DOM keeps `returnValue` as an IDL property with no reflecting content
attribute. This package has no separate property layer (see
`COMPATIBILITY.md`'s "Form controls are attribute-driven"), so it lives in a
reserved attribute, stripped from `OuterHTML` and `CloneNode` like every other
reserved marker.

## Events

| Event | Fires when | Bubbles | Cancelable |
|---|---|---|---|
| `"close"` | the dialog closes, by any route | no | no |
| `"cancel"` | Escape is pressed on a modal, before it closes | no | ✅ yes |

Neither bubbles, matching spec. That matters more here than it does for
`<details>`'s `"toggle"`, because dialogs nest: a `"close"` that bubbled would
reach an outer dialog's own listener.

## Not supported

- **`closedby`**, and light dismissal generally. A click on the backdrop does
  nothing; only Escape or an explicit `Close()` closes a modal.
- **A semi-transparent backdrop.** See `::backdrop` above.
- **`Element.RequestClose()`.** Use `Close()`/`CloseWith()`, or send Escape
  through `DispatchKey` to get the `"cancel"` step.
- **Focusing the dialog itself** when it contains nothing focusable. Real DOM
  focuses the `<dialog>` element; here it isn't focusable without a
  `tabindex`, so focus is dropped instead rather than left outside the modal,
  which would put a hole in the trap. Escape still closes such a dialog: see
  the modal section above.
- **Opening an already-open dialog.** `Show()` and `ShowModal()` both return
  false rather than switching a dialog between modes in place. Close it first.

## See also

- **`docs/RENDERING.md`**, "`position: absolute` / `position: fixed`" — the
  out-of-flow machinery a modal is built on, including shrink-to-fit sizing
  and auto-margin centering.
- **`docs/DOM_API.md`** — `Show`/`ShowModal`/`Close`/`ReturnValue` against
  real `HTMLDialogElement`.
- **`COMPATIBILITY.md`** — the gaps above, stated against spec.
