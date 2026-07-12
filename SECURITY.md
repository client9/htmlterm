# Security Policy

## Supported Versions

Security fixes are made on the main branch. If versioned releases are published,
the latest release line is supported unless otherwise noted in the release notes.

## Reporting a Vulnerability

Please do not open a public issue for a security vulnerability.

Report vulnerabilities using GitHub's private vulnerability reporting or by
contacting the maintainer privately. Include enough detail to reproduce and
assess the issue:

- the affected version, tag, or commit
- a minimal HTML/CSS input that demonstrates the issue
- the terminal output produced and the output you expected
- terminal emulator, shell, or multiplexer details if terminal behavior matters
- whether the input is trusted or attacker-controlled in your use case

## Security Model

htmlterm renders a restricted subset of HTML and CSS to terminal strings. It is
not a browser, HTML sandbox, sanitizer for web output, or policy engine.

The primary security boundary is terminal output safety. Terminals interpret
control sequences, including ANSI CSI sequences and OSC sequences such as OSC 8
hyperlinks. If attacker-controlled HTML or CSS can emit raw terminal controls,
the rendered output may affect terminal state, create misleading links, alter
displayed text, or trigger terminal-specific behavior.

htmlterm treats document-provided HTML/CSS text as untrusted terminal content
and strips terminal escape sequences and control characters from:

- text nodes
- CSS string content, including `content: "..."` pseudo-element output
- HTML attributes inserted via CSS `attr(...)`
- custom list marker strings
- custom `text-overflow` marker strings
- OSC 8 hyperlink URLs from `<a href="...">`

Renderer-generated formatting, such as ANSI styling and OSC 8 hyperlink wrappers,
may still be emitted intentionally. To disable generated OSC 8 hyperlinks, set
`Options.NoOSC8Links` to `true`.

## Trusted Pre-Rendered Content

`Document.SetPreRendered(el, ansi)` is the one deliberate exception to "text
content is always sanitized." It inserts `ansi` verbatim into `el`, with no
escape-sequence stripping, no whitespace normalization, and no CSS-driven
styling — on the assumption that `ansi` is the output of a prior
`Document.Render()` or `Renderer.Render()` call (content this package already
rendered, and in doing so already sanitized, once) rather than
document-provided text.

This exists for hosts that re-render the same content on every repaint (e.g.
scroll-triggered redraws) and want to pay layout cost once: render a subtree
to a string, cache it, and re-embed the cached string on subsequent repaints
instead of re-parsing and re-laying-out the original HTML each time.
`SetInnerHTML`-embedding a raw ANSI string this way does *not* work — its
sanitizer strips the escape sequences that carry the styling, silently
degrading the output to unstyled text — which is exactly why
`SetPreRendered` exists as a distinct, explicit call instead.

The mechanism: `SetPreRendered` attaches `ansi` as a single
[`html.RawNode`](https://pkg.go.dev/golang.org/x/net/html#NodeType) child of
`el`. `html.Parse`/`html.ParseFragment` (and therefore `SetInnerHTML`, which
is built on `ParseFragment`) never produce a `RawNode` from any input —
per `golang.org/x/net/html`'s own doc comment, "RawNode nodes are not
returned by the parser." The render engine's inline-content walk sanitizes
every `TextNode` unconditionally but passes a `RawNode`'s content straight
through. So the trust boundary here is enforced by *which code path put the
node in the tree*, not by any property of the string itself or a
naming/wrapper convention a caller could accidentally defeat — the only way
a `RawNode` enters a `Document`'s tree is a direct `SetPreRendered` call.

That said, `SetPreRendered` performs **no validation** of its own — it
trusts the caller completely. It does not check that `ansi` is well-formed
ANSI, that it was actually produced by this package, or that its line widths
match `el`'s resolved content width (a caller re-using stale output after a
resize will see it clipped/padded to the new width, not rewrapped). Passing
it arbitrary or attacker-controlled text reintroduces exactly the
terminal-escape-injection risk the rest of this document describes —
`SetPreRendered` is an escape hatch for content this package already
rendered, not a general-purpose "trust me" API. Callers should treat it with
the same care as, e.g., Go's `html/template.HTML`: a real tool for a real
problem, and a real risk if fed the wrong input.

## Untrusted HTML and CSS

htmlterm may be used with untrusted HTML/CSS for terminal rendering, but callers
should still treat the resulting string as untrusted display data. Be careful
when forwarding rendered output to shells, logs, pagers, terminal multiplexers,
chat systems, or other programs that may interpret control characters, markup,
or hyperlinks differently.

Unsupported HTML and CSS are ignored rather than rejected. Do not rely on
htmlterm to enforce content policy, remove sensitive information, validate URLs,
or make unsafe HTML safe for any non-terminal context.

## Defense in Depth

Applications that render attacker-controlled documents should consider:

- setting `Options.NoOSC8Links` when terminal hyperlinks are not required
- constraining accepted URL schemes before rendering links
- limiting input size before parsing/rendering
- rendering in a non-interactive context when displaying hostile samples
- keeping terminal emulators and multiplexers up to date

## What to Report

Please report issues such as:

- attacker-controlled terminal escape sequences reaching output
- malformed OSC 8 hyperlinks or hyperlink-boundary injection
- crashes or excessive resource use from crafted HTML/CSS
- security-relevant differences between documented and actual sanitization
- a code path other than `Document.SetPreRendered` that lets a `RawNode` (or
  otherwise-unsanitized text) reach rendered output
- vulnerabilities in dependencies that affect htmlterm's rendering path

General rendering bugs without security impact may be reported as public issues.
