package htmlterm_test

import "testing"

func TestFormControls(t *testing.T) {
	runCases(t, []renderCase{
		{name: "text input shows its value", html: `<input type="text" value="hello">`, want: "[hello]"},
		{name: "text input falls back to placeholder", html: `<input type="text" placeholder="Name">`, want: "[Name]"},
		{name: "text input with no value or placeholder is empty brackets", html: `<input type="text">`, want: "[]"},
		{name: "untyped input defaults to text-like", html: `<input value="hi">`, want: "[hi]"},
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
		{name: "label flows inline with its input", html: `<label>Name: <input type="text" value="Bob"></label>`, want: "Name: [Bob]"},
		{name: "textarea uses child text as its default value", html: "<textarea>line one\nline two</textarea>", want: "┌──────────────────────────────────────┐\n│ line one                             │\n│ line two                             │\n└──────────────────────────────────────┘\n"},
		{name: "textarea strips one leading newline per HTML spec", html: "<textarea>\nhi</textarea>", want: "┌──────────────────────────────────────┐\n│ hi                                   │\n└──────────────────────────────────────┘\n"},
		{name: "textarea value attribute overrides child text", html: `<textarea value="from attr">ignored</textarea>`, want: "┌──────────────────────────────────────┐\n│ from attr                            │\n└──────────────────────────────────────┘\n"},
		{name: "fieldset draws a border with legend and content", html: `<form><fieldset><legend>Info</legend><label>Name: <input type="text"></label></fieldset></form>`, width: 30, want: "┌────────────────────────────┐\n│                            │\n│ Info                       │\n│ Name: []                   │\n│                            │\n└────────────────────────────┘\n\n"},
		{name: "select shows its explicitly selected option", html: `<select><option value="a">Apple</option><option value="b" selected>Banana</option></select>`, want: "[ Banana ▾]"},
		{name: "select with no selected attribute falls back to the first option", html: `<select><option value="a">Apple</option><option value="b">Banana</option></select>`, want: "[ Apple ▾]"},
		{name: "select with no options renders empty brackets", html: `<select></select>`, want: "[  ▾]"},
		{name: "select in a label flows inline", html: `<label>Fruit: <select><option>Apple</option></select></label>`, want: "Fruit: [ Apple ▾]"},
		{name: "select's closed-state display picks up an option nested inside an optgroup", html: `<select><optgroup label="Fruit"><option value="a">Apple</option><option value="b" selected>Banana</option></optgroup></select>`, want: "[ Banana ▾]"},
		{name: "select falls back to the first option even when it's inside an optgroup", html: `<select><optgroup label="Fruit"><option value="a">Apple</option><option value="b">Banana</option></optgroup></select>`, want: "[ Apple ▾]"},
	})
}
