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
//	-strip-hidden-inline collapse elements hidden via their own inline style= (display:none, visibility:hidden, opacity:0, zero height/max-height with overflow:hidden)
//	-no-osc8-links       disable OSC 8 hyperlink sequences for <a> elements
//	-max-blank-lines <n> collapse runs of blank lines to at most n (0 = disabled)
//	-dump-html           parse input HTML and dump the normalized tree
//	-dump-html-tags      parse input HTML and dump the normalized tree with all attributes stripped
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/net/html"
	"golang.org/x/term"

	"github.com/client9/htmlterm"
)

// stripAttrs removes all attributes from every element node in the tree,
// in place, so html.Render dumps only the tag structure.
func stripAttrs(n *html.Node) {
	if n.Type == html.ElementNode {
		n.Attr = nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		stripAttrs(c)
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("htmlterm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cssPath := fs.String("css", "", "path to CSS file")
	width := fs.Int("width", 0, "terminal width (0 = auto-detect)")
	noDocCSS := fs.Bool("ignore-document-css", false, "ignore <style> elements and style= attributes in HTML")
	stripHiddenInline := fs.Bool("strip-hidden-inline", false, "remove elements hidden via their own inline style= attribute")
	noOSC8 := fs.Bool("no-osc8-links", false, "disable OSC 8 hyperlink sequences for <a> elements")
	maxBlankLines := fs.Int("max-blank-lines", 0, "collapse runs of blank lines to at most this many (0 = disabled)")
	dumpHTML := fs.Bool("dump-html", false, "parse input HTML and dump the normalized tree")
	dumpHTMLTags := fs.Bool("dump-html-tags", false, "parse input HTML and dump the normalized tree with all attributes stripped")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var src io.Reader = stdin
	if fs.NArg() > 0 {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "htmlterm: %v\n", err)
			return 1
		}
		defer f.Close()
		src = f
	}

	data, err := io.ReadAll(src)
	if err != nil {
		fmt.Fprintf(stderr, "htmlterm: %v\n", err)
		return 1
	}

	if *dumpHTML || *dumpHTMLTags {
		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			fmt.Fprintf(stderr, "htmlterm: %v\n", err)
			return 1
		}
		if *dumpHTMLTags {
			stripAttrs(doc)
		}
		if err := html.Render(stdout, doc); err != nil {
			fmt.Fprintf(stderr, "htmlterm: %v\n", err)
			return 1
		}
		return 0
	}

	css := ""
	if *cssPath != "" {
		data, err := os.ReadFile(*cssPath)
		if err != nil {
			fmt.Fprintf(stderr, "htmlterm: %v\n", err)
			return 1
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
		StripHiddenInline: *stripHiddenInline,
		NoOSC8Links:       *noOSC8,
		MaxBlankLines:     *maxBlankLines,
	})
	if err != nil {
		fmt.Fprintf(stderr, "htmlterm: %v\n", err)
		return 1
	}

	out, err := r.Render(string(data))
	if err != nil {
		fmt.Fprintf(stderr, "htmlterm: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, out)
	return 0
}
