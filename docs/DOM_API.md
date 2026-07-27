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
| `querySelector(sel)` | `QuerySelector(sel)` | Whole-document search only — see `querySelector` under Element for the missing scoped/subtree version. |
| `querySelectorAll(sel)` | `QuerySelectorAll(sel)` | Same scoping caveat as above. |
| `getElementsByClassName(cls)` | Missing | Use `QuerySelectorAll(".cls")` instead. |
| `getElementsByTagName(tag)` | Missing | Use `QuerySelectorAll("tag")` instead. |
| `getElementsByName(name)` | Missing | Use `QuerySelectorAll("[name=...]")` instead. |
| `createElement(tag)` | `CreateElement(tag)` | |
| `createTextNode(text)` | `CreateTextNode(text)` | |
| `createDocumentFragment()` | Missing | No `DocumentFragment` type; build a detached `Element` subtree via `CreateElement`/`AppendChild` and attach it directly instead. |
| `createComment(text)` | Missing | No comment-node support at all — comments are dropped during parsing. |
| `createEvent()` / `Event` constructor | N/A | Events are only ever synthesized internally by `Dispatch*` calls; there's no way to construct and dispatch an arbitrary custom event. |
| `importNode(node, deep)` / `adoptNode(node)` | N/A | Single-document model — no cross-document node transfer exists to import/adopt between. |
| `documentElement` | `DocumentElement()` | |
| `body` | Missing | No dedicated accessor; use `QuerySelector("body")`. |
| `head` | Missing | Use `QuerySelector("head")`. |
| `title` | Missing | No `<title>`-reading convenience; use `QuerySelector("title").TextContent()`. |
| `activeElement` | `FocusedElement()` | |
| `forms` / `images` / `links` / `scripts` / `styleSheets` | Missing | Use `QuerySelectorAll` with the relevant selector instead. |
| `URL` / `domain` / `cookie` / `location` / `defaultView` / `readyState` / `characterSet` / `doctype` | N/A | No network origin, no window, no navigation, no async loading — none of these concepts exist for a one-shot/embedded document. |
| `addEventListener(type, fn, opts)` (via `EventTarget`) | `AddEventListener(el, typ, capture, fn)` | Lives on `Document`, not `Element` — takes the target `*Element` as a parameter rather than being called on it. See "Also available" under Element below. |
| `removeEventListener` | `RemoveEventListener(handle)` | Takes the `ListenerHandle` returned by `AddEventListener`, not a `(type, fn)` pair. |
| `dispatchEvent(event)` | Missing (public) | Internal `dispatch` exists but isn't exported; only the fixed built-in event types (`DispatchClick`/`DispatchKey`/`DispatchWheel`/`DispatchResize`) can be fired — no arbitrary custom-event dispatch. |
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
- `ScrollTop`/`SetScrollTop`, `ScrollLeft`/`SetScrollLeft` — spec has these
  as `Element.scrollTop`/`scrollLeft` properties on the element itself;
  htmlterm puts them on `Document`, taking the element as a parameter (see
  Element's "Also available" section).
- `DispatchClick`/`DispatchWheel`/`DispatchKey`/`DispatchResize` — synthesize
  the one fixed built-in event of each kind and run its default action; see
  `COMPATIBILITY.md`'s "Only one click kind exists" deviation.
- `FocusNext`/`FocusPrev` — programmatic tab-order traversal; no spec
  equivalent (real browsers only move focus this way via actual Tab
  keypresses, not a scriptable method).
- `SetInnerHTML(el, html)` — spec's `Element.innerHTML` setter, but placed
  on `Document` and taking the target element as a parameter instead of
  being a property on the element.
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
| `Node.nodeValue` | Missing | Only meaningful for text/comment nodes, which htmlterm barely models; use `TextContent()`. |
| `Node.textContent` (getter) | `TextContent()` | |
| `Node.textContent` (setter) | Missing | No way to replace all children with a single text node in one call; use `RemoveChild` in a loop (or `SetInnerHTML` at the `Document` level) plus `AppendChild(doc.CreateTextNode(...))`. |
| `Node.parentNode` / `parentElement` | `Parent()` | Spec distinguishes "any parent node" vs. "parent that is an element"; htmlterm only has the element-returning form. |
| `Node.childNodes` | Missing | No `NodeList` including text nodes; only `Children()` (elements only). |
| `Node.firstChild` / `lastChild` | `FirstChild()` / `LastChild()` | May return a text-node-backed `*Element` with limited method support, matching spec's "may not be an element" semantics. |
| `Node.previousSibling` / `nextSibling` | `PreviousSibling()` / `NextSibling()` | Same text-node caveat as above. |
| `Node.ownerDocument` | Missing | An `Element` carries its owning `*Document` internally (used by `Focus`/`Rect`/etc.) but doesn't expose it as a gettable property. |
| `Node.appendChild(child)` | `AppendChild(child)` | |
| `Node.insertBefore(new, old)` | `InsertBefore(new, old)` | |
| `Node.removeChild(child)` | `RemoveChild(child)` | |
| `Node.replaceChild(new, old)` | `ReplaceChild(new, old)` | |
| `Node.cloneNode(deep)` | `CloneNode(deep)` | Deliberately drops htmlterm's reserved focus/select-popup state attributes on clone — see the doc comment in `element.go`. |
| `Node.contains(other)` | `Contains(other)` | |
| `Node.isEqualNode` / `isSameNode` | Missing | No node-identity/deep-equality comparison; compare `*Element` pointers directly for identity (works, since handles for the same node are interchangeable), or write your own deep-equality walk. |
| `Node.normalize()` | Missing | No adjacent-text-node merging exists (or is needed, since htmlterm doesn't produce split text nodes the way DOM mutation APIs can). |
| `Node.compareDocumentPosition` | Missing | No bitmask tree-position comparison; `Contains()` covers the common "is this an ancestor" case. |
| `Element.id` | `ID()` (getter only) | No `SetID`; use `SetAttribute("id", v")`. |
| `Element.className` | Missing | No string-shorthand accessor for the `class` attribute; use `ClassList()`, or `GetAttribute("class")`/`SetAttribute("class", v)` directly. |
| `Element.classList` | `ClassList()` | Returns a `*ClassList` with `Contains`/`Add`/`Remove`/`Toggle` — matches spec's `DOMTokenList` subset. |
| `Element.attributes` | Missing | No `NamedNodeMap`; no way to enumerate all attributes on an element, only look one up by name. |
| `Element.getAttributeNames()` | Missing | Same gap as above. |
| `Element.getAttribute(name)` | `GetAttribute(name)` | Returns `(string, bool)` instead of spec's `string \| null`. |
| `Element.setAttribute(name, v)` | `SetAttribute(name, v)` | |
| `Element.removeAttribute(name)` | `RemoveAttribute(name)` | |
| `Element.hasAttribute(name)` | `HasAttribute(name)` | |
| `Element.hasAttributes()` | Missing | Follows from the missing `attributes`/`getAttributeNames`. |
| `Element.dataset` | Missing | No structured view of `data-*` attributes; use `GetAttribute("data-foo")`/`SetAttribute("data-foo", v)` directly. |
| `Element.children` | `Children()` | |
| `Element.childElementCount` | Missing | Use `len(Children())`. |
| `Element.firstElementChild` / `lastElementChild` | `FirstElementChild()` / `LastElementChild()` | |
| `Element.previousElementSibling` / `nextElementSibling` | `PreviousElementSibling()` / `NextElementSibling()` | |
| `Element.matches(sel)` | `Matches(sel)` | |
| `Element.closest(sel)` | `Closest(sel)` | |
| `Element.querySelector(sel)` / `querySelectorAll(sel)` | Missing | Only `Document.QuerySelector(All)` exists, always searching the whole document — no way to scope a search to a subtree rooted at a given element. Workaround: walk `Children()` recursively and test `Matches`/build the filter yourself. |
| `Element.getElementsByClassName` / `getElementsByTagName` | Missing | Same scoping gap as above. |
| `Element.insertAdjacentElement` / `insertAdjacentHTML` / `insertAdjacentText` | Missing | No adjacent-position insertion helpers; use `Parent().InsertBefore(...)` (with `NextSibling()` for "after") for elements, or `Document.SetInnerHTML` for HTML-string insertion (container-replacement only, not adjacent). |
| `Element.remove()` (`ChildNode`) | Missing | No self-detach shortcut; use `e.Parent().RemoveChild(e)`. |
| `Element.before()` / `after()` (`ChildNode`) | Missing | Use `Parent().InsertBefore(newEl, e)` / `Parent().InsertBefore(newEl, e.NextSibling())`. |
| `Element.replaceWith()` (`ChildNode`) | Missing | Use `Parent().ReplaceChild(newEl, e)`. |
| `Element.replaceChildren(...)` | Missing | No bulk-replace-all-children helper; remove each child in a loop then `AppendChild` the new ones, or use `Document.SetInnerHTML`. |
| `Element.innerHTML` (getter) | Missing | No HTML-string serialization of an element's contents in either direction. |
| `Element.innerHTML` (setter) | `Document.SetInnerHTML(el, html)` | Lives on `Document`, taking the element as a parameter, not a property on the element itself. |
| `Element.outerHTML` | Missing | No serialization including the element's own tag either. |
| `Element.scrollIntoView()` | `Document.ScrollVisible(el)` | Same Document-vs-Element placement difference as `innerHTML`/`scrollTop`. |
| `Element.getBoundingClientRect()` | `Rect()` | Terminal cells (row/col/width/height), not pixels — see `COMPATIBILITY.md`. |
| `Element.getClientRects()` | Missing | No multi-rect (line-fragment) geometry; `Rect()` only returns a single box. |
| `Element.style` | Missing | No `CSSStyleDeclaration`; only the raw `style` attribute string via `GetAttribute`/`SetAttribute`. See the earlier discussion in this conversation — a `ClassList`-shaped `Style()` handle wrapping `internal/cssengine`'s inline-style parser would be the natural fit. |
| `window.getComputedStyle(el)` | Missing | No way to read the resolved cascade result at all from outside `internal/render`. |
| `HTMLElement.focus()` / `blur()` | `Focus()` / `Blur()` | |
| `HTMLElement.click()` | Missing | No programmatic-click-without-coordinates method; `Document.DispatchClick` always requires a row/col to hit-test against. |
| `HTMLInputElement.value` / `checked` | `Value()`/`SetValue()`, `Checked()`/`SetChecked()` | Attribute-backed, not a separate live property — see `COMPATIBILITY.md`'s "Form controls are attribute-driven" deviation. |
| `HTMLElement.dataset`, `.hidden`, `.tabIndex`, `.title`, etc. (typed property shortcuts) | Missing | None of `HTMLElement`'s typed convenience properties exist; use `GetAttribute`/`SetAttribute` with the matching attribute name for all of them. |
| `EventTarget.addEventListener` / `removeEventListener` / `dispatchEvent` | `Document.AddEventListener(el, ...)` / `Document.RemoveEventListener(handle)` | Lives on `Document`, not `Element` — see "Also available" below and the `Document` table above. No public `dispatchEvent`. |

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
- `Document.ScrollTop(el)`/`SetScrollTop(el, v)`,
  `Document.ScrollLeft(el)`/`SetScrollLeft(el, v)` — spec's
  `Element.scrollTop`/`scrollLeft` properties, but on `Document` with the
  element passed in, matching the same pattern as `SetInnerHTML`/
  `scrollIntoView` above.

---

## See Also

- **COMPATIBILITY.md** — the narrative deviations/additions/not-supported
  breakdown this table is derived from.
- **docs/INTERACTIVE.md** — design rationale for the `Document`/`Element`/
  event model's shape.
- **docs/SCROLLING.md** — scrolling-specific API design (`ScrollTop`/
  `ScrollLeft`/`ScrollVisible`).
