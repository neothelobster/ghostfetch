package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFormatParallelResults(t *testing.T) {
	t.Run("two results produce separated output with ---", func(t *testing.T) {
		results := []fetchResult{
			{
				URL:        "https://example.com/a",
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"text/html"}},
				Body:       []byte("<p>Page A content</p>"),
			},
			{
				URL:        "https://example.com/b",
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"text/html"}},
				Body:       []byte("<p>Page B content</p>"),
			},
		}
		var buf bytes.Buffer
		formatParallelResults(&buf, results, outputOptions{})
		output := buf.String()

		// Verify both URL headers are present.
		if !strings.Contains(output, "# Page: https://example.com/a") {
			t.Fatalf("missing header for URL A in output:\n%s", output)
		}
		if !strings.Contains(output, "# Page: https://example.com/b") {
			t.Fatalf("missing header for URL B in output:\n%s", output)
		}

		// Verify separator lines.
		if strings.Count(output, "---") < 4 {
			t.Fatalf("expected at least 4 --- separators, got %d in output:\n%s",
				strings.Count(output, "---"), output)
		}

		// Verify content is present.
		if !strings.Contains(output, "<p>Page A content</p>") {
			t.Fatalf("missing Page A content in output:\n%s", output)
		}
		if !strings.Contains(output, "<p>Page B content</p>") {
			t.Fatalf("missing Page B content in output:\n%s", output)
		}

		// Verify url: lines.
		if !strings.Contains(output, "url: https://example.com/a") {
			t.Fatalf("missing url: line for URL A in output:\n%s", output)
		}
		if !strings.Contains(output, "url: https://example.com/b") {
			t.Fatalf("missing url: line for URL B in output:\n%s", output)
		}

		// Verify order: A comes before B.
		idxA := strings.Index(output, "# Page: https://example.com/a")
		idxB := strings.Index(output, "# Page: https://example.com/b")
		if idxA >= idxB {
			t.Fatalf("expected URL A before URL B in output:\n%s", output)
		}
	})
}

func TestFormatParallelResultsWithError(t *testing.T) {
	t.Run("errors are included inline", func(t *testing.T) {
		results := []fetchResult{
			{
				URL:        "https://example.com/good",
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"text/html"}},
				Body:       []byte("<p>Good page</p>"),
			},
			{
				URL:   "https://example.com/bad",
				Error: fmt.Errorf("connection refused"),
			},
		}
		var buf bytes.Buffer
		formatParallelResults(&buf, results, outputOptions{})
		output := buf.String()

		// Verify successful result.
		if !strings.Contains(output, "# Page: https://example.com/good") {
			t.Fatalf("missing header for good URL in output:\n%s", output)
		}
		if !strings.Contains(output, "<p>Good page</p>") {
			t.Fatalf("missing good page content in output:\n%s", output)
		}

		// Verify error result.
		if !strings.Contains(output, "# Error: https://example.com/bad") {
			t.Fatalf("missing error header for bad URL in output:\n%s", output)
		}
		if !strings.Contains(output, "connection refused") {
			t.Fatalf("missing error message in output:\n%s", output)
		}
	})
}

func TestFormatParallelJSON(t *testing.T) {
	t.Run("outputs JSON array", func(t *testing.T) {
		results := []fetchResult{
			{
				URL:        "https://example.com/one",
				StatusCode: 200,
				Headers:    http.Header{"Content-Type": []string{"text/html"}},
				Body:       []byte("<p>First</p>"),
			},
			{
				URL:   "https://example.com/two",
				Error: fmt.Errorf("timeout"),
			},
		}
		var buf bytes.Buffer
		formatParallelJSON(&buf, results, outputOptions{})

		var entries []parallelJSONEntry
		if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
			t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}

		// First entry: success.
		if entries[0].URL != "https://example.com/one" {
			t.Fatalf("unexpected URL for entry 0: %s", entries[0].URL)
		}
		if entries[0].Status != 200 {
			t.Fatalf("unexpected status for entry 0: %d", entries[0].Status)
		}
		if entries[0].Body != "<p>First</p>" {
			t.Fatalf("unexpected body for entry 0: %s", entries[0].Body)
		}
		if entries[0].Error != "" {
			t.Fatalf("unexpected error for entry 0: %s", entries[0].Error)
		}
		if entries[0].Headers == nil {
			t.Fatal("expected headers for entry 0, got nil")
		}

		// Second entry: error.
		if entries[1].URL != "https://example.com/two" {
			t.Fatalf("unexpected URL for entry 1: %s", entries[1].URL)
		}
		if entries[1].Error != "timeout" {
			t.Fatalf("unexpected error for entry 1: %s", entries[1].Error)
		}
		if entries[1].Status != 0 {
			t.Fatalf("expected status 0 for error entry, got %d", entries[1].Status)
		}
		if entries[1].Body != "" {
			t.Fatalf("expected empty body for error entry, got %s", entries[1].Body)
		}
	})
}
