package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// pageLink represents a single link extracted from a page.
type pageLink struct {
	URL  string `json:"url"`
	Text string `json:"text"`
}

// extractLinks parses HTML and extracts all <a href="..."> links, resolving
// relative URLs against baseURL. It skips empty hrefs, fragment-only (#...),
// and javascript: links, and deduplicates by URL.
func extractLinks(body []byte, baseURL string) []pageLink {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var links []pageLink

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")

			// Skip empty hrefs, fragment-only, and javascript: links.
			if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
				// Still recurse into children — there might be nested <a> tags
				// (unusual but possible).
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				return
			}

			// Resolve relative URLs.
			parsed, err := url.Parse(href)
			if err != nil {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				return
			}
			resolved := base.ResolveReference(parsed).String()

			// Deduplicate.
			if !seen[resolved] {
				seen[resolved] = true
				text := strings.TrimSpace(textContent(n))
				links = append(links, pageLink{
					URL:  resolved,
					Text: text,
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return links
}

// formatLinks formats a slice of pageLink as a markdown list.
// Each link is rendered as "- [Text](url)". If Text is empty, the URL is used.
func formatLinks(links []pageLink) string {
	var sb strings.Builder
	for _, l := range links {
		text := l.Text
		if text == "" {
			text = l.URL
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", text, l.URL))
	}
	return sb.String()
}

// runLinks fetches a URL, extracts links, optionally filters them, and outputs
// the result as markdown text or JSON.
func runLinks(rawURL string, filterPattern string) error {
	result, err := fetchOne(fetchOptions{
		url:            rawURL,
		browser:        flagBrowser,
		timeout:        flagTimeout,
		noCookies:      flagNoCookies,
		verbose:        flagVerbose,
		captchaService: flagCaptchaService,
		captchaKey:     flagCaptchaKey,
	})
	if err != nil {
		return err
	}

	links := extractLinks(result.Body, result.URL)

	// Filter links if pattern is provided.
	if filterPattern != "" {
		re, err := regexp.Compile(filterPattern)
		if err != nil {
			return fmt.Errorf("invalid filter pattern: %w", err)
		}
		var filtered []pageLink
		for _, l := range links {
			if re.MatchString(l.URL) || re.MatchString(l.Text) {
				filtered = append(filtered, l)
			}
		}
		links = filtered
	}

	if flagJSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(links)
	}

	fmt.Fprint(os.Stdout, formatLinks(links))
	return nil
}
