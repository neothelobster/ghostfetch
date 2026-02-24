package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// searchResult represents a single search result.
type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// searchEngine defines a search engine with its URL builder and parser.
type searchEngine struct {
	Name      string
	SearchURL func(query string, maxResults int) string
	Parse     func(body []byte) []searchResult
}

// engines is the registry of available search engines.
var engines = map[string]searchEngine{
	"google": {
		Name: "Google",
		SearchURL: func(query string, maxResults int) string {
			return fmt.Sprintf("https://www.google.com/search?q=%s&num=%d&hl=en", url.QueryEscape(query), maxResults)
		},
		Parse: parseGoogleResults,
	},
	"bing": {
		Name: "Bing",
		SearchURL: func(query string, maxResults int) string {
			return fmt.Sprintf("https://www.bing.com/search?q=%s&count=%d", url.QueryEscape(query), maxResults)
		},
		Parse: parseBingResults,
	},
	"duckduckgo": {
		Name: "DuckDuckGo",
		SearchURL: func(query string, maxResults int) string {
			return fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
		},
		Parse: parseDuckDuckGoResults,
	},
	"brave": {
		Name: "Brave",
		SearchURL: func(query string, maxResults int) string {
			return fmt.Sprintf("https://search.brave.com/search?q=%s&count=%d", url.QueryEscape(query), maxResults)
		},
		Parse: parseBraveResults,
	},
}

// parseGoogleResults parses Google search result HTML and extracts results.
func parseGoogleResults(body []byte) []searchResult {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "g") {
			if r, ok := extractGoogleResult(n); ok {
				results = append(results, r)
			}
			return // don't recurse into result blocks
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results
}

// extractGoogleResult extracts a single search result from a <div class="g"> block.
func extractGoogleResult(n *html.Node) (searchResult, bool) {
	var r searchResult

	// Find the first <a> with an href starting with "http".
	var findLink func(*html.Node) string
	findLink = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "http") {
					return attr.Val
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if link := findLink(c); link != "" {
				return link
			}
		}
		return ""
	}
	r.URL = findLink(n)

	// Find the <h3> for the title.
	var findH3 func(*html.Node) string
	findH3 = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "h3" {
			return textContent(node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if t := findH3(c); t != "" {
				return t
			}
		}
		return ""
	}
	r.Title = findH3(n)

	// Find the snippet from <div class="VwiC3b"> or <div class="IsZvec">.
	var findSnippet func(*html.Node) string
	findSnippet = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "div" {
			if hasClass(node, "VwiC3b") || hasClass(node, "IsZvec") {
				return textContent(node)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if s := findSnippet(c); s != "" {
				return s
			}
		}
		return ""
	}
	r.Snippet = findSnippet(n)

	if r.URL == "" && r.Title == "" {
		return r, false
	}
	return r, true
}

// getAttr returns the value of an attribute on an HTML node, or empty string if not found.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// parseBingResults parses Bing search result HTML and extracts results.
func parseBingResults(body []byte) []searchResult {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "b_algo") {
			if r, ok := extractBingResult(n); ok {
				results = append(results, r)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results
}

// extractBingResult extracts a single search result from a <li class="b_algo"> block.
func extractBingResult(n *html.Node) (searchResult, bool) {
	var r searchResult

	// Find the <h2> and extract the <a> inside it for URL and title.
	var findH2Link func(*html.Node)
	findH2Link = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "h2" {
			// Find <a> inside the h2.
			var findA func(*html.Node)
			findA = func(inner *html.Node) {
				if inner.Type == html.ElementNode && inner.Data == "a" {
					href := getAttr(inner, "href")
					if strings.HasPrefix(href, "http") {
						r.URL = href
						r.Title = textContent(inner)
					}
					return
				}
				for c := inner.FirstChild; c != nil; c = c.NextSibling {
					findA(c)
				}
			}
			findA(node)
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			findH2Link(c)
		}
	}
	findH2Link(n)

	// Find snippet from <div class="b_caption"> -> <p>.
	var findSnippet func(*html.Node) string
	findSnippet = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "b_caption") {
			// Look for <p> inside.
			var findP func(*html.Node) string
			findP = func(inner *html.Node) string {
				if inner.Type == html.ElementNode && inner.Data == "p" {
					return textContent(inner)
				}
				for c := inner.FirstChild; c != nil; c = c.NextSibling {
					if s := findP(c); s != "" {
						return s
					}
				}
				return ""
			}
			if s := findP(node); s != "" {
				return s
			}
			return textContent(node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if s := findSnippet(c); s != "" {
				return s
			}
		}
		return ""
	}
	r.Snippet = findSnippet(n)

	if r.URL == "" && r.Title == "" {
		return r, false
	}
	return r, true
}

