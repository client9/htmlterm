package render

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// TestWordWrapANSIStyleCarry verifies that a single SGR-styled run spanning
// multiple wrapped lines is closed before each line break and reopened at
// the start of the next line, so every emitted line is self-contained.
func TestWordWrapANSIStyleCarry(t *testing.T) {
	text := "\x1b[4m" + "alpha beta gamma delta epsilon" + "\x1b[m"
	got := wordWrapANSI(text, 12, "")
	want := []string{
		"\x1b[4malpha beta\x1b[m",
		"\x1b[4mgamma delta\x1b[m",
		"\x1b[4mepsilon\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI() = %#v, want %#v", got, want)
	}
}

// TestWrapHyperlinkMarginNotUnderlined is the wrap.html bug scenario: a
// hyperlink's underlined text wraps inside a block with left/right margin.
// The margin spaces on each wrapped line must fall outside the underline
// and hyperlink span, and every wrapped line must remain independently
// styled and clickable.
func TestWrapHyperlinkMarginNotUnderlined(t *testing.T) {
	r, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(`<p style="margin: 0 3">` +
		`<a href="https://x.test/">alpha beta gamma delta epsilon</a></p>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	osc, oscReset := "\x1b]8;;https://x.test/\x07", "\x1b]8;;\x07"
	wantContent := strings.Join([]string{
		osc + "\x1b[4malpha beta\x1b[m" + oscReset,
		osc + "\x1b[4mgamma delta\x1b[m" + oscReset,
		osc + "\x1b[4mepsilon\x1b[m" + oscReset,
	}, "\n")
	// margin: 0 3 sets margin-top and margin-bottom to 0 (CSS 2-value
	// shorthand), overriding the UA stylesheet's default p margin-bottom:1,
	// so there's no extra trailing blank line here.
	want := applyLineEdges(wantContent, "   ", "   ") + "\n"
	if out != want {
		t.Fatalf("Render() =\n%q\nwant:\n%q", out, want)
	}
}

// TestSplitAtVisualWidthCarryHardBreak verifies that a hard break
// (break-word) inside a single overlong styled token also closes and
// reopens the style at each internal cut point.
func TestSplitAtVisualWidthCarryHardBreak(t *testing.T) {
	text := "\x1b[1m" + "Supercalifragilisticexpialidocious" + "\x1b[m"
	got := wordWrapANSI(text, 10, "break-word")
	want := []string{
		"\x1b[1mSupercalif\x1b[m",
		"\x1b[1mragilistic\x1b[m",
		"\x1b[1mexpialidoc\x1b[m",
		"\x1b[1mious\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI(break-word) = %#v, want %#v", got, want)
	}
}

// TestWordWrapANSINoDoubleOpenOnCombinedBreak guards against a naive
// close+reopen implementation double-emitting the open sequence when a
// normal width-overflow break and a hard mid-token break both fire for the
// same token (an over-width word arriving right after already-packed words).
func TestWordWrapANSINoDoubleOpenOnCombinedBreak(t *testing.T) {
	text := "\x1b[4m" + "hi Supercalifragilisticexpialidocious" + "\x1b[m"
	got := wordWrapANSI(text, 10, "break-word")
	want := []string{
		"\x1b[4mhi\x1b[m",
		"\x1b[4mSupercalif\x1b[m",
		"\x1b[4mragilistic\x1b[m",
		"\x1b[4mexpialidoc\x1b[m",
		"\x1b[4mious\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI() = %#v, want %#v (check for doubled \\x1b[4m open codes)", got, want)
	}
}

// TestCollapseDeadANSISpans is a direct unit test of collapseDeadANSISpans,
// the final cleanup pass wordWrapTokens applies to every line it emits.
func TestCollapseDeadANSISpans(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no escapes at all",
			in:   "plain text",
			want: "plain text",
		},
		{
			name: "normal self-contained span is untouched",
			in:   "\x1b[3mregister\x1b[m",
			want: "\x1b[3mregister\x1b[m",
		},
		{
			name: "dangling SGR open immediately reset is dropped, reset kept",
			in:   "\x1b[3m\x1b[m",
			want: "\x1b[m",
		},
		{
			name: "dangling SGR open immediately reset with explicit 0 param",
			in:   "\x1b[4m\x1b[0m",
			want: "\x1b[0m",
		},
		{
			name: "dangling OSC8 open immediately reset is dropped, reset kept",
			in:   "\x1b]8;;https://x.test/\x07\x1b]8;;\x07",
			want: "\x1b]8;;\x07",
		},
		{
			name: "the reported sample5.html bug: register's own reset " +
				"collapses onto the wrap boundary as an empty italic pair",
			in:   "\x1b[3m\x1b[m\x1b[3;4mregister\x1b[m",
			want: "\x1b[m\x1b[3;4mregister\x1b[m",
		},
		{
			name: "SGR open with a visible character after it is never dropped",
			in:   "\x1b[3mx\x1b[m",
			want: "\x1b[3mx\x1b[m",
		},
		{
			name: "open A directly followed by open B (no reset) is left " +
				"alone - SGR codes are additive, not full-replace, so " +
				"dropping A could leave a bit stuck on that B never sets",
			in:   "\x1b[3;4m\x1b[3m",
			want: "\x1b[3;4m\x1b[3m",
		},
		{
			name: "reset directly followed by open is left alone - only an " +
				"open immediately before a reset is a collapse candidate",
			in:   "\x1b[m\x1b[3m",
			want: "\x1b[m\x1b[3m",
		},
		{
			name: "SGR and OSC8 tracked independently: a dangling SGR open " +
				"before a hyperlink reset is not touched",
			in:   "\x1b[3m\x1b]8;;\x07",
			want: "\x1b[3m\x1b]8;;\x07",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collapseDeadANSISpans(tc.in)
			if got != tc.want {
				t.Errorf("collapseDeadANSISpans(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRenderNoDeadSpanAroundLinkWithTrailingWhitespace is the sample5.html
// regression: an <a> wrapping an <em> and immediately followed by its own
// trailing whitespace-only text node (a real, pretty-printed-HTML pattern —
// indentation between a child element and its closing tag) sits right next
// to a differently-styled <em> sibling. coalesceTextRuns/splitANSITokens
// misattribute the escape sequences at that junction (see
// collapseDeadANSISpans's doc comment on wordWrapTokens); at some widths
// this used to surface as a dangling/empty style span, or as leaked
// underline styling on the following word. This pins the end-to-end
// rendered output, not just the cleanup pass in isolation.
func TestRenderNoDeadSpanAroundLinkWithTrailingWhitespace(t *testing.T) {
	html := `<p><em>Alternatively this line wraps right here yes indeed it does so </em><a href='https://x.test/'>
		<em>register</em>
	</a><em> to read more.</em></p>`

	for _, width := range []int{20, 24, 30, 40, 50, 60, 80, 100} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			r, err := New(Options{Width: width, Profile: colorprofile.TrueColor, NoOSC8Links: true})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := r.Render(html)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			if strings.Contains(got, "\x1b[3m\x1b[m") || strings.Contains(got, "\x1b[4m\x1b[m") {
				t.Errorf("width %d: output contains a dead empty-style span:\n%q", width, got)
			}
			if plain := stripANSI(got); strings.Contains(plain, "registerto") || strings.Contains(plain, "register  to") {
				t.Errorf("width %d: visible text malformed around the link:\n%q", width, plain)
			}
			// "to" (from the trailing <em>) must never inherit the <a>'s
			// underline — this is the actual visual bug a leaked/misplaced
			// escape sequence at the wrap boundary produces.
			if strings.Contains(got, "\x1b[4mto") {
				t.Errorf("width %d: \"to\" leaked the link's underline styling:\n%q", width, got)
			}
			// Nor should the separator space itself render as its own
			// isolated underlined span — that's <a>'s own trailing
			// whitespace-only child (the indentation before </a> in the
			// source) wrongly keeping the link's styling instead of being
			// re-styled plain by restyleTrailingWhitespaceOnlyToken.
			if strings.Contains(got, "\x1b[4m \x1b[m") {
				t.Errorf("width %d: the register/to separator space is its own underlined span:\n%q", width, got)
			}
		})
	}
}

// TestRestyleTrailingWhitespaceOnlyTokenPreservesTheSpace guards the fix
// above against the naive alternative of trimming the trailing whitespace-
// only child instead of re-styling it: trimming loses the character
// entirely whenever the parent's own stream has no other whitespace to
// supply as a replacement (e.g. these two tags glued together with no
// whitespace between them in the source), merging two words together.
func TestRestyleTrailingWhitespaceOnlyTokenPreservesTheSpace(t *testing.T) {
	html := `<p><em>foo </em><a href='https://x.test/'>
		<em>register</em>
	</a><em> bar</em></p>`
	r, err := New(Options{Width: 80, Profile: colorprofile.TrueColor, NoOSC8Links: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := r.Render(html)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if plain := strings.TrimSpace(stripANSI(got)); plain != "foo register bar" {
		t.Fatalf("Render() = %q, want text %q", plain, "foo register bar")
	}
	if strings.Contains(got, "\x1b[4m \x1b[m") {
		t.Errorf("separator space still carries the link's own underline styling:\n%q", got)
	}
}

// TestRestyleTrailingWhitespaceOnlyTokenPlainInlineBranch is the same
// pretty-printed-HTML trailing-whitespace pattern as the two tests above,
// but wrapped in a plain inline element (<b>) instead of <a> — exercising
// the splice branch of renderInlineAccTokens (childTokens), not the <a>/
// inline-block string branch (childToks). <b>'s real content is itself
// wrapped in a nested <span> so the trailing whitespace before </b> is a
// distinct whitespace-only text-node token, not fused into "bold"'s own
// token (a bare "<b>\n\tbold\n</b>" wouldn't exercise this: with no nested
// element, "bold" and its trailing whitespace collapse into a single text
// node/token together, which is a different, legitimate case — the same as
// "Alternatively, "'s own trailing space in the other tests, which must
// stay styled since it's real prose content, not pure structural
// whitespace). The two branches call restyleTrailingWhitespaceOnlyToken
// independently; this pins the splice branch's own call site.
func TestRestyleTrailingWhitespaceOnlyTokenPlainInlineBranch(t *testing.T) {
	html := `<p><em>foo </em><b>
		<span>bold</span>
	</b><em> bar</em></p>`
	r, err := New(Options{Width: 80, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := r.Render(html)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if plain := strings.TrimSpace(stripANSI(got)); plain != "foo bold bar" {
		t.Fatalf("Render() = %q, want text %q", plain, "foo bold bar")
	}
	if strings.Contains(got, "\x1b[1m \x1b[m") {
		t.Errorf("separator space still carries <b>'s own bold styling:\n%q", got)
	}
}
