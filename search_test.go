package main

import (
	"strings"
	"testing"
)

func TestParseGoogleResults(t *testing.T) {
	htmlBody := `<html><body>
<div class="g"><div><a href="https://example.com/first"><h3>First Result</h3></a></div>
<div class="VwiC3b">This is the first snippet</div></div>
<div class="g"><div><a href="https://example.com/second"><h3>Second Result</h3></a></div>
<div class="VwiC3b">This is the second snippet</div></div>
</body></html>`

	results := parseGoogleResults([]byte(htmlBody))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result.
	if results[0].Title != "First Result" {
		t.Errorf("result[0].Title = %q, want %q", results[0].Title, "First Result")
	}
	if results[0].URL != "https://example.com/first" {
		t.Errorf("result[0].URL = %q, want %q", results[0].URL, "https://example.com/first")
	}
	if results[0].Snippet != "This is the first snippet" {
		t.Errorf("result[0].Snippet = %q, want %q", results[0].Snippet, "This is the first snippet")
	}

	// Second result.
	if results[1].Title != "Second Result" {
		t.Errorf("result[1].Title = %q, want %q", results[1].Title, "Second Result")
	}
	if results[1].URL != "https://example.com/second" {
		t.Errorf("result[1].URL = %q, want %q", results[1].URL, "https://example.com/second")
	}
	if results[1].Snippet != "This is the second snippet" {
		t.Errorf("result[1].Snippet = %q, want %q", results[1].Snippet, "This is the second snippet")
	}
}

func TestParseBingResults(t *testing.T) {
	htmlBody := `<html><body>
<li class="b_algo"><h2><a href="https://example.com/bing1">Bing First</a></h2>
<div class="b_caption"><p>Bing first snippet</p></div></li>
<li class="b_algo"><h2><a href="https://example.com/bing2">Bing Second</a></h2>
<div class="b_caption"><p>Bing second snippet</p></div></li>
</body></html>`

	results := parseBingResults([]byte(htmlBody))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result.
	if results[0].Title != "Bing First" {
		t.Errorf("result[0].Title = %q, want %q", results[0].Title, "Bing First")
	}
	if results[0].URL != "https://example.com/bing1" {
		t.Errorf("result[0].URL = %q, want %q", results[0].URL, "https://example.com/bing1")
	}
	if results[0].Snippet != "Bing first snippet" {
		t.Errorf("result[0].Snippet = %q, want %q", results[0].Snippet, "Bing first snippet")
	}

	// Second result.
	if results[1].Title != "Bing Second" {
		t.Errorf("result[1].Title = %q, want %q", results[1].Title, "Bing Second")
	}
	if results[1].URL != "https://example.com/bing2" {
		t.Errorf("result[1].URL = %q, want %q", results[1].URL, "https://example.com/bing2")
	}
	if results[1].Snippet != "Bing second snippet" {
		t.Errorf("result[1].Snippet = %q, want %q", results[1].Snippet, "Bing second snippet")
	}
}

func TestParseDuckDuckGoResults(t *testing.T) {
	// Uses DDG redirect URLs to verify cleanDDGURL extracts real destinations.
	htmlBody := `<html><body>
<div class="result"><h2 class="result__title"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fddg1&amp;rut=abc123">DDG First</a></h2>
<a class="result__snippet">DDG first snippet</a></div>
<div class="result"><h2 class="result__title"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fddg2&amp;rut=def456">DDG Second</a></h2>
<a class="result__snippet">DDG second snippet</a></div>
</body></html>`

	results := parseDuckDuckGoResults([]byte(htmlBody))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result.
	if results[0].Title != "DDG First" {
		t.Errorf("result[0].Title = %q, want %q", results[0].Title, "DDG First")
	}
	if results[0].URL != "https://example.com/ddg1" {
		t.Errorf("result[0].URL = %q, want %q", results[0].URL, "https://example.com/ddg1")
	}
	if results[0].Snippet != "DDG first snippet" {
		t.Errorf("result[0].Snippet = %q, want %q", results[0].Snippet, "DDG first snippet")
	}

	// Second result.
	if results[1].Title != "DDG Second" {
		t.Errorf("result[1].Title = %q, want %q", results[1].Title, "DDG Second")
	}
	if results[1].URL != "https://example.com/ddg2" {
		t.Errorf("result[1].URL = %q, want %q", results[1].URL, "https://example.com/ddg2")
	}
	if results[1].Snippet != "DDG second snippet" {
		t.Errorf("result[1].Snippet = %q, want %q", results[1].Snippet, "DDG second snippet")
	}
}

