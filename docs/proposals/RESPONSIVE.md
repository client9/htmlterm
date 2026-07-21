# Responsive styling (size/theme breakpoints) — design spec

Status: **not started, not urgent — revisit later.** This document records a
decision made during design discussion so the reasoning doesn't need to be
re-derived next time this comes up.

## Problem

Consumers may want htmlterm's rendered output to adapt to:

- **Size** — different layout/CSS below/above some terminal width (a
  "responsive" breakpoint, the terminal-column analog of CSS `@media
  (min-width/max-width: ...)`).
- **Color scheme** — dark vs. light (or "no preference") variants of colors,
  the analog of CSS `@media (prefers-color-scheme: ...)`.

Real CSS solves both with `@media` at-rules inside a stylesheet. The question
was whether htmlterm should implement `@media` in `internal/cssengine`, or
push the whole problem to the host application (in practice, `tui.Loop`).

## Decision

**Do not implement `@media` in `internal/cssengine`.** Instead, let the host
(typically `tui.Loop`, which is the only layer that legitimately knows real
terminal facts — see `SizeAutomatic`'s terminal-query resolution in
`htmlterm.go`) decide *which pre-authored stylesheet strings* to hand to
`Options.Stylesheets` in response to a resize or theme-change event, and
re-render. No new CSS grammar, no changes to `cssengine`'s parser, `Rule`, or
`Cascade` are required.

This works because `Options.Stylesheets` (`htmlterm.go`) is already a list of
strings layered on top of `Options.CSS` in cascade order, one per "sheet" —
exactly the shape needed to conditionally include (or omit) a `narrow.css` /
`wide.css` or `dark.css` / `light.css` variant. A breakpoint becomes an
ordinary Go conditional selecting list membership, not a CSS-level construct
evaluated per-node.

### Why not implement real `@media`

- `internal/cssengine.ParseStylesheet` (css.go) is a **flat, single-level**
  hand-written lexer state machine (`inSelector` / `inDeclarations`) with no
  brace-depth tracking at all. Supporting `@media { ... }` would require a
  genuine rewrite of that loop (nesting-aware, at-rule-aware), not an
  incremental add.
- `Cascade`/`matchSelector`/`Rule` would all need a condition-evaluation
  concept threaded through them, plus a `MediaContext` (width, color scheme)
  plumbed from `Options` down through `render.Engine` into
  `DocumentRules`/`Request` — touching several invariants documented in
  `Rule`'s and `Document.cachedEngine`'s doc comments around cached/reused
  parsed selectors and rule sets.
- There is no reliable, portable way to *detect* "dark vs. light" from a real
  terminal (OSC 11 background queries are blocking, slow, and unsupported in
  many emulators/multiplexers) — so even a full `@media
  (prefers-color-scheme)` implementation would still need an explicit
  host-supplied signal, same as the simpler approach below. Implementing the
  CSS grammar buys nothing on that front.
- The project's existing philosophy (see `CSS.md`) is a deliberately
  restricted CSS subset, not a full implementation — host-layer breakpoints
  fit that philosophy better than adding conditional-rule grammar.

## What this requires (when picked back up)

### 1. A way to change a live `Document`'s stylesheets after construction

Today, per the doc comment on `Document.cachedEngine`/`cachedRules`
(`document/document.go`), there is deliberately **no setter** for
`Options.CSS`/`Stylesheets` after `ParseDocument` — those fields are assumed
immutable so the parsed rule set can be cached for the Document's whole
lifetime (avoiding re-lexing the UA stylesheet + `Options.CSS` on every
frame).

This needs a new method mirroring `SetSize`'s existing pattern:

```go
// SetStylesheets updates the stylesheet list the next Render call cascades
// against, without discarding the parsed tree — same spirit as SetSize, but
// for CSS instead of layout size.
func (d *Document) SetStylesheets(css string, stylesheets []string)
```

which updates `d.opts.CSS`/`d.opts.Stylesheets` and invalidates
`cachedEngine`/`cachedRules` (forcing the next `Render` to rebuild them),
exactly the way `SetSize` invalidates layout inputs today. This is the one
actual code change this design needs — small and contained, unlike an
`@media` implementation.

### 2. Signal sources, owned by the host

- **Width breakpoints**: `tui.Loop` already receives resize events and
  already calls `Document.SetSize`. It would additionally recompute which
  stylesheet(s) apply for the new width and call `SetStylesheets` before the
  next repaint.
- **Color scheme**: no such event exists yet ("event TBD"). When/if it's
  added, it should be an explicit signal the host decides to feed in (e.g. an
  OS-level hint the embedding application already tracks, a config file, a
  keybinding to toggle, or — if someone wants to gamble on OSC 11 querying —
  an opt-in `tui.Loop` feature). htmlterm itself should never attempt
  terminal-based autodetection; that decision stays entirely at the host
  layer, same as today's `Options.Profile`/`colorprofile.Detect` pattern.

### 3. Non-goals

- No `@media` CSS syntax, no boolean combinators (`and`/`or`/`not`), no
  `aspect-ratio`/`orientation`/px-based conditions.
- No terminal-based dark/light autodetection inside htmlterm.
- No dynamic per-node condition evaluation inside `Cascade` — breakpoint
  selection happens once, at the Go level, before a `Render` call, not during
  cascade resolution.

## Separate, lower-priority item: parser robustness

Independent of this decision: since htmlterm explicitly supports rendering
untrusted HTML (`example_test.go`'s "New with untrusted HTML" case),
`<style>` content containing a real `@media { ... }` block (or any other
unrecognized at-rule) will currently be **misparsed** by `ParseStylesheet` —
the nested ruleset's inner `{`/`}` and property-like tokens get read as if
they were a single flat rule's selector/declarations, potentially corrupting
parsing of the rest of that stylesheet.

Worth fixing on its own, independent of whether host-layer breakpoints or a
real `@media` implementation is ever pursued: teach `ParseStylesheet` to
recognize `@ident ... { ... }` and skip it as a balanced-brace block (nested
braces counted, not assumed single-level), so unsupported at-rules are
silently ignored rather than corrupting subsequent rules. Small, contained,
defensive — not a stepping stone to full `@media` support.

## Summary

| Concern | Approach |
|---|---|
| Width breakpoints | Host (`tui.Loop`) computes which stylesheet(s) apply on resize, calls new `Document.SetStylesheets` |
| Color scheme | Host supplies an explicit signal (source TBD); no terminal autodetection in htmlterm |
| `cssengine` changes | None |
| `Document` API changes | One new method: `SetStylesheets` (+ cache invalidation, mirroring `SetSize`) |
| Parser hardening | Separate, optional: make `ParseStylesheet` skip unknown at-rule blocks instead of misparsing them |
