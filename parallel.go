package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// runParallelFetch fetches multiple URLs concurrently using goroutines.
// Concurrency is limited by flagMaxParallel (default 5).
// Results are output in input-URL order, not completion order.
func runParallelFetch(urls []string) error {
	maxPar := flagMaxParallel
	if maxPar <= 0 {
		maxPar = 5
	}

	results := make([]fetchResult, len(urls))
	sem := make(chan struct{}, maxPar)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		go func(idx int, rawURL string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire semaphore slot
			defer func() { <-sem }() // release semaphore slot

			res, err := fetchOne(fetchOptions{
				url:       rawURL,
				browser:   flagBrowser,
				timeout:   flagTimeout,
				noCookies: flagNoCookies,
				verbose:   flagVerbose,
			})
			if err != nil {
				results[idx] = fetchResult{
					URL:   rawURL,
					Error: err,
				}
				return
			}
			results[idx] = *res
		}(i, u)
	}

	wg.Wait()

	opts := outputOptions{
		asJSON:       flagJSONOutput,
		markdown:     flagMarkdown,
		markdownFull: flagMarkdownFull,
	}

	if opts.asJSON {
		formatParallelJSON(os.Stdout, results, opts)
	} else {
		formatParallelResults(os.Stdout, results, opts)
	}

	return nil
}

// formatParallelResults writes results in text/markdown mode, separated by
// --- headers. Each result is preceded by a header block:
//
//	---
//	# Page: <url>
//	url: <url>
//	---
//
//	<content>
//
// Errors are shown as:
//
//	---
//	# Error: <url>
//	---
//
//	<error message>
func formatParallelResults(w io.Writer, results []fetchResult, opts outputOptions) {
	for i, r := range results {
		if r.Error != nil {
			fmt.Fprintf(w, "---\n# Error: %s\n---\n\n%s\n", r.URL, r.Error.Error())
		} else {
			content := string(r.Body)
			if opts.markdown || opts.markdownFull {
				readerMode := opts.markdown
				md, err := htmlToMarkdown(content, r.URL, readerMode)
				if err == nil {
					content = md
				}
			}
			fmt.Fprintf(w, "---\n# Page: %s\nurl: %s\n---\n\n%s\n", r.URL, r.URL, content)
		}
		// Add a blank line between results (but not after the last one).
		if i < len(results)-1 {
			fmt.Fprintln(w)
		}
	}
}

// parallelJSONEntry represents a single result in the JSON array output.
type parallelJSONEntry struct {
	URL     string              `json:"url"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// formatParallelJSON outputs a JSON array of result objects.
// Each object has url, status, headers, body, and error fields.
func formatParallelJSON(w io.Writer, results []fetchResult, opts outputOptions) {
	entries := make([]parallelJSONEntry, len(results))
	for i, r := range results {
		entry := parallelJSONEntry{
			URL:    r.URL,
			Status: r.StatusCode,
		}
		if r.Error != nil {
			entry.Error = r.Error.Error()
		} else {
			entry.Headers = r.Headers
			content := string(r.Body)
			if opts.markdown || opts.markdownFull {
				readerMode := opts.markdown
				md, err := htmlToMarkdown(content, r.URL, readerMode)
				if err == nil {
					content = md
				}
			}
			entry.Body = content
		}
		entries[i] = entry
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(entries)
}