// parseDuckDuckGoResults parses DuckDuckGo HTML search result page and extracts results.
func parseDuckDuckGoResults(body []byte) []searchResult {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "result") {
			if r, ok := extractDuckDuckGoResult(n); ok {
				results = append(results, r)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results
}

// extractDuckDuckGoResult extracts a single search result from a <div class="result"> block.
func extractDuckDuckGoResult(n *html.Node) (searchResult, bool) {
	var r searchResult

	// Find <a class="result__a"> for URL and title.
	var findResultA func(*html.Node)
	findResultA = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" && hasClass(node, "result__a") {
			r.URL = getAttr(node, "href")
			r.Title = textContent(node)
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			findResultA(c)
		}
	}
	findResultA(n)

	// Find snippet from element with class "result__snippet".
	var findSnippet func(*html.Node) string
	findSnippet = func(node *html.Node) string {
		if node.Type == html.ElementNode && hasClass(node, "result__snippet") {
			return textContent(node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if s := findSnippet(c); s != "" {
				return s
			}
		}
		return ""
	}
	r.Snippet = findSnippet(n)

	if r.URL == "" && r.Title == "" {
		return r, false
	}
	return r, true
}

// parseBraveResults parses Brave search result HTML and extracts results.
func parseBraveResults(body []byte) []searchResult {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "snippet") && getAttr(n, "data-type") == "web" {
			if r, ok := extractBraveResult(n); ok {
				results = append(results, r)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results
}

// extractBraveResult extracts a single search result from a <div class="snippet" data-type="web"> block.
func extractBraveResult(n *html.Node) (searchResult, bool) {
	var r searchResult

	// Find <a> inside .snippet-title for URL and title.
	var findTitle func(*html.Node)
	findTitle = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "snippet-title") {
			// Find <a> inside.
			var findA func(*html.Node)
			findA = func(inner *html.Node) {
				if inner.Type == html.ElementNode && inner.Data == "a" {
					href := getAttr(inner, "href")
					if strings.HasPrefix(href, "http") {
						r.URL = href
						r.Title = textContent(inner)
					}
					return
				}
				for c := inner.FirstChild; c != nil; c = c.NextSibling {
					findA(c)
				}
			}
			findA(node)
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}
	findTitle(n)

	// Find snippet from .snippet-description.
	var findSnippet func(*html.Node) string
	findSnippet = func(node *html.Node) string {
		if node.Type == html.ElementNode && hasClass(node, "snippet-description") {
			return textContent(node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if s := findSnippet(c); s != "" {
				return s
			}
		}
		return ""
	}
	r.Snippet = findSnippet(n)

	if r.URL == "" && r.Title == "" {
		return r, false
	}
	return r, true
}

// hasClass checks if an HTML node has a specific class in its class attribute.
func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

// textContent returns the concatenated text content of a node and all its children.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// formatSearchResults formats search results as a numbered markdown list.
func formatSearchResults(query string, results []searchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search: %q\n\n", query))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**\n", i+1, r.Title, r.URL))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// searchJSONOutput is the JSON output format for search results.
type searchJSONOutput struct {
	Query   string         `json:"query"`
	Engine  string         `json:"engine"`
	Results []searchResult `json:"results"`
}

// runSearch executes a web search using the specified engine.
func runSearch(query string, engineName string, maxResults int) error {
	eng, ok := engines[engineName]
	if !ok {
		return fmt.Errorf("unknown search engine: %s", engineName)
	}

	searchURL := eng.SearchURL(query, maxResults)

	result, err := fetchOne(fetchOptions{
		url:           searchURL,
		browser:       flagBrowser,
		headers:       flagHeaders,
		timeout:       flagTimeout,
		noCookies:     flagNoCookies,
		cookieJarPath: flagCookieJarPath,
		verbose:       flagVerbose,
	})
	if err != nil {
		return fmt.Errorf("search fetch failed: %w", err)
	}

	results := eng.Parse(result.Body)

	// Truncate to maxResults if needed.
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	if flagJSONOutput {
		out := searchJSONOutput{
			Query:   query,
			Engine:  engineName,
			Results: results,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Print(formatSearchResults(query, results))
	return nil
}
