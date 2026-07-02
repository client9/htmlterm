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
- vulnerabilities in dependencies that affect htmlterm's rendering path

General rendering bugs without security impact may be reported as public issues.
