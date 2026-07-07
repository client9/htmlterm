// htmlterm-tui is a small interactive demo of htmlterm's Loop: it renders a
// form to the real terminal and lets you drive it with the keyboard and
// mouse (Tab to move forward, type into the focused field, click a
// checkbox, Enter or click Submit to submit, Ctrl-C to quit). Submitting
// fires a real "submit" event (Document.AddEventListener) on the <form>,
// whose handler reveals a result line underneath echoing what was entered —
// demonstrating the event system end to end, not just individual field
// mutation.
//
// Width is SizeAutomatic and Height is SizeNatural: resize the terminal
// window and the long paragraph below the form reflows live at the new
// width (via Loop's SIGWINCH handling — see loop.go's applyTerminalSize),
// while the document's height is left unconstrained rather than
// clipped/padded to the terminal's row count.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/client9/htmlterm"
)

const formHTML = `
<style>
  input:focus, button:focus { background-color: #4477cc; color: #ffffff; }
  #result { display: none; }
  #result.visible { display: block; }
  #result.visible::before {
    content: "Submitted! Name: " attr(data-name) " — Subscribed: " attr(data-subscribed);
  }
  #status { color: #888888; }
  #spinner::before { content: attr(data-frame); }
  #clock::before { content: attr(data-time); }
  #lorem { margin-top: 1; }
</style>
<form id="myform">
  <label>Name: <input type="text" id="name" placeholder="your name"></label><br>
  <label><input type="checkbox" id="subscribe"> Subscribe to updates</label><br>
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
`

// spinnerFrames cycles a decorative Braille spinner, driven by
// Loop.SetInterval (timer.go) — a periodic update with nothing to do with
// keyboard/mouse input, demonstrating the timer mechanism end to end.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func main() {
	os.Exit(run())
}

func run() int {
	doc, err := htmlterm.ParseDocument(formHTML, htmlterm.Options{
		Width:  htmlterm.SizeAutomatic,
		Height: htmlterm.SizeNatural,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}

	name := doc.GetElementByID("name")
	if name != nil {
		doc.Focus(name)
	}
	subscribe := doc.GetElementByID("subscribe")
	result := doc.GetElementByID("result")

	doc.AddEventListener(doc.GetElementByID("myform"), "submit", false, func(e *htmlterm.Event) {
		subscribed := "no"
		if subscribe.Checked() {
			subscribed = "yes"
		}
		result.SetAttribute("data-name", name.Value())
		result.SetAttribute("data-subscribed", subscribed)
		result.ClassList().Add("visible")
	})

	loop := htmlterm.NewLoop(doc, os.Stdin, os.Stdout)

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
