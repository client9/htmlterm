// htmlterm-tui is a small interactive demo of htmlterm's Loop: it renders a
// form to the real terminal and lets you drive it with the keyboard and
// mouse (Tab to move forward, type into the focused field, click a
// checkbox, Enter or click Submit to submit, Ctrl-C to quit). Submitting
// fires a real "submit" event (Document.AddEventListener) on the <form>,
// whose handler reveals a result line underneath echoing what was entered —
// demonstrating the event system end to end, not just individual field
// mutation.
//
// The "Favorite color" <select> demonstrates the dropdown popup: Tab to it
// and press Enter/Space (or click it) to open the option list, then click an
// option to select it and close the popup (Escape closes it without
// changing the selection). ArrowUp/ArrowDown change the selection directly
// whether the popup is open or closed, matching a real <select>. The popup
// itself is composited as a reverse-video overlay directly beneath the
// control, on top of whatever content follows it — see docs/RENDERING.md's
// "Popups / z-order" section.
//
// Width is SizeAutomatic and Height is SizeNatural: resize the terminal
// window and the long paragraph below the form reflows live at the new
// width (via Loop's SIGWINCH handling — see loop.go's applyTerminalSize),
// while the document's height is left unconstrained rather than
// clipped/padded to the terminal's row count.
//
// Below the paragraph is a scrollable log pane (overflow-y:scroll with an
// explicit height — see docs/SCROLLING.md), with a second, nested scrollable
// pane inside it (plain overflow:auto) demonstrating that nested scrollable
// regions need no special handling. Scroll either with the mouse wheel over
// a pane, or Tab to the button inside the outer one (focus auto-scrolls it
// into view — Document.Focus's scrollIntoView), then use
// PageUp/PageDown/ArrowUp/ArrowDown. The outer pane's overflow-y:scroll
// draws an always-on │/█ gutter/thumb tracking the scroll position — the
// nested pane's plain overflow:auto deliberately draws none (see
// docs/SCROLLING.md's "Scrollbar gutter and indicator" for why).
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
	"github.com/client9/htmlterm/tui"
)

const formHTML = `
<style>
  input:focus, button:focus, select:focus { background-color: #4477cc; color: #ffffff; }
  #result { display: none; }
  #result.visible { display: block; }
  #result.visible::before {
    content: "Submitted! Name: " attr(data-name) " — Subscribed: " attr(data-subscribed) " — Color: " attr(data-color);
  }
  #status { color: #888888; }
  #spinner::before { content: attr(data-frame); }
  #clock::before { content: attr(data-time); }
  #lorem { margin-top: 1; }
</style>
<form id="myform">
  <label>Name: <input type="text" id="name" placeholder="your name"></label><br>
  <label><input type="checkbox" id="subscribe"> Subscribe to updates</label><br>
  <label>Favorite color: <select id="color">
    <option value="red">Red</option>
    <option value="green" selected>Green</option>
    <option value="blue">Blue</option>
  </select></label><br>
  <button type="submit">Submit</button>
</form>
<div id="result"></div>
<div id="status"><span id="spinner" data-frame="⠋"></span> <span id="clock" data-time="00:00:00"></span></div>
<p id="lorem">Resize this terminal window to see this paragraph reflow live
at the new width. Width tracks the terminal automatically (SizeAutomatic),
kept live across every resize via Loop's SIGWINCH handling, while Height
stays SizeNatural: only the wrap width changes, and nothing above or below
gets clipped or padded vertically. Lorem ipsum dolor sit amet, consectetur
adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris
nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in
reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla
pariatur.</p>
<div id="log" style="height:6;overflow-y:scroll;border-style:solid;padding-left:1;margin-top:1">
Log line 1<br>Log line 2<br>Log line 3<br>Log line 4<br>Log line 5<br>
<div id="nested" style="height:3;overflow:auto;border-style:solid;margin-top:1;margin-bottom:1">
Nested 1<br>Nested 2<br>Nested 3<br>Nested 4<br>Nested 5<br>Nested 6
</div>
Log line 6<br>Log line 7<br>Log line 8<br>Log line 9<br>Log line 10<br>
<button id="logbtn">Jump target (Tab here)</button>
</div>
`

// spinnerFrames cycles a decorative Braille spinner, driven by
// Loop.SetInterval (timer.go) — a periodic update with nothing to do with
// keyboard/mouse input, demonstrating the timer mechanism end to end.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func main() {
	os.Exit(run())
}

func run() int {
	doc, err := document.ParseDocument(formHTML, htmlterm.Options{
		Width:  htmlterm.SizeAutomatic,
		Height: htmlterm.SizeNatural,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}

	name := doc.GetElementByID("name")
	if name != nil {
		name.Focus()
	}
	subscribe := doc.GetElementByID("subscribe")
	color := doc.GetElementByID("color")
	result := doc.GetElementByID("result")

	doc.AddEventListener(doc.GetElementByID("myform"), "submit", false, func(e *document.Event) {
		subscribed := "no"
		if subscribe.Checked() {
			subscribed = "yes"
		}
		result.SetAttribute("data-name", name.Value())
		result.SetAttribute("data-subscribed", subscribed)
		result.SetAttribute("data-color", color.Value())
		result.ClassList().Add("visible")
	})

	loop, err := tui.NewLoop(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}

	spinner := doc.GetElementByID("spinner")
	spinnerFrame := 0
	loop.SetInterval(100*time.Millisecond, func() {
		spinnerFrame = (spinnerFrame + 1) % len(spinnerFrames)
		spinner.SetAttribute("data-frame", spinnerFrames[spinnerFrame])
	})

	clock := doc.GetElementByID("clock")
	loop.SetInterval(time.Second, func() {
		clock.SetAttribute("data-time", time.Now().Format("15:04:05"))
	})

	if err := loop.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}
	return 0
}
