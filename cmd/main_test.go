package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReadsFromStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-width", "40"}, strings.NewReader("<p>hello</p>"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	if got, want := stdout.String(), "hello\n\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunReadsFromFileAndCSS(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "input.html")
	cssPath := filepath.Join(dir, "style.css")
	if err := os.WriteFile(htmlPath, []byte(`<p class="x">hello</p>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssPath, []byte(`.x { text-transform: uppercase; }`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-width", "40", "-css", cssPath, htmlPath}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	if got, want := stdout.String(), "HELLO\n\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunIgnoreDocumentCSS(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"-width", "40", "-ignore-document-css"},
		strings.NewReader(`<style>p { display: none; }</style><p>visible</p>`),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	if got, want := stdout.String(), "visible\n\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunNoOSC8Links(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-width", "40", "-no-osc8-links"}, strings.NewReader(`<a href="https://example.com">link</a>`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b]8;;") {
		t.Fatalf("stdout contains OSC 8 hyperlink: %q", stdout.String())
	}
}

func TestRunDumpHTMLFromStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dump-html"}, strings.NewReader(`<p>hello<b>world`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	want := `<html><head></head><body><p>hello<b>world</b></p></body></html>`
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunDumpHTMLFromFile(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "input.html")
	if err := os.WriteFile(htmlPath, []byte(`<table><tr><td>x`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dump-html", htmlPath}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, stderr.String())
	}
	want := `<html><head></head><body><table><tbody><tr><td>x</td></tr></tbody></table></body></html>`
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunMissingCSSFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-css", filepath.Join(t.TempDir(), "missing.css")}, strings.NewReader("<p>x</p>"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "htmlterm:") {
		t.Fatalf("stderr = %q, want htmlterm error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
