package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// datalistSrc builds an <input list> plus its <datalist>, with the reserved
// markers the document layer would normally set written directly into the
// markup: open on the input, match on each option named in matches, and the
// shared option-highlight marker on highlight.
//
// Options are given as value strings. Every option is declared; only the ones
// in matches carry the match marker, which is what the popup draws.
func datalistSrc(values []string, matches []string, highlight string) string {
	var b strings.Builder
	b.WriteString(`<input list="l" ` + defaultDatalistOpenAttr + `>`)
	b.WriteString(`<datalist id="l">`)
	for _, v := range values {
		b.WriteString(`<option value="` + v + `"`)
		for _, m := range matches {
			if m == v {
				b.WriteString(` ` + defaultDatalistMatchAttr)
				break
			}
		}
		if v == highlight {
			b.WriteString(` ` + defaultSelectHighlightAttr)
		}
		b.WriteString(`>`)
	}
	b.WriteString(`</datalist>`)
	return b.String()
}

func optionRects(t *testing.T, positions map[*html.Node]Rect) map[string]Rect {
	t.Helper()
	out := map[string]Rect{}
	for n, r := range positions {
		if n.Type == html.ElementNode && n.Data == "option" {
			out[nodeAttr(n, "value")] = r
		}
	}
	return out
}

func TestDatalistPopupDrawsOnlyMatchingOptions(t *testing.T) {
	// "banana" is declared but unmarked, so the renderer must not draw it:
	// the document layer alone decides what matches.
	src := datalistSrc([]string{"apple", "apricot", "banana"}, []string{"apple", "apricot"}, "apple")
	e, err := New(Options{Width: 30})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	// The field's own line is just the 20-column input; only the popup rows
	// below it are spliced onto full-width blank lines.
	want := "                    \n" +
		"▸ apple                       \n" +
		"  apricot                     "
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if strings.Contains(got, "banana") {
		t.Error("an unmarked option was drawn in the popup")
	}
}

func TestDatalistPopupLabelFallsBackToValue(t *testing.T) {
	// The ordinary datalist option form, <option value="x"> with no text and
	// no label attribute, is exactly the case selectOptionLabel returns "" for.
	src := datalistSrc([]string{"apple"}, []string{"apple"}, "")
	e, _ := New(Options{Width: 20})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if !strings.Contains(stripPopupANSI(result.Output), "apple") {
		t.Errorf("value-only option rendered blank:\n%q", stripPopupANSI(result.Output))
	}
}

func TestDatalistPopupLabelPrefersLabelThenTextThenValue(t *testing.T) {
	src := `<input list="l" ` + defaultDatalistOpenAttr + `><datalist id="l">` +
		`<option value="v1" label="L1" ` + defaultDatalistMatchAttr + `>text1</option>` +
		`<option value="v2" ` + defaultDatalistMatchAttr + `>text2</option>` +
		`<option value="v3" ` + defaultDatalistMatchAttr + `>` +
		`</datalist>`
	e, _ := New(Options{Width: 20})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	for _, want := range []string{"L1", "text2", "v3"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%q", want, got)
		}
	}
	if strings.Contains(got, "v1") || strings.Contains(got, "text1") {
		t.Errorf("label attribute did not win over value and text:\n%q", got)
	}
}

func TestDatalistPopupAnchorsUnderTheField(t *testing.T) {
	// A text input's Rect is its whole padded field, 20 columns by default,
	// so the popup aligns to that rather than to the typed text.
	src := `<span>Name: </span>` + datalistSrc([]string{"apple"}, []string{"apple"}, "")
	e, _ := New(Options{Width: 40})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var inputRect Rect
	for n, r := range result.Positions {
		if n.Type == html.ElementNode && n.Data == "input" {
			inputRect = r
		}
	}
	rect, ok := optionRects(t, result.Positions)["apple"]
	if !ok {
		t.Fatal("no Rect recorded for the suggestion row")
	}
	if rect.Col != inputRect.Col {
		t.Errorf("row Col = %d, want %d (the field's own left edge)", rect.Col, inputRect.Col)
	}
	if rect.Row != inputRect.Row+inputRect.Height {
		t.Errorf("row Row = %d, want %d (directly beneath the field)", rect.Row, inputRect.Row+inputRect.Height)
	}
	if rect.Width != inputRect.Width {
		t.Errorf("row Width = %d, want %d (at least the field's own width)", rect.Width, inputRect.Width)
	}
}

