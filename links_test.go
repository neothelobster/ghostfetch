package main

import (
	"strings"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	t.Run("absolute links", func(t *testing.T) {
		htmlBody := `<html><body>
<a href="https://example.com/page1">Page One</a>
<a href="https://example.com/page2">Page Two</a>
</body></html>`

		links := extractLinks([]byte(htmlBody), "https://example.com")

		if len(links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(links))
		}

		if links[0].URL != "https://example.com/page1" {
			t.Errorf("links[0].URL = %q, want %q", links[0].URL, "https://example.com/page1")
		}
		if links[0].Text != "Page One" {
			t.Errorf("links[0].Text = %q, want %q", links[0].Text, "Page One")
		}

		if links[1].URL != "https://example.com/page2" {
			t.Errorf("links[1].URL = %q, want %q", links[1].URL, "https://example.com/page2")
		}
		if links[1].Text != "Page Two" {
			t.Errorf("links[1].Text = %q, want %q", links[1].Text, "Page Two")
		}
	})

	t.Run("relative link resolution", func(t *testing.T) {
		htmlBody := `<html><body>
<a href="/about">About Us</a>
</body></html>`

		links := extractLinks([]byte(htmlBody), "https://example.com")

		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(links))
		}

		if links[0].URL != "https://example.com/about" {
			t.Errorf("links[0].URL = %q, want %q", links[0].URL, "https://example.com/about")
		}
		if links[0].Text != "About Us" {
			t.Errorf("links[0].Text = %q, want %q", links[0].Text, "About Us")
		}
	})

	t.Run("skip fragment-only and javascript links", func(t *testing.T) {
		htmlBody := `<html><body>
<a href="#section1">Section 1</a>
<a href="javascript:void(0)">Do Nothing</a>
<a href="javascript:alert('hi')">Alert</a>
<a href="">Empty</a>
<a href="https://example.com/real">Real Link</a>
</body></html>`

		links := extractLinks([]byte(htmlBody), "https://example.com")

		if len(links) != 1 {
			t.Fatalf("expected 1 link (only real link), got %d", len(links))
		}

		if links[0].URL != "https://example.com/real" {
			t.Errorf("links[0].URL = %q, want %q", links[0].URL, "https://example.com/real")
		}
		if links[0].Text != "Real Link" {
			t.Errorf("links[0].Text = %q, want %q", links[0].Text, "Real Link")
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		htmlBody := `<html><body>
<a href="https://example.com/page1">First</a>
<a href="https://example.com/page1">Duplicate</a>
</body></html>`

		links := extractLinks([]byte(htmlBody), "https://example.com")

		if len(links) != 1 {
			t.Fatalf("expected 1 link after dedup, got %d", len(links))
		}

		if links[0].Text != "First" {
			t.Errorf("links[0].Text = %q, want %q (should keep first occurrence)", links[0].Text, "First")
		}
	})
}

func TestFormatLinks(t *testing.T) {
	links := []pageLink{
		{URL: "https://example.com/page1", Text: "Page One"},
		{URL: "https://example.com/page2", Text: "Page Two"},
		{URL: "https://example.com/page3", Text: ""},
	}

	output := formatLinks(links)

	if !strings.Contains(output, "- [Page One](https://example.com/page1)") {
		t.Errorf("output missing first link, got:\n%s", output)
	}
	if !strings.Contains(output, "- [Page Two](https://example.com/page2)") {
		t.Errorf("output missing second link, got:\n%s", output)
	}
	// When text is empty, URL should be used as text.
	if !strings.Contains(output, "- [https://example.com/page3](https://example.com/page3)") {
		t.Errorf("output missing third link with URL as text, got:\n%s", output)
	}
}
