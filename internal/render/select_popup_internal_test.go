package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

var popupAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripPopupANSI(s string) string { return popupAnsiRe.ReplaceAllString(s, "") }

func TestSelectPopupComposition(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option value="a">Apple</option><option value="b" selected>Banana</option><option value="c">Cherry</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	want := "[ Banana ▾]\n  Apple             \n▸ Banana            \n  Cherry            "
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}

	// Every rendered line must carry the reverse-video SGR wrapper for the
	// popup rows (not the closed control's own first line) — this is the
	// only per-cell visual distinction the popup gets, per docs/RENDERING.md.
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4:\n%q", len(lines), result.Output)
	}
	for i, line := range lines[1:] {
		if !strings.Contains(line, "\x1b[7m") || !strings.Contains(line, "\x1b[27m") {
			t.Errorf("popup line %d missing reverse-video wrapper: %q", i+1, line)
		}
	}
}

func TestSelectPopupMarkerFollowsHighlightNotSelected(t *testing.T) {
	// "selected" stays on Banana (the committed value); the highlight attr
	// (what document's moveSelectHighlight sets while arrow-browsing) is on
	// Cherry — the marker must follow the highlight, not the still-committed
	// selected option, matching the browse-then-confirm model.
	src := `<select ` + defaultSelectOpenAttr + `>` +
		`<option value="a">Apple</option>` +
		`<option value="b" selected>Banana</option>` +
		`<option value="c" ` + defaultSelectHighlightAttr + `>Cherry</option>` +
		`</select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	want := "[ Banana ▾]\n  Apple             \n  Banana            \n▸ Cherry            "
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSelectPopupOptionsGetSyntheticPositions(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option>Apple</option><option>Banana</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var selectNode, appleNode *html.Node
	for n := range result.Positions {
		switch n.Data {
		case "select":
			selectNode = n
		case "option":
			if selectOptionLabel(n) == "Apple" {
				appleNode = n
			}
		}
	}
	if selectNode == nil {
		t.Fatalf("no Rect recorded for <select>")
	}
	selRect := result.Positions[selectNode]
	appleRect, ok := result.Positions[appleNode]
	if !ok {
		t.Fatalf("no Rect recorded for the first <option>")
	}
	if appleRect.Row != selRect.Row+selRect.Height {
		t.Errorf("first option Row = %d, want %d (directly below the select)", appleRect.Row, selRect.Row+selRect.Height)
	}
	if appleRect.Col != selRect.Col {
		t.Errorf("first option Col = %d, want %d (aligned with the select)", appleRect.Col, selRect.Col)
	}
}

func TestSelectPopupClosedRendersNoExtraLines(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(`<select><option>Apple</option><option selected>Banana</option></select>`)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if got != "[ Banana ▾]" {
		t.Errorf("got %q, want %q", got, "[ Banana ▾]")
	}
}

func TestSelectPopupBackgroundColorOverridesReverseVideo(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + ` style="background-color: blue; color: white">` +
		`<option>Apple</option><option selected>Banana</option></select>`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%q", len(lines), result.Output)
	}
	for i, line := range lines[1:] {
		if strings.Contains(line, "\x1b[7m") {
			t.Errorf("popup line %d still reverse-video wrapped, want CSS background instead: %q", i+1, line)
		}
		if !strings.Contains(line, "\x1b[") {
			t.Errorf("popup line %d has no SGR styling at all: %q", i+1, line)
		}
	}
}

func TestSelectPopupOptionColorOverride(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `>` +
		`<option>Apple</option>` +
		`<option selected style="color: red">Banana</option>` +
		`</select>`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%q", len(lines), result.Output)
	}
	// Apple has no override anywhere in the chain, so it keeps the
	// historical reverse-video fallback; Banana's own color: red wins
	// instead of that fallback.
	if !strings.Contains(lines[1], "\x1b[7m") {
		t.Errorf("Apple row should keep the reverse-video fallback: %q", lines[1])
	}
	if strings.Contains(lines[2], "\x1b[7m") {
		t.Errorf("Banana row should carry its own color override, not reverse-video: %q", lines[2])
	}
	if !strings.Contains(lines[2], "\x1b[") {
		t.Errorf("Banana row should carry SGR styling for its color override: %q", lines[2])
	}
}

func TestSelectPopupHoverPseudoTracksHighlightAttr(t *testing.T) {
	src := `<style>option:hover { background-color: green; }</style>` +
		`<select ` + defaultSelectOpenAttr + `>` +
		`<option>Apple</option>` +
		`<option ` + defaultSelectHighlightAttr + `>Banana</option>` +
		`</select>`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%q", len(lines), result.Output)
	}
	// Apple doesn't match option:hover, so it keeps the reverse-video
	// fallback; Banana carries the highlight attr, matches option:hover, and
	// gets the green background instead.
	if !strings.Contains(lines[1], "\x1b[7m") {
		t.Errorf("Apple row (not highlighted) should keep the reverse-video fallback: %q", lines[1])
	}
	if strings.Contains(lines[2], "\x1b[7m") {
		t.Errorf("Banana row (highlighted, matches option:hover) should not be reverse-video: %q", lines[2])
	}
	if !strings.Contains(lines[2], "\x1b[") {
		t.Errorf("Banana row (highlighted, matches option:hover) should carry the green background: %q", lines[2])
	}
}

func TestSelectPopupBorderAndPadding(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + ` style="border: solid; padding: 1">` +
		`<option>Apple</option><option selected>Banana</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	// 1 closed control + top border + top padding + 2 options + bottom padding + bottom border.
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 7 {
		t.Fatalf("got %d lines, want 7:\n%q", len(lines), result.Output)
	}
	var selectNode, appleNode *html.Node
	for n := range result.Positions {
		switch n.Data {
		case "select":
			selectNode = n
		case "option":
			if selectOptionLabel(n) == "Apple" {
				appleNode = n
			}
		}
	}
	selRect := result.Positions[selectNode]
	appleRect, ok := result.Positions[appleNode]
	if !ok {
		t.Fatalf("no Rect recorded for the first <option>")
	}
	// Content row sits below the select's own row, plus the border row, plus
	// the one padding-top row.
	wantRow := selRect.Row + selRect.Height + 2
	if appleRect.Row != wantRow {
		t.Errorf("first option Row = %d, want %d (border+padding pushes it down)", appleRect.Row, wantRow)
	}
	// Content column sits to the right of the border char plus the one
	// padding-left column.
	wantCol := selRect.Col + 2
	if appleRect.Col != wantCol {
		t.Errorf("first option Col = %d, want %d (border+padding offset)", appleRect.Col, wantCol)
	}
}

