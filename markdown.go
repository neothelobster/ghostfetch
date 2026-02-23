package main

import (
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

// Tags to strip in reader mode.
var stripTags = map[string]bool{
	"script":   true,
	"style":    true,
	"nav":      true,
	"footer":   true,
	"header":   true,
	"aside":    true,
	"iframe":   true,
	"noscript": true,
	"svg":      true,
	"form":     true,
}

// htmlToMarkdown converts raw HTML to markdown.
// If readerMode is true, it first extracts the main content and strips boilerplate.
func htmlToMarkdown(rawHTML string, pageURL string, readerMode bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}

	if readerMode {
		stripUnwantedNodes(doc)
		if main := findMainContent(doc); main != nil {
			doc = main
		}
	}

	opts := []converter.ConvertOptionFunc{}
	if pageURL != "" {
		opts = append(opts, converter.WithDomain(pageURL))
	}

	md, err := htmltomarkdown.ConvertNode(doc, opts...)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(md))
	return result, nil
}

// stripUnwantedNodes removes script, style, nav, footer, header, aside, etc.
func stripUnwantedNodes(doc *html.Node) {
	var toRemove []*html.Node
	collectUnwanted(doc, &toRemove)
	for _, n := range toRemove {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
}

func collectUnwanted(n *html.Node, toRemove *[]*html.Node) {
	if n.Type == html.ElementNode && stripTags[n.Data] {
		*toRemove = append(*toRemove, n)
		return // don't recurse into removed nodes
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectUnwanted(c, toRemove)
	}
}

// findMainContent looks for <article> or <main> tags.
func findMainContent(doc *html.Node) *html.Node {
	return findElement(doc, "article", "main")
}

func findElement(n *html.Node, tags ...string) *html.Node {
	if n.Type == html.ElementNode {
		for _, tag := range tags {
			if n.Data == tag {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tags...); found != nil {
			return found
		}
	}
	return nil
}
