# DOM API Coverage

A field-by-field comparison of the real DOM's `Document` and `Element`/`Node`
interfaces against `document.Document` and `document.Element`. This is a
lookup table, not an explainer. See `COMPATIBILITY.md`'s "DOM & Events"
section for the narrative version (deviations, terminal-native additions,
what's deliberately out of scope) and `docs/INTERACTIVE.md` for the design
rationale.

Column 2 is one of: the htmlterm method or property name, **Missing** (real
DOM has it, htmlterm doesn't, no workaround), **Partial** (htmlterm has
something narrower or shaped differently), or **N/A** (doesn't apply, since
there is no window, no network, and no scripting engine).

---

## `Document`

| Spec method/property | htmlterm equivalent | Notes |
|---|---|---|
| `getElementById(id)` | `GetElementByID(id)` | |
| `querySelector(sel)` | `QuerySelector(sel)` | Whole-document search; see `Element.QuerySelector` below for the subtree-scoped version. |
| `querySelectorAll(sel)` | `QuerySelectorAll(sel)` | Same as above. |
| `getElementsByClassName(cls)` | `GetElementsByClassName(cls)` | `cls` may be a whitespace-separated list of multiple class names, matching spec. |
| `getElementsByTagName(tag)` | `GetElementsByTagName(tag)` | |
| `getElementsByName(name)` | `GetElementsByName(name)` | |
| `createElement(tag)` | `CreateElement(tag)` | |
| `createTextNode(text)` | `CreateTextNode(text)` | |
| `createDocumentFragment()` | Missing | No `DocumentFragment` type; build a detached `Element` subtree via `CreateElement`/`AppendChild` and attach it directly instead. |
| `createComment(text)` | `CreateComment(text)` | A comment node never produces visible output once attached — `internal/render`'s dispatch explicitly skips `html.CommentNode` wherever it appears, the same way a browser never renders comment content either. |
| `createEvent()` / `Event` constructor | `NewCustomEvent(typ, CustomEventInit{...})` | Mirrors `new CustomEvent(type, init)`, not the legacy `createEvent`/`initEvent` two-step. Pair with `Element.DispatchEvent(ev)` to fire it — see the `dispatchEvent` row below. |
| `importNode(node, deep)` / `adoptNode(node)` | N/A | Single-document model — no cross-document node transfer exists to import/adopt between. |
| `documentElement` | `DocumentElement()` | |
| `body` | `Body()` | |
| `head` | `Head()` | |
| `title` | `Title()` (getter only) | No setter equivalent — set the `<title>` element's text content directly instead. |
| `activeElement` | `FocusedElement()` | |
| `forms` / `images` / `links` / `scripts` / `styleSheets` | `Forms()` / `Images()` / `Links()` / `Scripts()` / `StyleSheets()` | Plain `[]*Element` slices, not live `HTMLCollection`s; `styleSheets` returns the raw `<style>` elements rather than parsed CSSOM objects (this package has no CSSOM to expose). |
| `URL` / `domain` / `cookie` / `location` / `defaultView` / `readyState` / `characterSet` / `doctype` | N/A | No network origin, no window, no navigation, no async loading — none of these concepts exist for a one-shot/embedded document. |
| `addEventListener(type, fn, opts)` (via `EventTarget`) | `AddEventListener(el, typ, capture, fn)` | Lives on `Document`, not `Element` — takes the target `*Element` as a parameter rather than being called on it. See "Also available" under Element below. |
| `removeEventListener` | `RemoveEventListener(handle)` | Takes the `ListenerHandle` returned by `AddEventListener`, not a `(type, fn)` pair. |
| `dispatchEvent(event)` | `Element.DispatchEvent(ev)` | Lives on `Element`, not `Document` — spec's other `EventTarget` implementer, `Document` itself, is reached here via `doc.DocumentElement().DispatchEvent(ev)` rather than a second method. Runs listeners through the same capture/target/bubble engine every built-in `Dispatch*` uses, but fires no default action, even for a built-in-named type like `"click"` — see `COMPATIBILITY.md`. |
| `write()` / `writeln()` / `open()` / `close()` | N/A | Legacy streaming-parser API; `ParseDocument` is parse-once instead. |
| `execCommand()` | N/A | Legacy rich-text-editing API; no editable-content model exists. |

### Also available (no direct spec equivalent)

Terminal and interactive-layer additions with no `Document` spec counterpart.
See `COMPATIBILITY.md` for the narrative, and `docs/SCROLLING.md` and
`docs/REPAINT.md` for design rationale:

- `Render()` does a one-shot render to an ANSI string. The closest spec
  analogue is a browser's internal reflow and paint, which is never exposed
  to script at all.
- `SetSize` and `Size` are viewport dimensions in character cells. A spec
  equivalent would be `window.innerWidth`/`innerHeight`, but those hang off
  `window`, not `document`.
- `ContentOffset(el)` and `ContentOffsetX(el)` are the row and column shifts
  from an element's own `Rect` (the CSS *border* box) to its first content cell,
  i.e. its border-top/padding-top and border-left/padding-left. No spec
  equivalent: a browser hands out `clientTop`/`clientLeft` in pixels, and a
  host here needs cells to place a caret inside a bordered, padded
  `<textarea>`.
- `DispatchClick`, `DispatchWheel`, `DispatchKey`, and `DispatchResize`
  synthesize
  the one fixed built-in event of each kind and run its default action; see
  `COMPATIBILITY.md`'s "Only one click kind exists" deviation.
  `DispatchKeyUp` is `DispatchKey`'s counterpart for a key release: it
  synthesizes `"keyup"` but runs no default action of its own, since nothing
  in this package is gated on a release. A host can only call it when its own
  key source distinguishes a release from a press; `tui.Loop` does, but only
  under a terminal that negotiated the Kitty or Win32 keyboard protocol. See
  `COMPATIBILITY.md`'s `keyup`/`keypress` entry.
  `DispatchClick` on a focusable text entry also focuses it and positions
  its caret at the clicked rune (Shift held extends the existing selection
  instead), folded into "click" for the same "only one click kind" reason,
  mirroring a real browser's separate mousedown-driven focus/caret
  placement. See `docs/proposals/CARET_SELECTION.md`. A click that hits a
  `<label>` is redirected to the control it names (via `for` or a nested
  labelable descendant) before any of that runs, as a single event targeted
  at the control, not the two-event label-then-control sequence a real
  browser fires; see `COMPATIBILITY.md`'s label deviation.
- `DispatchPaste(text)` and `DispatchCut()` synthesize `"paste"` and `"cut"`
  with
  `Event.ClipboardData` pre-populated (from `text` for paste, from the
  selected range, or the focused field's whole value if the selection is
  collapsed, for cut), and run their default action (insert into, or
  remove from, a focused text-like field's value at the current selection).
  Real DOM never exposes a scriptable "fire a paste or cut" call like this,
  since those only ever come from an actual OS clipboard action, but `Document`
  has no OS clipboard access of its own, so a host (`tui.Loop`, on
  bracketed paste / Ctrl-X) calls these explicitly instead. See
  `docs/proposals/CARET_SELECTION.md`.
- `FocusNext` and `FocusPrev` do programmatic tab-order traversal. No spec
  equivalent (real browsers only move focus this way via actual Tab
  keypresses, not a scriptable method).
- `SetPreRendered(el, ansi)` splices raw pre-rendered ANSI text into the
  tree as an element's content, bypassing layout entirely; no spec
  equivalent (nothing in real HTML lets you inject already-painted pixels).
- `DocumentElement()`'s spec equivalent is `documentElement`, listed above;
  repeated here only because it's this package's sole capitalized-property-
  as-method naming choice worth flagging.

---

## `Element` / `Node`

| Spec method/property | htmlterm equivalent | Notes |
|---|---|---|
| `Node.nodeType` | Missing | No generic node-type discriminator; every `*Element` handle is assumed to wrap an element (or, for `CreateTextNode`'s result, a text node with limited method support). |
| `Node.nodeName` / `Element.tagName` | `TagName()` | Real DOM uppercases `tagName`; htmlterm returns it as parsed (lowercase for HTML). |
| `Node.nodeValue` | `NodeValue()` | Returns e's own text for a text/comment node, `""` for an element (matching spec's `null` there). |
| `Node.textContent` (getter) | `TextContent()` | |
| `Node.textContent` (setter) | `SetTextContent(text)` | Built on `ReplaceChildren` — an empty string removes all children rather than adding an empty text node, matching spec's algorithm exactly. |
| `Node.parentNode` / `parentElement` | `Parent()` | Spec distinguishes "any parent node" vs. "parent that is an element"; htmlterm only has the element-returning form. |
| `Node.childNodes` | `ChildNodes()` | Unlike the DOM's live `NodeList`, this is a snapshot slice, same as `Children()`; includes text nodes, unlike `Children()`. |
| `Node.firstChild` / `lastChild` | `FirstChild()` / `LastChild()` | May return a text-node-backed `*Element` with limited method support, matching spec's "may not be an element" semantics. |
| `Node.previousSibling` / `nextSibling` | `PreviousSibling()` / `NextSibling()` | Same text-node caveat as above. |
| `Node.ownerDocument` | `OwnerDocument()` | |
| `Node.appendChild(child)` | `AppendChild(child)` | |
| `Node.insertBefore(new, old)` | `InsertBefore(new, old)` | |
| `Node.removeChild(child)` | `RemoveChild(child)` | |
| `Node.replaceChild(new, old)` | `ReplaceChild(new, old)` | |
| `Node.cloneNode(deep)` | `CloneNode(deep)` | Deliberately drops htmlterm's reserved focus/select-popup state attributes on clone — see the doc comment in `element.go`. |
| `Node.contains(other)` | `Contains(other)` | |
| `Node.isSameNode` | `IsSameNode(other)` | |
| `Node.isEqualNode` | Missing | No deep structural (tag+attrs+children) comparison; `IsSameNode` covers identity, or write your own deep-equality walk. |
| `Node.normalize()` | Missing | No adjacent-text-node merging exists (or is needed, since htmlterm doesn't produce split text nodes the way DOM mutation APIs can). |
| `Node.compareDocumentPosition` | Missing | No bitmask tree-position comparison; `Contains()` covers the common "is this an ancestor" case. |
| `Element.id` | `ID()` / `SetID(v)` | |
| `Element.className` | `ClassName()` / `SetClassName(v)` | Raw string accessor; see `ClassList()` for the token-oriented view of the same attribute. |
| `Element.classList` | `ClassList()` | Returns a `*ClassList` with `Contains`/`Add`/`Remove`/`Toggle` — matches spec's `DOMTokenList` subset. |
| `Element.attributes` | Missing | No `NamedNodeMap`; `GetAttributeNames()` gives you the names, but there's still no single call returning name→value pairs — call `GetAttribute` per name. |
| `Element.getAttributeNames()` | `GetAttributeNames()` | |
| `Element.getAttribute(name)` | `GetAttribute(name)` | Returns `(string, bool)` instead of spec's `string \| null`. |
| `Element.setAttribute(name, v)` | `SetAttribute(name, v)` | |
| `Element.removeAttribute(name)` | `RemoveAttribute(name)` | |
| `Element.hasAttribute(name)` | `HasAttribute(name)` | |
| `Element.hasAttributes()` | `HasAttributes()` | |
| `Element.dataset` | `Dataset()` | Returns a `*Dataset` with `Get`/`Set`/`Has`/`Delete`, keyed by the same camelCase name the DOM's `dataset` itself uses (e.g. `"fooBar"` for the `data-foo-bar` attribute) — not a live property bag, each call reads/writes the backing attribute directly. |
| `Element.children` | `Children()` | |
| `Element.childElementCount` | `ChildElementCount()` | |
| `Element.firstElementChild` / `lastElementChild` | `FirstElementChild()` / `LastElementChild()` | |
| `Element.previousElementSibling` / `nextElementSibling` | `PreviousElementSibling()` / `NextElementSibling()` | |
| `Element.matches(sel)` | `Matches(sel)` | |
| `Element.closest(sel)` | `Closest(sel)` | |
| `Element.querySelector(sel)` / `querySelectorAll(sel)` | `QuerySelector(sel)` / `QuerySelectorAll(sel)` | Scoped to e's subtree; e itself is never matched, matching spec. |
| `Element.getElementsByClassName` / `getElementsByTagName` | `GetElementsByClassName(cls)` / `GetElementsByTagName(tag)` | Scoped to e's subtree, same as `QuerySelectorAll`. |
| `Element.insertAdjacentElement` | `InsertAdjacentElement(position, newEl)` | `position` is one of `"beforebegin"`/`"afterbegin"`/`"beforeend"`/`"afterend"`, same as spec; an invalid value returns an `error` instead of spec's `SyntaxError` exception. |
| `Element.insertAdjacentText` | `InsertAdjacentText(position, text)` | Same `position` values as `InsertAdjacentElement`. |
| `Element.insertAdjacentHTML` | Missing | No HTML-*string* adjacent insertion — `InsertAdjacentElement`/`InsertAdjacentText` cover the element/text-argument cases above, but there's no fragment-parsing equivalent; `SetInnerHTML` only replaces a container's entire contents, not an adjacent position. |
| `Element.remove()` (`ChildNode`) | `Remove()` | |
| `Element.before()` / `after()` (`ChildNode`) | `Before(newSibling)` / `After(newSibling)` | |
| `Element.replaceWith()` (`ChildNode`) | `ReplaceWith(newSibling)` | |
| `Element.replaceChildren(...)` | `ReplaceChildren(newChildren...)` | |
| `Element.innerHTML` (getter) | `InnerHTML()` | Returns `(string, error)`; serializes via `golang.org/x/net/html`'s own `Render`, so escaping/void-element/raw-text-element handling matches how the same library would re-parse the result. Strips the reserved focus/select-popup marker attributes first, same as `CloneNode`. |
| `Element.innerHTML` (setter) | `SetInnerHTML(html)` | |
| `Element.outerHTML` (getter) | `OuterHTML()` | Same fidelity/stripping as `InnerHTML`, including e's own tag. |
| `Element.outerHTML` (setter) | Missing | Unlike the getters, this is a real feature, not a thin wrapper: it needs `html.ParseFragment` with e's *parent* as context (so context-sensitive fragments like a bare `<td>` parse correctly), then splicing possibly-multiple result nodes in where e was. Scoped separately from the getters; not implemented yet. |
| `Element.scrollIntoView()` | `ScrollVisible()` | Reports whether e is currently visible within its scrollable ancestors rather than actively scrolling it into view — see "Also available" below. |
| `Element.getBoundingClientRect()` | `Rect()` | Terminal cells (row/col/width/height), not pixels — see `COMPATIBILITY.md`. |
| `Element.getClientRects()` | Missing | No multi-rect (line-fragment) geometry; `Rect()` only returns a single box. |
| `Element.style` | `Style()` | Returns a `*Style` with `GetPropertyValue`/`SetProperty`/`RemoveProperty`/`CSSText`/`SetCSSText`, mirroring `CSSStyleDeclaration` for inline styles only. Two gaps versus spec, both documented on the `Style` type: shorthand properties don't round-trip (`SetProperty("margin", "1px")` stores only the expanded longhands — `GetPropertyValue("margin")` afterward returns `""`), and `CSSText`/`SetCSSText` don't preserve declaration order (serialized sorted by property name instead). |
| `window.getComputedStyle(el)` | Missing | No way to read the resolved cascade result at all from outside `internal/render`. |
| `HTMLElement.focus()` / `blur()` | `Focus()` / `Blur()` | |
| `HTMLElement.click()` | `Click()` | Synthesizes the click at e's own `Rect()` (top-left cell) and dispatches through the normal `DispatchClick` path; returns `false` if e has no recorded `Rect` (e.g. `display:none`, or `Render` hasn't run yet). |
| `HTMLInputElement.value` / `checked` | `Value()`/`SetValue()`, `Checked()`/`SetChecked()` | Attribute-backed, not a separate live property — see `COMPATIBILITY.md`'s "Form controls are attribute-driven" deviation. `SetValue` does apply spec's newline sanitization for single-line controls (CR/LF stripped for everything but `<textarea>`); `SetAttribute("value", ...)` is the unsanitized escape hatch. |
| `HTMLInputElement.selectionStart` / `selectionEnd` / `selectionDirection` | `SelectionStart()` / `SelectionEnd()` / `SelectionDirection()` | Rune offsets into `Value()`, not UTF-16 code units. Unlike `value`, this *is* live property state, not a reflected attribute — matching real spec's own split (`Element.SetValue` collapses it to the end, same as a real programmatic `.value` assignment). Defaults to a caret collapsed at the end of the value if `SetSelectionRange` has never been called, matching this package's original append-at-end typing behavior. See `docs/proposals/CARET_SELECTION.md`. |
| `HTMLInputElement.setSelectionRange(start, end, direction)` | `SetSelectionRange(start, end, direction...)` | `direction` is variadic (0 or 1 args) rather than a required third parameter; an omitted or unrecognized direction normalizes to `"none"`. `DispatchKey`'s arrow/Home/End/Backspace/Delete/typing default actions, and `DispatchClick`'s click-to-caret default action (with Shift extending), all go through this same state — see `docs/proposals/CARET_SELECTION.md`. |
| `HTMLElement.hidden` | `Hidden()` / `SetHidden(v)` | Attribute-presence wrapper, same shape as `Checked`/`SetChecked`; the UA stylesheet already maps a present `hidden` attribute to `display:none` (see COMPATIBILITY.md). |
| `HTMLDetailsElement.open` | `Open()` / `SetOpen(v)` | Attribute-presence wrapper, same shape as `Hidden`/`SetHidden`. Clicking, or pressing Enter or Space while focused on, the `<details>`'s `<summary>` toggles it and fires a non-bubbling, non-cancelable `"toggle"` event on the `<details>`, matching `HTMLDetailsElement`'s own event. See CSS.md's `details`/`summary` entries for the collapse/marker rendering this drives. |
| `HTMLDialogElement.show()` / `showModal()` | `Show()` / `ShowModal()` | Both return `false`, changing nothing, for a non-`<dialog>` or an already-open dialog, rather than throwing as spec does; close it first to switch modes. `ShowModal()` additionally marks the dialog `:modal`, which the UA stylesheet turns into `position: fixed` + a maximal `z-index` + auto-margin centering, traps focus and clicks inside it, and focuses its first focusable descendant (dropping focus when it has none, where spec would focus the `<dialog>` itself). See `docs/DIALOG.md`. |
| `HTMLDialogElement.close(returnValue)` | `Close()` / `CloseWith(rv)` | Two methods rather than one optional argument, matching this package's other value-taking setters. Both fire a non-bubbling, non-cancelable `"close"` event after the state has settled, and return `false` for an already-closed dialog. Escape on a modal fires a cancelable `"cancel"` first; preventing it suppresses the close. No `requestClose()` equivalent. |
| `HTMLDialogElement.returnValue` | `ReturnValue()` / `SetReturnValue(rv)` | Stored in a reserved attribute, stripped from `OuterHTML`/`CloneNode`, since this package has no property layer separate from attributes (see COMPATIBILITY.md's "Form controls are attribute-driven"). A `<form method="dialog">` submission sets it from the submitting control's `value`. |
| `HTMLElement.tabIndex` | Missing (typed accessor) | `tabindex` itself *is* read (drives focus order — see COMPATIBILITY.md's DOM & Events "At a Glance" and "Deviations from Spec"); there's just no typed `Element.TabIndex()`/`SetTabIndex()` shortcut — use `GetAttribute`/`SetAttribute("tabindex", ...)` directly, same as the rest of this row. |
| `HTMLElement.autofocus` | Missing (typed accessor) | `autofocus` itself *is* read, once, by `ParseDocument` (see COMPATIBILITY.md's "`autofocus` applies once, at `ParseDocument`" deviation); there's no typed `Element.Autofocus()`/`SetAutofocus()` shortcut and, unlike `tabindex`, setting the attribute afterward has no effect at all, since there's no later moment `ParseDocument`'s one-time walk could react to. |
| `HTMLElement.title`, etc. (remaining typed property shortcuts) | Missing | Use `GetAttribute`/`SetAttribute` with the matching attribute name. |
| `EventTarget.addEventListener` / `removeEventListener` | `Document.AddEventListener(el, ...)` / `Document.RemoveEventListener(handle)` | Registration lives on `Document`, not `Element` — see "Also available" below and the `Document` table above. |
| `EventTarget.dispatchEvent` | `Element.DispatchEvent(ev)` | Unlike registration, dispatch *does* live on `Element` here, matching spec's `target.dispatchEvent(...)` shape — pair with `NewCustomEvent`/`CustomEventInit` to build `ev`. See the `Document` table's `dispatchEvent` row above for how `Document`-wide dispatch is covered without a second method. |

### Also available (no direct spec equivalent)

- `IsTextEntry()` reports whether an element is a text-like `<input>` or
  `<textarea>`. It is used by both `DispatchKey`'s default actions and a
  host's own cursor-placement logic. No spec equivalent (real DOM has no single
  predicate for "this is a text-entry control").
- `Document.AddEventListener(el, typ, capture, fn)` /
  `Document.RemoveEventListener(handle)`. Spec's `EventTarget` interface
  puts these directly on the node; htmlterm centralizes listener bookkeeping
  on `Document` instead, keyed off a `ListenerHandle` rather than a
  `(type, listener)` pair (which also means, unlike spec, the *same*
  `(type, fn)` pair registered twice yields two independent listeners, not
  one de-duplicated registration). Mutating an element's listeners from
  inside one of its own running listeners behaves as spec requires: each
  node's list is snapshotted before it's walked, so a listener added during
  dispatch isn't called for that same event, and one removed during dispatch
  doesn't run even though it was in the snapshot.
- `ScrollTop()`/`SetScrollTop(v)` and `ScrollLeft()`/`SetScrollLeft(v)` are
  spec's `Element.scrollTop` and `scrollLeft` properties, matching spec
  placement exactly.
- `ScrollVisible()` has no direct spec equivalent; see the `scrollIntoView()`
  row above for how it differs from the real thing.

---

## See Also

- **COMPATIBILITY.md**, for the narrative deviations, additions, and
  not-supported breakdown this table is derived from.
- **docs/INTERACTIVE.md**, for design rationale on the shape of the
  `Document`, `Element`, and event model.
- **docs/SCROLLING.md**, for scrolling-specific API design (`ScrollTop`/
  `ScrollLeft`/`ScrollVisible`).
