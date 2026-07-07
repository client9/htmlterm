// htmlterm-tui is a small interactive demo of htmlterm's Loop: it renders a
// form to the real terminal and lets you drive it with the keyboard and
// mouse (Tab/Shift is not wired to a key here — use Tab to move forward,
// type into the focused field, click a checkbox, Ctrl-C to quit).
package main

import (
	"fmt"
	"os"

	"github.com/client9/htmlterm"
)

const formHTML = `
<style>
  input:focus, button:focus { background-color: #4477cc; color: #ffffff; }
</style>
<form>
  <label>Name: <input type="text" id="name" placeholder="your name"></label><br>
  <label><input type="checkbox" id="subscribe"> Subscribe to updates</label><br>
  <button type="submit">Submit</button>
</form>
`

func main() {
	os.Exit(run())
}

func run() int {
	doc, err := htmlterm.ParseDocument(formHTML, htmlterm.Options{Width: 60})
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}

	if name := doc.GetElementByID("name"); name != nil {
		doc.Focus(name)
	}

	if err := htmlterm.NewLoop(doc, os.Stdin, os.Stdout).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm-tui: %v\n", err)
		return 1
	}
	return 0
}
