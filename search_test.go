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
