// Package htmlterm renders a restricted subset of HTML and CSS to ANSI-styled
// terminal strings.
//
// It is intended for CLIs, terminal UIs, and text-first applications that need
// richer formatting than plain text without embedding a browser engine.
//
// The renderer supports HTML fragments or full documents, a focused CSS subset,
// inherited inline styles, block layout, lists, tables, pseudo-elements, and
// OSC 8 hyperlinks. Unsupported HTML and CSS are ignored rather than treated as
// errors. See CSS.md in the repository for the supported surface.
//
// # Security
//
// htmlterm is not a browser sandbox or a general-purpose HTML sanitizer. It
// renders to terminal output, where control sequences can affect display state
// or create misleading hyperlinks. Document-provided text, CSS string content,
// generated attribute content, custom markers, and hyperlink URLs are sanitized
// before output. Renderer-generated ANSI styling and OSC 8 wrappers may still
// be emitted intentionally.
//
// When rendering attacker-controlled documents, consider setting
// Options.NoOSC8Links and applying application-level URL and size limits. See
// SECURITY.md in the repository for the full security policy and threat model.
package htmlterm
