package htmlterm_test

import (
	"fmt"
	"log"

	"github.com/client9/htmlterm"
)

func ExampleRenderer_Render() {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, NoOSC8Links: true})
	if err != nil {
		log.Fatal(err)
	}
	out, err := r.Render(`<p>Hello terminal.</p>`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(out)
	// Output:
	// Hello terminal.
	//
}

func ExampleNew_customCSS() {
	css := `.note { border-left: "│"; padding-left: 1; }`
	r, err := htmlterm.New(htmlterm.Options{CSS: css, Width: 24, NoOSC8Links: true})
	if err != nil {
		log.Fatal(err)
	}
	out, err := r.Render(`<p class="note">custom CSS</p>`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(out)
	// Output:
	// │ custom CSS
	//
}

func ExampleNew_untrustedHTML() {
	r, err := htmlterm.New(htmlterm.Options{
		Width:             40,
		IgnoreDocumentCSS: true,
		NoOSC8Links:       true,
	})
	if err != nil {
		log.Fatal(err)
	}
	out, err := r.Render("<p>\x1b[31mnot red\x1b[0m</p>")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(out)
	// Output:
	// not red
	//
}
