package main

import (
	"strings"
	"testing"
)

func TestHTMLToMarkdown(t *testing.T) {
	t.Run("converts basic HTML to markdown", func(t *testing.T) {
		html := `<h1>Hello</h1><p>This is a <strong>test</strong> with a <a href="https://example.com">link</a>.</p>`
		md, err := htmlToMarkdown(html, "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(md, "# Hello") {
			t.Fatalf("expected '# Hello' in output, got: %s", md)
		}
		if !strings.Contains(md, "**test**") {
			t.Fatalf("expected '**test**' in output, got: %s", md)
		}
		if !strings.Contains(md, "[link](https://example.com)") {
			t.Fatalf("expected markdown link in output, got: %s", md)
		}
	})

	t.Run("reader mode strips nav and footer", func(t *testing.T) {
		html := `<html><body>
			<nav><a href="/">Home</a><a href="/about">About</a></nav>
			<main><h1>Article Title</h1><p>Main content here.</p></main>
			<footer>Copyright 2024</footer>
		</body></html>`
		md, err := htmlToMarkdown(html, "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(md, "Article Title") {
			t.Fatalf("expected 'Article Title' in output, got: %s", md)
		}
		if !strings.Contains(md, "Main content") {
			t.Fatalf("expected 'Main content' in output, got: %s", md)
		}
		if strings.Contains(md, "Home") {
			t.Fatalf("expected nav to be stripped, but found 'Home' in: %s", md)
		}
		if strings.Contains(md, "Copyright") {
			t.Fatalf("expected footer to be stripped, but found 'Copyright' in: %s", md)
		}
	})

	t.Run("reader mode uses article tag when present", func(t *testing.T) {
		html := `<html><body>
			<div class="sidebar">Sidebar junk</div>
			<article><h2>Blog Post</h2><p>Content of the post.</p></article>
			<div class="ads">Buy stuff</div>
		</body></html>`
		md, err := htmlToMarkdown(html, "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(md, "Blog Post") {
			t.Fatalf("expected 'Blog Post' in output, got: %s", md)
		}
		if strings.Contains(md, "Sidebar junk") {
			t.Fatalf("expected sidebar to be excluded, got: %s", md)
		}
		if strings.Contains(md, "Buy stuff") {
			t.Fatalf("expected ads to be excluded, got: %s", md)
		}
	})

	t.Run("full mode keeps everything", func(t *testing.T) {
		html := `<html><body>
			<nav><a href="/">Home</a></nav>
			<main><p>Content</p></main>
			<footer>Footer text</footer>
		</body></html>`
		md, err := htmlToMarkdown(html, "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(md, "Home") {
			t.Fatalf("expected 'Home' in full mode, got: %s", md)
		}
		if !strings.Contains(md, "Content") {
			t.Fatalf("expected 'Content' in full mode, got: %s", md)
		}
	})

	t.Run("strips script and style in reader mode", func(t *testing.T) {
		html := `<html><body>
			<script>var x = 1;</script>
			<style>.foo { color: red; }</style>
			<main><p>Clean content</p></main>
		</body></html>`
		md, err := htmlToMarkdown(html, "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(md, "var x") {
			t.Fatalf("expected script to be stripped, got: %s", md)
		}
		if strings.Contains(md, "color") {
			t.Fatalf("expected style to be stripped, got: %s", md)
		}
		if !strings.Contains(md, "Clean content") {
			t.Fatalf("expected 'Clean content' in output, got: %s", md)
		}
	})
}
