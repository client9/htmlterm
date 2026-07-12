package cssengine

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// buildDeepDocument returns a document consisting of a single chain of
// `depth` nested <div class="lN box"> elements, each wrapping the next, so a
// Cascade.Resolve call on the innermost node walks `depth` ancestors.
func buildDeepDocument(depth int) *html.Node {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&sb, `<div class="l%d box">`, i)
	}
	sb.WriteString("leaf")
	for i := 0; i < depth; i++ {
		sb.WriteString("</div>")
	}
	doc, err := html.Parse(strings.NewReader(sb.String()))
	if err != nil {
		panic(err)
	}
	return doc
}

// buildManyRules returns n rules each matching one of buildDeepDocument's
// per-level classes, plus a shared ".box" rule declaring an inheritable
// property so every ancestor in the chain contributes to Resolve's
// inheritance walk.
func buildManyRules(n int) []Rule {
	var rules []Rule
	for i := 0; i < n; i++ {
		sel := fmt.Sprintf(".l%d", i)
		rules = append(rules, Rule{selector: sel, decls: map[string]declValue{"background-color": {value: "red"}}, parts: parseSelector(sel)})
	}
	rules = append(rules, Rule{selector: ".box", decls: map[string]declValue{"color": {value: "blue"}}, parts: parseSelector(".box")})
	return rules
}

func elementNodes(doc *html.Node) []*html.Node {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return nodes
}

// BenchmarkCascadeResolveDeepDocumentNoCache simulates one render pass with
// no Cascade.Cache set (a zero-value Cascade{Rules: ...}, as every
// cssengine_internal_test.go caller and any pre-Cache-field caller
// constructs it): resolving every element node in a deeply nested document
// once, against a ruleset sized to match. Cascade.Resolve's ancestor walk
// re-runs Direct(anc) — a full rule-match pass — from scratch for every
// descendant of anc, so this benchmark's cost scales with nodes * depth *
// rules.
func BenchmarkCascadeResolveDeepDocumentNoCache(b *testing.B) {
	const depth = 200
	doc := buildDeepDocument(depth)
	cascade := Cascade{Rules: buildManyRules(depth)}
	nodes := elementNodes(doc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, n := range nodes {
			cascade.Resolve(n)
		}
	}
}

// BenchmarkCascadeResolveDeepDocumentCached is
// BenchmarkCascadeResolveDeepDocumentNoCache with Cache set, the way
// internal/render's Engine wires it up: one fresh map per render pass (reset
// every b.N iteration, matching Engine.RenderNode making a fresh map per
// RenderNode call), reused across every node resolved within that pass so
// Direct(anc) is computed once per ancestor and reused by all of its
// descendants instead of being recomputed from scratch by each one.
func BenchmarkCascadeResolveDeepDocumentCached(b *testing.B) {
	const depth = 200
	doc := buildDeepDocument(depth)
	rules := buildManyRules(depth)
	nodes := elementNodes(doc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cascade := Cascade{Rules: rules, Cache: make(map[*html.Node]map[string]string)}
		for _, n := range nodes {
			cascade.Resolve(n)
		}
	}
}
