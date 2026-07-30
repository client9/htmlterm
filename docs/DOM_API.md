# DOM API Coverage

A field-by-field comparison of the real DOM's `Document` and `Element`/`Node`
interfaces against `document.Document` and `document.Element`. This is a
lookup table, not an explainer — see `COMPATIBILITY.md`'s "DOM & Events"
section for the narrative version (deviations, terminal-native additions,
what's deliberately out of scope) and `docs/INTERACTIVE.md` for the design
rationale.

Column 2 is one of: the htmlterm method/property name, **Missing** (real DOM
has it, htmlterm doesn't, no workaround), **Partial** (htmlterm has something
narrower or shaped differently), or **N/A** (doesn't apply — no window, no
network, no scripting engine).

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

Terminal/interactive-layer additions with no `Document` spec counterpart —
see `COMPATIBILITY.md` for the narrative and `docs/SCROLLING.md`/
`docs/REPAINT.md` for design rationale:

- `Render()` — one-shot render to an ANSI string; the closest spec analogue
  is a browser's internal reflow/paint, which is never exposed to script at
  all.
- `SetSize`/`Size` — viewport dimensions in character cells; a spec
  equivalent would be `window.innerWidth`/`innerHeight`, but those hang off
  `window`, not `document`.
- `ContentOffset(el)` — scroll-adjusted content-area offset for a scrollable
  ancestor.
- `DispatchClick`/`DispatchWheel`/`DispatchKey`/`DispatchResize` — synthesize
  the one fixed built-in event of each kind and run its default action; see
  `COMPATIBILITY.md`'s "Only one click kind exists" deviation.
- `DispatchPaste(text)`/`DispatchCut()` — synthesize `"paste"`/`"cut"` with
  `Event.ClipboardData` pre-populated (from `text` for paste, from the
  focused field's value for cut), and run their default action (insert into,
  or clear, a focused text-like field's value). Real DOM never exposes a
  scriptable "fire a paste/cut" call like this — those only ever come from
  an actual OS clipboard action — but `Document` has no OS clipboard access
  of its own, so a host (`tui.Loop`, on bracketed paste / Ctrl-X) calls
  these explicitly instead. Always whole-field, not selection-scoped — see
  `COMPATIBILITY.md`.
- `FocusNext`/`FocusPrev` — programmatic tab-order traversal; no spec
  equivalent (real browsers only move focus this way via actual Tab
  keypresses, not a scriptable method).
- `SetPreRendered(el, ansi)` — splices raw pre-rendered ANSI text into the
  tree as an element's content, bypassing layout entirely; no spec
  equivalent (nothing in real HTML lets you inject already-painted pixels).
- `DocumentElement()` — spec equivalent is `documentElement`, listed above;
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
| `HTMLInputElement.value` / `checked` | `Value()`/`SetValue()`, `Checked()`/`SetChecked()` | Attribute-backed, not a separate live property — see `COMPATIBILITY.md`'s "Form controls are attribute-driven" deviation. |
| `HTMLElement.hidden` | `Hidden()` / `SetHidden(v)` | Attribute-presence wrapper, same shape as `Checked`/`SetChecked`; the UA stylesheet already maps a present `hidden` attribute to `display:none` (see COMPATIBILITY.md). |
| `HTMLElement.tabIndex` | Missing (typed accessor) | `tabindex` itself *is* read (drives focus order — see COMPATIBILITY.md's DOM & Events "At a Glance" and "Deviations from Spec"); there's just no typed `Element.TabIndex()`/`SetTabIndex()` shortcut — use `GetAttribute`/`SetAttribute("tabindex", ...)` directly, same as the rest of this row. |
| `HTMLElement.title`, etc. (remaining typed property shortcuts) | Missing | Use `GetAttribute`/`SetAttribute` with the matching attribute name. |
| `EventTarget.addEventListener` / `removeEventListener` | `Document.AddEventListener(el, ...)` / `Document.RemoveEventListener(handle)` | Registration lives on `Document`, not `Element` — see "Also available" below and the `Document` table above. |
| `EventTarget.dispatchEvent` | `Element.DispatchEvent(ev)` | Unlike registration, dispatch *does* live on `Element` here, matching spec's `target.dispatchEvent(...)` shape — pair with `NewCustomEvent`/`CustomEventInit` to build `ev`. See the `Document` table's `dispatchEvent` row above for how `Document`-wide dispatch is covered without a second method. |

### Also available (no direct spec equivalent)

- `IsTextEntry()` — reports whether an element is a text-like `<input>`/
  `<textarea>`; used by both `DispatchKey`'s default actions and a host's
  own cursor-placement logic. No spec equivalent (real DOM has no single
  predicate for "this is a text-entry control").
- `Document.AddEventListener(el, typ, capture, fn)` /
  `Document.RemoveEventListener(handle)` — spec's `EventTarget` interface
  puts these directly on the node; htmlterm centralizes listener bookkeeping
  on `Document` instead, keyed off a `ListenerHandle` rather than a
  `(type, listener)` pair (which also means, unlike spec, the *same*
  `(type, fn)` pair registered twice yields two independent listeners, not
  one de-duplicated registration).
- `ScrollTop()`/`SetScrollTop(v)`, `ScrollLeft()`/`SetScrollLeft(v)` — spec's
  `Element.scrollTop`/`scrollLeft` properties, matching spec placement
  exactly.
- `ScrollVisible()` — no direct spec equivalent; see the `scrollIntoView()`
  row above for how it differs from the real thing.

---

## See Also

- **COMPATIBILITY.md** — the narrative deviations/additions/not-supported
  breakdown this table is derived from.
- **docs/INTERACTIVE.md** — design rationale for the `Document`/`Element`/
  event model's shape.
- **docs/SCROLLING.md** — scrolling-specific API design (`ScrollTop`/
  `ScrollLeft`/`ScrollVisible`).