func TestDatalistPopupRectsExcludeBorderAndPadding(t *testing.T) {
	src := datalistSrc([]string{"apple"}, []string{"apple"}, "")
	e, _ := New(Options{Width: 30, CSS: `datalist { border-style: solid; padding-left: 2; }`})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	rect := optionRects(t, result.Positions)["apple"]
	// Border column plus two padding columns, so a click on the popup's own
	// chrome isn't misattributed to the row.
	if rect.Col != 3 {
		t.Errorf("row Col = %d, want 3 (past one border column and two of padding)", rect.Col)
	}
	if rect.Row != 2 {
		t.Errorf("row Row = %d, want 2 (past the input's row and the top border)", rect.Row)
	}
}

func TestDatalistPopupStyledByDatalistElement(t *testing.T) {
	src := datalistSrc([]string{"apple"}, []string{"apple"}, "")
	e, _ := New(Options{Width: 30, CSS: `datalist { border-style: rounded; }`})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if !strings.Contains(got, "╭") || !strings.Contains(got, "╰") {
		t.Errorf("datalist border-style did not reach the popup:\n%q", got)
	}
}

func TestDatalistPopupOptionHoverStyling(t *testing.T) {
	// The highlight marker is the same one <select>'s popup uses, so a single
	// `option:hover` rule styles both.
	src := datalistSrc([]string{"apple", "apricot"}, []string{"apple", "apricot"}, "apricot")
	e, _ := New(Options{Width: 30, CSS: `option:hover { background-color: #ff0000; }`, Profile: colorprofile.TrueColor})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	for _, line := range strings.Split(result.Output, "\n") {
		plain := stripPopupANSI(line)
		if strings.Contains(plain, "apricot") && !strings.Contains(line, "48;2;255;0;0") {
			t.Errorf("highlighted row missing option:hover background: %q", line)
		}
		if strings.Contains(plain, "apple") && strings.Contains(line, "48;2;255;0;0") {
			t.Errorf("unhighlighted row picked up option:hover background: %q", line)
		}
	}
}

func TestDatalistPopupWidthOverride(t *testing.T) {
	src := datalistSrc([]string{"apple"}, []string{"apple"}, "")
	e, _ := New(Options{Width: 30, CSS: `datalist { width: 8; }`})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if rect := optionRects(t, result.Positions)["apple"]; rect.Width != 8 {
		t.Errorf("row Width = %d, want 8 (explicit width beats the field's own)", rect.Width)
	}
}

func TestDatalistPopupClipsUnderFixedHeight(t *testing.T) {
	// With Options.Height set the document can't grow, so rows are dropped
	// from the end rather than the popup overflowing the viewport.
	src := datalistSrc([]string{"a1", "a2", "a3", "a4"}, []string{"a1", "a2", "a3", "a4"}, "")
	e, _ := New(Options{Width: 20, Height: 3})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want exactly Options.Height (3):\n%q", len(lines), result.Output)
	}
	got := stripPopupANSI(result.Output)
	if !strings.Contains(got, "a1") || !strings.Contains(got, "a2") {
		t.Errorf("rows that fit were dropped:\n%q", got)
	}
	if strings.Contains(got, "a3") || strings.Contains(got, "a4") {
		t.Errorf("rows past the viewport were drawn:\n%q", got)
	}
}

func TestDatalistPopupAbsentWithoutMarkers(t *testing.T) {
	// No open marker: nothing is composited, and the <datalist> itself stays
	// invisible rather than leaking its options' text (the UA display:none
	// rule), which is what it did before this element was supported.
	src := `<input list="l"><datalist id="l"><option value="apple">apple-text</option></datalist>after`
	e, _ := New(Options{Width: 30})
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if strings.Contains(got, "apple") {
		t.Errorf("a closed datalist leaked its options:\n%q", got)
	}
	if !strings.Contains(got, "after") {
		t.Errorf("content after the datalist went missing:\n%q", got)
	}
}

func TestDatalistPopupIgnoresUnresolvableList(t *testing.T) {
	for _, src := range []string{
		`<input ` + defaultDatalistOpenAttr + `>`,                                      // no list attribute
		`<input list="nope" ` + defaultDatalistOpenAttr + `>`,                          // unknown id
		`<input list="l" ` + defaultDatalistOpenAttr + `><div id="l">not a list</div>`, // wrong element
	} {
		e, _ := New(Options{Width: 20})
		result, err := e.RenderHTML(src)
		if err != nil {
			t.Fatalf("RenderHTML(%q): %v", src, err)
		}
		// The <div> case renders its own content, as any block would; what
		// must not happen is a popup being composited for it.
		if len(optionRects(t, result.Positions)) != 0 {
			t.Errorf("src %q recorded suggestion-row Rects", src)
		}
		if strings.Contains(result.Output, "▸") {
			t.Errorf("src %q drew a popup:\n%q", src, stripPopupANSI(result.Output))
		}
	}
}
