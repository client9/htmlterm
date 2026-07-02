// htmlterm renders HTML to styled terminal output.
//
// Usage:
//
//	termrender [flags] [file]
//
// If no file is given, HTML is read from stdin.
//
// Flags:
//
//	-css <file>          load a CSS stylesheet before rendering
//	-width <n>           override terminal width (default: auto-detect, fallback 80)
//	-ignore-document-css     ignore <style> elements and style= attributes in the HTML
//	-no-osc8-links       disable OSC 8 hyperlink sequences for <a> elements
//	-max-blank-lines <n> collapse runs of blank lines to at most n (0 = disabled)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/client9/htmlterm"
)

func main() {
	cssPath := flag.String("css", "", "path to CSS file")
	width := flag.Int("width", 0, "terminal width (0 = auto-detect)")
	noDocCSS := flag.Bool("ignore-document-css", false, "ignore <style> elements and style= attributes in HTML")
	noOSC8 := flag.Bool("no-osc8-links", false, "disable OSC 8 hyperlink sequences for <a> elements")
	maxBlankLines := flag.Int("max-blank-lines", 0, "collapse runs of blank lines to at most this many (0 = disabled)")
	flag.Parse()

	css := ""
	if *cssPath != "" {
		data, err := os.ReadFile(*cssPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "htmlterm: %v\n", err)
			os.Exit(1)
		}
		css = string(data)
	}

	if *width <= 0 {
		w, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || w <= 0 {
			w = 80
		}
		*width = w
	}

	r, err := htmlterm.New(htmlterm.Options{
		CSS:               css,
		Width:             *width,
		IgnoreDocumentCSS: *noDocCSS,
		NoOSC8Links:       *noOSC8,
		MaxBlankLines:     *maxBlankLines,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm: %v\n", err)
		os.Exit(1)
	}

	var src io.Reader = os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "htmlterm: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		src = f
	}

	data, err := io.ReadAll(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm: %v\n", err)
		os.Exit(1)
	}

	out, err := r.Render(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "htmlterm: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(out)
}