func TestParseBraveResults(t *testing.T) {
	// Matches real Brave HTML structure: <a class="l1"> wraps <div class="title search-snippet-title">,
	// and description is in <div class="content line-clamp-dynamic"> inside <div class="generic-snippet">.
	htmlBody := `<html><body>
<div class="snippet svelte-jmfu5f" data-type="web"><div class="result-content svelte-1rq4ngz">
<a href="https://example.com/brave1" class="svelte-14r20fy l1">
<div class="title search-snippet-title line-clamp-1 svelte-14r20fy">Brave First</div></a>
<div class="generic-snippet svelte-1cwdgg3"><div class="content desktop-default-regular t-primary line-clamp-dynamic svelte-1cwdgg3">Brave first snippet</div></div>
</div></div>
<div class="snippet svelte-jmfu5f" data-type="web"><div class="result-content svelte-1rq4ngz">
<a href="https://example.com/brave2" class="svelte-14r20fy l1">
<div class="title search-snippet-title line-clamp-1 svelte-14r20fy">Brave Second</div></a>
<div class="generic-snippet svelte-1cwdgg3"><div class="content desktop-default-regular t-primary line-clamp-dynamic svelte-1cwdgg3">Brave second snippet</div></div>
</div></div>
</body></html>`

	results := parseBraveResults([]byte(htmlBody))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result.
	if results[0].Title != "Brave First" {
		t.Errorf("result[0].Title = %q, want %q", results[0].Title, "Brave First")
	}
	if results[0].URL != "https://example.com/brave1" {
		t.Errorf("result[0].URL = %q, want %q", results[0].URL, "https://example.com/brave1")
	}
	if results[0].Snippet != "Brave first snippet" {
		t.Errorf("result[0].Snippet = %q, want %q", results[0].Snippet, "Brave first snippet")
	}

	// Second result.
	if results[1].Title != "Brave Second" {
		t.Errorf("result[1].Title = %q, want %q", results[1].Title, "Brave Second")
	}
	if results[1].URL != "https://example.com/brave2" {
		t.Errorf("result[1].URL = %q, want %q", results[1].URL, "https://example.com/brave2")
	}
	if results[1].Snippet != "Brave second snippet" {
		t.Errorf("result[1].Snippet = %q, want %q", results[1].Snippet, "Brave second snippet")
	}
}

func TestFormatSearchResults(t *testing.T) {
	results := []searchResult{
		{Title: "First", URL: "https://example.com/1", Snippet: "Snippet one"},
		{Title: "Second", URL: "https://example.com/2", Snippet: "Snippet two"},
	}

	output := formatSearchResults("test query", results)

	// Check header.
	if !strings.Contains(output, `## Search: "test query"`) {
		t.Errorf("output missing header, got:\n%s", output)
	}

	// Check numbered items.
	if !strings.Contains(output, "1. **[First](https://example.com/1)**") {
		t.Errorf("output missing first numbered item, got:\n%s", output)
	}
	if !strings.Contains(output, "2. **[Second](https://example.com/2)**") {
		t.Errorf("output missing second numbered item, got:\n%s", output)
	}

	// Check snippets.
	if !strings.Contains(output, "Snippet one") {
		t.Errorf("output missing first snippet, got:\n%s", output)
	}
	if !strings.Contains(output, "Snippet two") {
		t.Errorf("output missing second snippet, got:\n%s", output)
	}
}
