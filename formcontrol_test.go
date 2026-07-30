package htmlterm_test

import "testing"

func TestFormControls(t *testing.T) {
	runCases(t, []renderCase{
		{name: "text input shows its value, padded to its default size", html: `<input type="text" value="hello">`, want: "hello               "},
		{name: "text input falls back to placeholder", html: `<input type="text" placeholder="Name">`, want: "Name                "},
		{name: "text input with no value or placeholder is a blank field", html: `<input type="text">`, want: "                    "},
		{name: "untyped input defaults to text-like", html: `<input value="hi">`, want: "hi                  "},
		{name: "size attribute overrides the default 20-column width", html: `<input value="hi" size="5">`, want: "hi   "},
		{name: "an explicit CSS width overrides the size attribute", html: `<input value="hi" size="5" style="width: 8">`, want: "hi      "},
		// Padding is measured in terminal columns, not runes: "日本語" is
		// three runes but six cells, so a size=10 field has four spaces left.
		{name: "wide runes pad by display width, not rune count", html: `<input value="日本語" size="10">`, want: "日本語    "},
		{name: "unchecked checkbox", html: `<input type="checkbox">`, want: "☐"},
		{name: "checked checkbox", html: `<input type="checkbox" checked>`, want: "☑"},
		{name: "checked=checked also counts as checked", html: `<input type="checkbox" checked="checked">`, want: "☑"},
		{name: "unchecked radio", html: `<input type="radio">`, want: "○"},
		{name: "checked radio", html: `<input type="radio" checked>`, want: "●"},
		{name: "submit input with default label", html: `<input type="submit">`, want: "[ Submit ]"},
		{name: "submit input with custom label", html: `<input type="submit" value="Go">`, want: "[ Go ]"},
		{name: "reset input with default label", html: `<input type="reset">`, want: "[ Reset ]"},
		{name: "button-type input with default label", html: `<input type="button">`, want: "[ Button ]"},
		{name: "hidden input renders nothing", html: `<input type="hidden" value="secret">end`, want: "end"},
		{name: "button wraps its rendered children in brackets", html: `<button>Click me</button>`, want: "[ Click me ]"},
		{name: "button with styled child content", html: `<button><b>OK</b></button>`, want: "[ OK ]"},
		{name: "label flows inline with its input", html: `<label>Name: <input type="text" value="Bob"></label>`, want: "Name: Bob                 "},
		{name: "textarea uses child text as its default value", html: "<textarea>line one\nline two</textarea>", want: "┌──────────────────────────────────────┐\n│ line one                             │\n│ line two                             │\n└──────────────────────────────────────┘\n"},
		{name: "textarea strips one leading newline per HTML spec", html: "<textarea>\nhi</textarea>", want: "┌──────────────────────────────────────┐\n│ hi                                   │\n└──────────────────────────────────────┘\n"},
		{name: "textarea value attribute overrides child text", html: `<textarea value="from attr">ignored</textarea>`, want: "┌──────────────────────────────────────┐\n│ from attr                            │\n└──────────────────────────────────────┘\n"},
		{name: "fieldset draws a border with legend and content", html: `<form><fieldset><legend>Info</legend><label>Name: <input type="text"></label></fieldset></form>`, width: 30, want: "┌────────────────────────────┐\n│                            │\n│ Info                       │\n│ Name:                      │\n│                            │\n└────────────────────────────┘\n\n"},
		{name: "select shows its explicitly selected option", html: `<select><option value="a">Apple</option><option value="b" selected>Banana</option></select>`, want: "[ Banana ▾]"},
		{name: "select with no selected attribute falls back to the first option", html: `<select><option value="a">Apple</option><option value="b">Banana</option></select>`, want: "[ Apple ▾]"},
		{name: "select with no options renders empty brackets", html: `<select></select>`, want: "[  ▾]"},
		{name: "select in a label flows inline", html: `<label>Fruit: <select><option>Apple</option></select></label>`, want: "Fruit: [ Apple ▾]"},
		{name: "select's closed-state display picks up an option nested inside an optgroup", html: `<select><optgroup label="Fruit"><option value="a">Apple</option><option value="b" selected>Banana</option></optgroup></select>`, want: "[ Banana ▾]"},
		{name: "select falls back to the first option even when it's inside an optgroup", html: `<select><optgroup label="Fruit"><option value="a">Apple</option><option value="b">Banana</option></optgroup></select>`, want: "[ Apple ▾]"},
	})
}