func TestSelectPopupWidthOverride(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + ` style="width: 12">` +
		`<option>A</option><option selected>B</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%q", len(lines), result.Output)
	}
	var optNode *html.Node
	for n := range result.Positions {
		if n.Data == "option" && selectOptionLabel(n) == "A" {
			optNode = n
		}
	}
	rect, ok := result.Positions[optNode]
	if !ok {
		t.Fatalf("no Rect recorded for the first <option>")
	}
	// The line itself stays the full terminal width (spliceColumns only
	// overwrites its own column range, leaving the rest of the pre-allocated
	// blank row untouched) — the CSS width override shows up in the row's
	// own Rect.Width instead.
	if rect.Width != 12 {
		t.Errorf("option row Width = %d, want 12 (CSS width override)", rect.Width)
	}
}

func TestSelectPopupClipsRowsBeforeTopBorderWhenNoRoomBelow(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + ` style="border: solid">` +
		`<option>Apple</option><option>Banana</option><option selected>Cherry</option></select>`
	e, err := New(Options{Width: 20, Height: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (fixed height, no growth):\n%q", len(lines), result.Output)
	}
	// The top border row must still be drawn even though there's only room
	// for 2 of the 3 options below the closed control.
	if !strings.Contains(stripPopupANSI(lines[1]), "─") {
		t.Errorf("top border row missing when clipped, got %q", lines[1])
	}
}

func TestSelectPopupGrowsDocumentWhenNoRoomBelow(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option>Apple</option><option>Banana</option><option>Cherry</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (1 closed control + 3 options):\n%q", len(lines), result.Output)
	}
}
