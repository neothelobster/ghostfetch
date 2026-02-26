package main

import (
	"strings"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	t.Run("parses key:value headers", func(t *testing.T) {
		input := []string{"Content-Type: application/json", "X-Custom:value"}
		result := parseHeaders(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 headers, got %d", len(result))
		}
		if result[0][0] != "Content-Type" || result[0][1] != "application/json" {
			t.Fatalf("unexpected header: %v", result[0])
		}
	})

	t.Run("trims whitespace from key and value", func(t *testing.T) {
		input := []string{"  Authorization :  Bearer token123  "}
		result := parseHeaders(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 header, got %d", len(result))
		}
		if result[0][0] != "Authorization" || result[0][1] != "Bearer token123" {
			t.Fatalf("unexpected header: %v", result[0])
		}
	})

	t.Run("skips entries without colon", func(t *testing.T) {
		input := []string{"InvalidHeader", "Valid: header"}
		result := parseHeaders(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 header, got %d", len(result))
		}
		if result[0][0] != "Valid" || result[0][1] != "header" {
			t.Fatalf("unexpected header: %v", result[0])
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := parseHeaders(nil)
		if len(result) != 0 {
			t.Fatalf("expected 0 headers, got %d", len(result))
		}
	})

	t.Run("preserves value with colons", func(t *testing.T) {
		input := []string{"X-URL: https://example.com:8080/path"}
		result := parseHeaders(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 header, got %d", len(result))
		}
		if result[0][1] != "https://example.com:8080/path" {
			t.Fatalf("unexpected value: %q", result[0][1])
		}
	})
}

func TestExtractScriptContent(t *testing.T) {
	t.Run("extracts inline scripts", func(t *testing.T) {
		html := []byte(`<html><head><script>var x = 1;</script></head><body><script type="text/javascript">var y = 2;</script></body></html>`)
		result := extractScriptContent(html)
		if !strings.Contains(result, "var x = 1") {
			t.Fatalf("expected 'var x = 1' in result, got: %s", result)
		}
		if !strings.Contains(result, "var y = 2") {
			t.Fatalf("expected 'var y = 2' in result, got: %s", result)
		}
	})

	t.Run("returns empty for no scripts", func(t *testing.T) {
		html := []byte(`<html><body><p>Hello</p></body></html>`)
		result := extractScriptContent(html)
		if result != "" {
			t.Fatalf("expected empty result, got: %s", result)
		}
	})

	t.Run("skips script tags with src attribute", func(t *testing.T) {
		html := []byte(`<html><script src="external.js"></script><script>var inline = true;</script></html>`)
		result := extractScriptContent(html)
		if !strings.Contains(result, "var inline = true") {
			t.Fatalf("expected inline script content, got: %s", result)
		}
	})
}

func TestDefaultCookieJarPath(t *testing.T) {
	t.Run("returns path under home directory", func(t *testing.T) {
		path := defaultCookieJarPath()
		if !strings.HasSuffix(path, "/.web_search/cookies.json") {
			t.Fatalf("unexpected path: %s", path)
		}
	})
}