func TestProgressMeter(t *testing.T) {
	runCases(t, []renderCase{
		// <progress> value/max resolution
		{name: "progress at 0% shows only the track glyph", html: `<progress value="0" max="1"></progress>`, want: "░░░░░░░░░░░░░░░░░░░░"},
		{name: "progress at 50% fills half the bar", html: `<progress value="0.5" max="1"></progress>`, want: "██████████░░░░░░░░░░"},
		{name: "progress at 100% fills the entire bar", html: `<progress value="1" max="1"></progress>`, want: "████████████████████"},
		{name: "progress value clamps to max when it exceeds max", html: `<progress value="5" max="2"></progress>`, want: "████████████████████"},
		{name: "progress value clamps to zero when negative", html: `<progress value="-5" max="2"></progress>`, want: "░░░░░░░░░░░░░░░░░░░░"},
		{name: "progress max defaults to 1 when omitted", html: `<progress value="0.5"></progress>`, want: "██████████░░░░░░░░░░"},
		{name: "progress non-positive max falls back to 1", html: `<progress value="0.5" max="0"></progress>`, want: "██████████░░░░░░░░░░"},

		// :indeterminate
		{name: ":indeterminate matches a value-less progress", css: `progress:indeterminate::progress-bar { content: "?"; }`, html: `<progress></progress>`, want: "????????????????????"},
		{name: ":indeterminate does not match a progress with a value", css: `progress:indeterminate::progress-bar { content: "?"; }`, html: `<progress value="0.5"></progress>`, want: "██████████░░░░░░░░░░"},
		{name: "indeterminate default rendering is the track glyph across the full width, no value glyph", html: `<progress></progress>`, want: "░░░░░░░░░░░░░░░░░░░░"},

		// progress-style presets
		{name: "progress-style ascii preset uses #/- glyphs", html: `<progress value="0.5" style="progress-style: ascii"></progress>`, want: "##########----------"},
		{name: "progress-style classic preset uses space-on-background glyphs", css: `progress { progress-style: classic; }`, html: `<progress value="0.5"></progress>`, want: "                    "},

		// custom width
		{name: "explicit width resizes the bar", html: `<progress value="0.5" style="width: 10"></progress>`, want: "█████░░░░░"},
		{name: "a percentage width that rounds to zero columns renders an empty bar, not the 20-column default", html: `<div style="width:10"><progress value="0.5" style="width:1%"></progress></div>`, want: "          \n"},

		// <meter> region algorithm: optimum inside [low,high] — both outer
		// ranges are suboptimum, no even-less-good region exists in this case
		{name: "meter: optimum inside [low,high], value inside is optimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.5" low="0.3" high="0.8"></meter>`, want: "OOOOOOOOOO----------"},
		{name: "meter: optimum inside [low,high], value outside is suboptimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.9" low="0.3" high="0.8"></meter>`, want: "SSSSSSSSSSSSSSSSSS--"},

		// optimum < low: lower is better — [min,low] optimum, [low,high]
		// suboptimum, (high,max] even-less-good
		{name: "meter: optimum below low, value at or below low is optimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.1" low="0.5" high="0.8" optimum="0.2"></meter>`, want: "OO------------------"},
		{name: "meter: optimum below low, value between low and high is suboptimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.6" low="0.5" high="0.8" optimum="0.2"></meter>`, want: "SSSSSSSSSSSS--------"},
		{name: "meter: optimum below low, value above high is even-less-good", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.9" low="0.5" high="0.8" optimum="0.2"></meter>`, want: "EEEEEEEEEEEEEEEEEE--"},

		// optimum > high: higher is better — [high,max] optimum, [low,high]
		// suboptimum, [min,low) even-less-good
		{name: "meter: optimum above high, value at or above high is optimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.9" low="0.2" high="0.5" optimum="0.8"></meter>`, want: "OOOOOOOOOOOOOOOOOO--"},
		{name: "meter: optimum above high, value between low and high is suboptimum", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.3" low="0.2" high="0.5" optimum="0.8"></meter>`, want: "SSSSSS--------------"},
		{name: "meter: optimum above high, value below low is even-less-good", css: `meter::meter-optimum-value{content:"O"} meter::meter-suboptimum-value{content:"S"} meter::meter-even-less-good-value{content:"E"} meter::meter-bar{content:"-"}`, html: `<meter value="0.1" low="0.2" high="0.5" optimum="0.8"></meter>`, want: "EE------------------"},

		// defaults and degenerate ranges
		{name: "meter default optimum is the midpoint, and matches when value is also the midpoint", css: `meter::meter-optimum-value{content:"O"} meter::meter-bar{content:"-"}`, html: `<meter value="5" min="0" max="10"></meter>`, want: "OOOOOOOOOO----------"},
		{name: "meter forces max up to min when max is given less than min, then fully fills for value>=min", css: `meter::meter-optimum-value{content:"O"}`, html: `<meter value="5" min="10" max="0"></meter>`, want: "OOOOOOOOOOOOOOOOOOOO"},

		// meter-style presets
		{name: "meter-style ascii preset uses #/- glyphs", html: `<meter value="0.5"></meter>`, css: `meter { meter-style: ascii; }`, want: "##########----------"},
	})
}
