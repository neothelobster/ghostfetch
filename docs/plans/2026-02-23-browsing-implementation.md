# LLM-Friendly Browsing Subcommands Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `fetch` (parallel), `search`, and `links` subcommands to brwoser for LLM-driven web research.

**Architecture:** Restructure main.go from a single root command to cobra subcommands. Extract the existing single-URL fetch logic into a reusable `fetchOne()` function. Add parallel orchestration, search engine parsers, and link extraction as separate files.

**Tech Stack:** Go, cobra subcommands, goroutines + sync.WaitGroup for parallelism, regexp for HTML parsing of search results, golang.org/x/net/html for link extraction.

---

### Task 1: Extract `fetchOne()` from `run()`

Refactor the existing fetch pipeline (steps 1-13 in `run()`) into a reusable `fetchOne()` function that other subcommands can call.

**Files:**
- Create: `fetch.go`
- Create: `fetch_test.go`
- Modify: `main.go:104-302` (move fetch logic out)

**Step 1: Write the failing test**

Create `fetch_test.go`:

```go
package main

import (
	"testing"
)

func TestFetchOne(t *testing.T) {
	t.Run("returns error for invalid timeout", func(t *testing.T) {
		_, err := fetchOne(fetchOptions{
			url:     "https://example.com",
			timeout: "invalid",
			browser: "chrome",
		})
		if err == nil {
			t.Fatal("expected error for invalid timeout")
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test -run TestFetchOne -v`
Expected: FAIL — `fetchOne` undefined

**Step 3: Create `fetch.go` with `fetchOne()`**

Create `fetch.go` by extracting the core fetch pipeline from `main.go:run()`. The function signature:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type fetchOptions struct {
	url            string
	browser        string
	headers        []string
	timeout        string
	noCookies      bool
	cookieJarPath  string
	verbose        bool
	method         string
	data           string
	captchaService string
	captchaKey     string
}

type fetchResult struct {
	URL        string
	StatusCode int
	Headers    http.Header
	Body       []byte
	Error      error
}

// fetchOne performs a single URL fetch with the full anti-detection pipeline:
// TLS spoofing, challenge detection/solving, captcha handling, cookie persistence.
func fetchOne(opts fetchOptions) (*fetchResult, error) {
	// Parse URL
	targetURL := opts.url
	if !strings.Contains(targetURL, "://") {
		targetURL = "https://" + targetURL
	}

	// Parse timeout
	timeout := opts.timeout
	if timeout == "" {
		timeout = "30s"
	}
	dur, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout %q: %w", timeout, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	profile := getProfile(opts.browser)
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Using %s profile\n", profile.Name)
	}

	tr, err := newTransport(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Load cookie jar
	var jar *PersistentJar
	if !opts.noCookies {
		jarPath := opts.cookieJarPath
		if jarPath == "" {
			jarPath = defaultCookieJarPath()
		}
		jar = newPersistentJar(jarPath)
		if err := jar.Load(); err != nil {
			return nil, fmt.Errorf("failed to load cookie jar: %w", err)
		}
	}

	extraHeaders := parseHeaders(opts.headers)

	var cookies []*http.Cookie
	if jar != nil {
		if u, err := url.Parse(targetURL); err == nil {
			cookies = jar.Cookies(u)
		}
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Fetching %s\n", targetURL)
	}

	// Perform fetch
	var resp *http.Response
	var body []byte
	if opts.data != "" {
		resp, body, err = doFetchWithBody(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies, opts.data)
	} else {
		method := opts.method
		if method == "" {
			method = "GET"
		}
		resp, body, err = doFetch(ctx, tr, profile, method, targetURL, extraHeaders, cookies)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Challenge detection + solving (same logic as current run())
	challenge := detectChallenge(resp, body)
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Challenge: %s\n", challenge)
	}

	if challenge == ChallengeJS {
		script := extractScriptContent(body)
		if script != "" {
			solver := newJSSolver(targetURL)
			result, solveErr := solver.Solve(script)
			if solveErr != nil {
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] JS solver error: %v\n", solveErr)
				}
			} else if result.CookieName != "" {
				solvedCookie := &http.Cookie{Name: result.CookieName, Value: result.CookieValue}
				cookies = append(cookies, solvedCookie)
				if jar != nil {
					if u, err := url.Parse(targetURL); err == nil {
						jar.SetCookies(u, []*http.Cookie{solvedCookie})
					}
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Retrying with solved JS cookie: %s\n", result.CookieName)
				}
				resp, body, err = doFetch(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies)
				if err != nil {
					return nil, fmt.Errorf("retry fetch failed: %w", err)
				}
			}
		}
	}

	if challenge == ChallengeCaptcha {
		sitekey, captchaType := extractSitekey(body)
		if sitekey != "" {
			svc := opts.captchaService
			if svc == "" {
				svc = os.Getenv("BRWOSER_CAPTCHA_SERVICE")
			}
			key := opts.captchaKey
			if key == "" {
				key = os.Getenv("BRWOSER_CAPTCHA_KEY")
			}
			if svc != "" && key != "" {
				captchaSolver, solverErr := newCaptchaSolver(svc, key)
				if solverErr != nil {
					return nil, fmt.Errorf("captcha solver init failed: %w", solverErr)
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Solving %s captcha via %s\n", captchaType, svc)
				}
				token, solveErr := captchaSolver.Solve(ctx, sitekey, targetURL, captchaType)
				if solveErr != nil {
					return nil, fmt.Errorf("captcha solve failed: %w", solveErr)
				}
				solvedCookie := &http.Cookie{Name: "cf_clearance", Value: token}
				cookies = append(cookies, solvedCookie)
				if jar != nil {
					if u, err := url.Parse(targetURL); err == nil {
						jar.SetCookies(u, []*http.Cookie{solvedCookie})
					}
				}
				resp, body, err = doFetch(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies)
				if err != nil {
					return nil, fmt.Errorf("retry fetch after captcha failed: %w", err)
				}
			}
		}
	}

	// Save cookies
	if jar != nil {
		if resp != nil && resp.Request != nil && resp.Request.URL != nil {
			if respCookies := resp.Cookies(); len(respCookies) > 0 {
				jar.SetCookies(resp.Request.URL, respCookies)
			}
		}
		if saveErr := jar.Save(); saveErr != nil && opts.verbose {
			fmt.Fprintf(os.Stderr, "[*] Warning: failed to save cookies: %v\n", saveErr)
		}
	}

	return &fetchResult{
		URL:        targetURL,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}, nil
}
```

**Step 4: Update `main.go` — replace `run()` body with call to `fetchOne()`**

Replace the body of `run()` in `main.go` (lines 104-302) with:

```go
func run(rawURL string, opts runOptions) error {
	result, err := fetchOne(fetchOptions{
		url:            rawURL,
		browser:        opts.browser,
		headers:        opts.headers,
		timeout:        opts.timeout,
		noCookies:      opts.noCookies,
		cookieJarPath:  opts.cookieJarPath,
		verbose:        opts.verbose,
		method:         opts.method,
		data:           opts.data,
		captchaService: opts.captchaService,
		captchaKey:     opts.captchaKey,
	})
	if err != nil {
		return err
	}

	writer := os.Stdout
	if opts.outputFile != "" {
		f, err := os.Create(opts.outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	resp := &http.Response{
		StatusCode: result.StatusCode,
		Header:     result.Headers,
	}
	formatOutput(writer, resp, result.Body, outputOptions{
		asJSON:       opts.jsonOutput,
		markdown:     opts.markdown,
		markdownFull: opts.markdownFull,
		pageURL:      result.URL,
	})

	return nil
}
```

Remove unused imports from `main.go` that were moved to `fetch.go`: `context`, `net/http`, `net/url`, `time`. Keep `fmt`, `os`, `strings`, `regexp`, `path/filepath`, `cobra`.

**Step 5: Run all tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All tests PASS (existing tests still work because `run()` delegates to `fetchOne()`)

**Step 6: Commit**

```bash
git add fetch.go fetch_test.go main.go
git commit -m "refactor: extract fetchOne() from run() for reuse by subcommands"
```

---

### Task 2: Restructure CLI with cobra subcommands

Change the CLI from a single root command to subcommands: `fetch`, `search`, `links`. Bare URL still works via root command.

**Files:**
- Modify: `main.go` (restructure with subcommands)

**Step 1: Restructure `main.go`**

Replace the root command with subcommand structure. The key changes:

1. Root command becomes the parent with `TraverseChildren: true`
2. Add `fetchCmd` subcommand accepting 1+ URL args
3. Root command's `RunE` handles bare `brwoser <url>` as shortcut to fetch
4. Add persistent flags (shared across subcommands) on root
5. Add `searchCmd` and `linksCmd` as placeholders (implemented in later tasks)

```go
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Shared flags (persistent across subcommands)
var (
	flagOutputFile     string
	flagHeaders        []string
	flagBrowser        string
	flagJSONOutput     bool
	flagFollowRedirs   bool
	flagCookieJarPath  string
	flagNoCookies      bool
	flagTimeout        string
	flagVerbose        bool
	flagMethod         string
	flagData           string
	flagCaptchaService string
	flagCaptchaKey     string
	flagMarkdown       bool
	flagMarkdownFull   bool
	flagMaxParallel    int
	flagRaw            bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "brwoser [command]",
		Short: "Fetch web pages like curl, but bypass bot detection",
		Long: `brwoser fetches web pages with browser-like TLS fingerprints,
solves JavaScript challenges, and handles captchas via external services.
It bypasses bot detection without running a full browser.`,
		// Allow bare: brwoser <url> as shortcut for brwoser fetch <url>
		Args:                  cobra.ArbitraryArgs,
		TraverseChildren:      true,
		DisableFlagParsing:    false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Treat bare URL as "fetch"
			return runFetch(args)
		},
	}

	// Persistent flags (available to all subcommands)
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagOutputFile, "output", "o", "", "write response to file")
	pf.StringArrayVarP(&flagHeaders, "header", "H", nil, "add custom header (repeatable)")
	pf.StringVarP(&flagBrowser, "browser", "b", "chrome", "browser to impersonate: chrome, firefox")
	pf.BoolVarP(&flagJSONOutput, "json", "j", false, "output JSON with body, status, headers")
	pf.BoolVarP(&flagFollowRedirs, "follow", "L", true, "follow redirects (up to 10)")
	pf.StringVarP(&flagCookieJarPath, "cookie-jar", "c", "", "cookie jar file path")
	pf.BoolVar(&flagNoCookies, "no-cookies", false, "don't load/save cookies")
	pf.StringVarP(&flagTimeout, "timeout", "t", "30s", "request timeout")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "print details to stderr")
	pf.StringVarP(&flagMethod, "method", "X", "GET", "HTTP method")
	pf.StringVarP(&flagData, "data", "d", "", "request body")
	pf.StringVar(&flagCaptchaService, "captcha-service", "", "captcha service: 2captcha, anticaptcha")
	pf.StringVar(&flagCaptchaKey, "captcha-key", "", "captcha service API key")
	pf.BoolVarP(&flagMarkdown, "markdown", "m", false, "convert to markdown (reader mode)")
	pf.BoolVar(&flagMarkdownFull, "markdown-full", false, "convert full page to markdown")
	pf.BoolVar(&flagRaw, "raw", false, "output raw HTML (no conversion)")

	// fetch subcommand
	fetchCmd := &cobra.Command{
		Use:   "fetch <url> [url2] [url3...]",
		Short: "Fetch one or more URLs (parallel for multiple)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(args)
		},
	}
	fetchCmd.Flags().IntVarP(&flagMaxParallel, "max-parallel", "p", 5, "max concurrent fetches")
	rootCmd.AddCommand(fetchCmd)

	// search subcommand (placeholder — implemented in Task 4)
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the web and return results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("search not yet implemented")
		},
	}
	rootCmd.AddCommand(searchCmd)

	// links subcommand (placeholder — implemented in Task 6)
	linksCmd := &cobra.Command{
		Use:   "links <url>",
		Short: "Extract all links from a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("links not yet implemented")
		},
	}
	rootCmd.AddCommand(linksCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runFetch handles both single and multi-URL fetch.
// For a single URL, it outputs directly.
// For multiple URLs, it fetches in parallel and outputs each with a separator.
func runFetch(urls []string) error {
	if len(urls) == 1 {
		return runSingleFetch(urls[0])
	}
	return runParallelFetch(urls)
}

// runSingleFetch fetches a single URL (same as old run()).
func runSingleFetch(rawURL string) error {
	result, err := fetchOne(fetchOptions{
		url:            rawURL,
		browser:        flagBrowser,
		headers:        flagHeaders,
		timeout:        flagTimeout,
		noCookies:      flagNoCookies,
		cookieJarPath:  flagCookieJarPath,
		verbose:        flagVerbose,
		method:         flagMethod,
		data:           flagData,
		captchaService: flagCaptchaService,
		captchaKey:     flagCaptchaKey,
	})
	if err != nil {
		return err
	}

	writer := os.Stdout
	if flagOutputFile != "" {
		f, err := os.Create(flagOutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	resp := &http.Response{
		StatusCode: result.StatusCode,
		Header:     result.Headers,
	}
	formatOutput(writer, resp, result.Body, outputOptions{
		asJSON:       flagJSONOutput,
		markdown:     flagMarkdown,
		markdownFull: flagMarkdownFull,
		pageURL:      result.URL,
	})

	return nil
}
```

Remove the old `run()`, `runOptions`, and related code from `main.go`. Keep `parseHeaders()`, `defaultCookieJarPath()`, `extractScriptContent()` and the regexp vars.

**Step 2: Run all tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add main.go
git commit -m "refactor: restructure CLI with cobra subcommands (fetch, search, links)"
```

---

### Task 3: Parallel fetch

Implement `runParallelFetch()` for fetching multiple URLs concurrently.

**Files:**
- Create: `parallel.go`
- Create: `parallel_test.go`

**Step 1: Write the failing test**

Create `parallel_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunParallelFetchFormatting(t *testing.T) {
	// Create test servers
	srv1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><p>Page One</p></body></html>"))
	}))
	defer srv1.Close()

	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><p>Page Two</p></body></html>"))
	}))
	defer srv2.Close()

	t.Run("formatParallelResults produces separated output", func(t *testing.T) {
		results := []fetchResult{
			{URL: "https://example.com/1", StatusCode: 200, Body: []byte("content one")},
			{URL: "https://example.com/2", StatusCode: 200, Body: []byte("content two")},
		}
		var buf strings.Builder
		formatParallelResults(&buf, results, outputOptions{})
		output := buf.String()
		if !strings.Contains(output, "content one") {
			t.Fatalf("missing content one in output: %s", output)
		}
		if !strings.Contains(output, "content two") {
			t.Fatalf("missing content two in output: %s", output)
		}
		if !strings.Contains(output, "---") {
			t.Fatalf("missing separator in output: %s", output)
		}
	})

	t.Run("formatParallelResults includes errors inline", func(t *testing.T) {
		results := []fetchResult{
			{URL: "https://example.com/ok", StatusCode: 200, Body: []byte("ok")},
			{URL: "https://example.com/fail", Error: fmt.Errorf("connection refused")},
		}
		var buf strings.Builder
		formatParallelResults(&buf, results, outputOptions{})
		output := buf.String()
		if !strings.Contains(output, "connection refused") {
			t.Fatalf("missing error in output: %s", output)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test -run TestRunParallelFetch -v`
Expected: FAIL — `formatParallelResults` undefined

**Step 3: Create `parallel.go`**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// runParallelFetch fetches multiple URLs concurrently and outputs all results.
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
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			res, err := fetchOne(fetchOptions{
				url:            rawURL,
				browser:        flagBrowser,
				headers:        flagHeaders,
				timeout:        flagTimeout,
				noCookies:      flagNoCookies,
				cookieJarPath:  flagCookieJarPath,
				verbose:        flagVerbose,
				method:         flagMethod,
				data:           flagData,
				captchaService: flagCaptchaService,
				captchaKey:     flagCaptchaKey,
			})
			if err != nil {
				results[idx] = fetchResult{URL: rawURL, Error: err}
			} else {
				results[idx] = *res
			}
		}(i, u)
	}

	wg.Wait()

	writer := os.Stdout
	if flagOutputFile != "" {
		f, err := os.Create(flagOutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	opts := outputOptions{
		asJSON:       flagJSONOutput,
		markdown:     flagMarkdown,
		markdownFull: flagMarkdownFull,
	}
	formatParallelResults(writer, results, opts)

	return nil
}

// formatParallelResults writes multiple fetch results to the writer.
func formatParallelResults(w io.Writer, results []fetchResult, opts outputOptions) {
	if opts.asJSON {
		formatParallelJSON(w, results, opts)
		return
	}

	for i, r := range results {
		if i > 0 {
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "---\n")
		if r.Error != nil {
			fmt.Fprintf(w, "# Error: %s\nurl: %s\n---\n\n%s\n", r.URL, r.URL, r.Error.Error())
			continue
		}
		fmt.Fprintf(w, "# Page: %s\nurl: %s\n---\n\n", r.URL, r.URL)

		content := string(r.Body)
		if opts.markdown || opts.markdownFull {
			readerMode := opts.markdown
			if md, err := htmlToMarkdown(content, r.URL, readerMode); err == nil {
				content = md
			}
		}
		fmt.Fprintf(w, "%s\n", content)
	}
}

// formatParallelJSON outputs all results as a JSON array.
func formatParallelJSON(w io.Writer, results []fetchResult, opts outputOptions) {
	type jsonResult struct {
		URL     string              `json:"url"`
		Status  int                 `json:"status,omitempty"`
		Headers map[string][]string `json:"headers,omitempty"`
		Body    string              `json:"body,omitempty"`
		Error   string              `json:"error,omitempty"`
	}

	var out []jsonResult
	for _, r := range results {
		if r.Error != nil {
			out = append(out, jsonResult{URL: r.URL, Error: r.Error.Error()})
			continue
		}
		content := string(r.Body)
		if opts.markdown || opts.markdownFull {
			readerMode := opts.markdown
			if md, err := htmlToMarkdown(content, r.URL, readerMode); err == nil {
				content = md
			}
		}
		out = append(out, jsonResult{
			URL:     r.URL,
			Status:  r.StatusCode,
			Headers: r.Headers,
			Body:    content,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
```

**Step 4: Run tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add parallel.go parallel_test.go
git commit -m "feat: add parallel fetch for multiple URLs"
```

---

### Task 4: Search engine abstraction + Google parser

Implement the search subcommand with Google as the first engine.

**Files:**
- Create: `search.go`
- Create: `search_test.go`
- Modify: `main.go` (wire up search subcommand)

**Step 1: Write the failing test**

Create `search_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestParseGoogleResults(t *testing.T) {
	// Minimal Google-like HTML with search results
	html := `<html><body>
<div class="g"><div><a href="https://example.com/first"><h3>First Result</h3></a></div>
<div class="VwiC3b">This is the first snippet</div></div>
<div class="g"><div><a href="https://example.com/second"><h3>Second Result</h3></a></div>
<div class="VwiC3b">This is the second snippet</div></div>
</body></html>`

	results := parseGoogleResults([]byte(html))
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].URL != "https://example.com/first" {
		t.Fatalf("unexpected first URL: %s", results[0].URL)
	}
	if results[0].Title != "First Result" {
		t.Fatalf("unexpected first title: %s", results[0].Title)
	}
}

func TestFormatSearchResults(t *testing.T) {
	results := []searchResult{
		{Title: "First", URL: "https://example.com/1", Snippet: "First snippet"},
		{Title: "Second", URL: "https://example.com/2", Snippet: "Second snippet"},
	}
	output := formatSearchResults("test query", results)
	if !strings.Contains(output, "## Search: \"test query\"") {
		t.Fatalf("missing header in output: %s", output)
	}
	if !strings.Contains(output, "1. **[First](https://example.com/1)**") {
		t.Fatalf("missing first result in output: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test -run TestParseGoogle -v`
Expected: FAIL — `parseGoogleResults` undefined

**Step 3: Create `search.go`**

```go
package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

type searchEngine struct {
	Name      string
	SearchURL func(query string, maxResults int) string
	Parse     func(body []byte) []searchResult
}

var engines = map[string]searchEngine{
	"google": {
		Name: "Google",
		SearchURL: func(query string, maxResults int) string {
			return fmt.Sprintf("https://www.google.com/search?q=%s&num=%d&hl=en", url.QueryEscape(query), maxResults)
		},
		Parse: parseGoogleResults,
	},
}

// parseGoogleResults extracts search results from Google HTML.
// Google wraps each result in <div class="g"> with an <a> containing <h3> for title,
// and a <div class="VwiC3b"> (or similar) for the snippet.
func parseGoogleResults(body []byte) []searchResult {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "g") {
			r := extractGoogleResult(n)
			if r.URL != "" && r.Title != "" {
				results = append(results, r)
			}
			return // don't recurse into result divs
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results
}

func extractGoogleResult(n *html.Node) searchResult {
	var r searchResult
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "a" && r.URL == "" {
				for _, attr := range node.Attr {
					if attr.Key == "href" && strings.HasPrefix(attr.Val, "http") {
						r.URL = attr.Val
					}
				}
			}
			if node.Data == "h3" && r.Title == "" {
				r.Title = textContent(node)
			}
			if node.Data == "div" && (hasClass(node, "VwiC3b") || hasClass(node, "IsZvec")) {
				if r.Snippet == "" {
					r.Snippet = textContent(node)
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return r
}

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

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return strings.TrimSpace(sb.String())
}

// formatSearchResults formats results as a numbered markdown list.
func formatSearchResults(query string, results []searchResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Search: %q\n\n", query)
	for i, r := range results {
		fmt.Fprintf(&sb, "%d. **[%s](%s)**\n", i+1, r.Title, r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&sb, "   %s\n", r.Snippet)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// runSearch fetches search results and outputs them.
func runSearch(query string, engine string, maxResults int) error {
	eng, ok := engines[engine]
	if !ok {
		return fmt.Errorf("unknown search engine: %s (available: google, bing, duckduckgo, brave)", engine)
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
		method:        "GET",
	})
	if err != nil {
		return fmt.Errorf("search fetch failed: %w", err)
	}

	results := eng.Parse(result.Body)
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	output := formatSearchResults(query, results)

	writer := os.Stdout
	if flagOutputFile != "" {
		f, err := os.Create(flagOutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	if flagJSONOutput {
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]interface{}{
			"query":   query,
			"engine":  engine,
			"results": results,
		})
	} else {
		fmt.Fprint(writer, output)
	}

	return nil
}
```

Note: the `os`, `json`, and `fmt` imports are needed — the implementer should add them. Also `regexp` import may not be needed — remove if unused.

**Step 4: Wire up the search subcommand in `main.go`**

Replace the placeholder `searchCmd` RunE with:

```go
var searchEngine string
var searchMaxResults int

searchCmd := &cobra.Command{
    Use:   "search <query>",
    Short: "Search the web and return results",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runSearch(args[0], searchEngine, searchMaxResults)
    },
}
searchCmd.Flags().StringVarP(&searchEngine, "engine", "e", "google", "search engine: google, bing, duckduckgo, brave")
searchCmd.Flags().IntVarP(&searchMaxResults, "results", "n", 10, "max results to return")
rootCmd.AddCommand(searchCmd)
```

**Step 5: Run tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add search.go search_test.go main.go
git commit -m "feat: add search subcommand with Google parser"
```

---

### Task 5: Add Bing, DuckDuckGo, and Brave search parsers

Add the remaining three search engines to the engines map.

**Files:**
- Modify: `search.go` (add engines)
- Modify: `search_test.go` (add tests)

**Step 1: Write failing tests**

Add to `search_test.go`:

```go
func TestParseBingResults(t *testing.T) {
	html := `<html><body>
<li class="b_algo"><h2><a href="https://example.com/bing1">Bing First</a></h2>
<div class="b_caption"><p>Bing first snippet</p></div></li>
<li class="b_algo"><h2><a href="https://example.com/bing2">Bing Second</a></h2>
<div class="b_caption"><p>Bing second snippet</p></div></li>
</body></html>`

	results := parseBingResults([]byte(html))
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Title != "Bing First" {
		t.Fatalf("unexpected title: %s", results[0].Title)
	}
}

func TestParseDuckDuckGoResults(t *testing.T) {
	html := `<html><body>
<div class="result"><h2 class="result__title"><a class="result__a" href="https://example.com/ddg1">DDG First</a></h2>
<a class="result__snippet">DDG first snippet</a></div>
<div class="result"><h2 class="result__title"><a class="result__a" href="https://example.com/ddg2">DDG Second</a></h2>
<a class="result__snippet">DDG second snippet</a></div>
</body></html>`

	results := parseDuckDuckGoResults([]byte(html))
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].URL != "https://example.com/ddg1" {
		t.Fatalf("unexpected URL: %s", results[0].URL)
	}
}

func TestParseBraveResults(t *testing.T) {
	html := `<html><body>
<div class="snippet" data-type="web"><div class="snippet-title"><a href="https://example.com/brave1">Brave First</a></div>
<div class="snippet-description">Brave first snippet</div></div>
<div class="snippet" data-type="web"><div class="snippet-title"><a href="https://example.com/brave2">Brave Second</a></div>
<div class="snippet-description">Brave second snippet</div></div>
</body></html>`

	results := parseBraveResults([]byte(html))
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Title != "Brave First" {
		t.Fatalf("unexpected title: %s", results[0].Title)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test -run "TestParseBing|TestParseDuck|TestParseBrave" -v`
Expected: FAIL — functions undefined

**Step 3: Add parsers to `search.go`**

Add to the `engines` map:

```go
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
```

Implement the parser functions following the same DOM-walking pattern as `parseGoogleResults`:

- `parseBingResults`: Walk for `<li class="b_algo">`, extract `<a href>` + `<h2>` text + `<div class="b_caption">` text
- `parseDuckDuckGoResults`: Walk for `<div class="result">`, extract `<a class="result__a" href>` + `<a class="result__snippet">` text
- `parseBraveResults`: Walk for `<div class="snippet">`, extract `<a href>` from `.snippet-title` + `.snippet-description` text

**Step 4: Run all tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add search.go search_test.go
git commit -m "feat: add Bing, DuckDuckGo, and Brave search parsers"
```

---

### Task 6: Links extraction subcommand

Implement `brwoser links <url>` to extract all links from a page.

**Files:**
- Create: `links.go`
- Create: `links_test.go`
- Modify: `main.go` (wire up links subcommand)

**Step 1: Write the failing test**

Create `links_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	t.Run("extracts absolute links", func(t *testing.T) {
		html := `<html><body>
<a href="https://example.com/page1">Page 1</a>
<a href="https://example.com/page2">Page 2</a>
</body></html>`
		links := extractLinks([]byte(html), "https://example.com")
		if len(links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(links))
		}
		if links[0].URL != "https://example.com/page1" {
			t.Fatalf("unexpected URL: %s", links[0].URL)
		}
		if links[0].Text != "Page 1" {
			t.Fatalf("unexpected text: %s", links[0].Text)
		}
	})

	t.Run("resolves relative links", func(t *testing.T) {
		html := `<html><body><a href="/about">About</a></body></html>`
		links := extractLinks([]byte(html), "https://example.com")
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(links))
		}
		if links[0].URL != "https://example.com/about" {
			t.Fatalf("expected resolved URL, got: %s", links[0].URL)
		}
	})

	t.Run("skips fragment-only and javascript links", func(t *testing.T) {
		html := `<html><body>
<a href="#section">Anchor</a>
<a href="javascript:void(0)">JS</a>
<a href="https://example.com/real">Real</a>
</body></html>`
		links := extractLinks([]byte(html), "https://example.com")
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(links))
		}
	})
}

func TestFormatLinks(t *testing.T) {
	links := []pageLink{
		{URL: "https://example.com/1", Text: "First"},
		{URL: "https://example.com/2", Text: "Second"},
	}
	output := formatLinks(links)
	if !strings.Contains(output, "[First](https://example.com/1)") {
		t.Fatalf("missing first link in output: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test -run TestExtractLinks -v`
Expected: FAIL — `extractLinks` undefined

**Step 3: Create `links.go`**

```go
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

type pageLink struct {
	URL  string `json:"url"`
	Text string `json:"text"`
}

// extractLinks parses HTML and returns all <a href> links.
// Relative URLs are resolved against baseURL.
// Fragment-only (#) and javascript: links are skipped.
func extractLinks(body []byte, baseURL string) []pageLink {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	base, _ := url.Parse(baseURL)

	var links []pageLink
	seen := make(map[string]bool)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := strings.TrimSpace(attr.Val)
					// Skip fragments and javascript
					if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
						continue
					}
					// Resolve relative URL
					resolved := href
					if u, err := url.Parse(href); err == nil && base != nil {
						resolved = base.ResolveReference(u).String()
					}
					// Deduplicate
					if !seen[resolved] {
						seen[resolved] = true
						links = append(links, pageLink{
							URL:  resolved,
							Text: strings.TrimSpace(textContent(n)),
						})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

// formatLinks outputs links as a markdown list.
func formatLinks(links []pageLink) string {
	var sb strings.Builder
	for _, l := range links {
		text := l.Text
		if text == "" {
			text = l.URL
		}
		fmt.Fprintf(&sb, "- [%s](%s)\n", text, l.URL)
	}
	return sb.String()
}

// runLinks fetches a page and extracts all links.
func runLinks(rawURL string, filterPattern string) error {
	result, err := fetchOne(fetchOptions{
		url:           rawURL,
		browser:       flagBrowser,
		headers:       flagHeaders,
		timeout:       flagTimeout,
		noCookies:     flagNoCookies,
		cookieJarPath: flagCookieJarPath,
		verbose:       flagVerbose,
		method:        "GET",
	})
	if err != nil {
		return err
	}

	links := extractLinks(result.Body, result.URL)

	// Filter if pattern specified
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

	writer := os.Stdout
	if flagOutputFile != "" {
		f, err := os.Create(flagOutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	if flagJSONOutput {
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		enc.Encode(links)
	} else {
		fmt.Fprint(writer, formatLinks(links))
	}

	return nil
}
```

Note: `textContent()` is already defined in `search.go`. The implementer should NOT redefine it in `links.go`. It's shared.

**Step 4: Wire up links subcommand in `main.go`**

Replace the placeholder `linksCmd`:

```go
var linksFilter string

linksCmd := &cobra.Command{
    Use:   "links <url>",
    Short: "Extract all links from a page",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runLinks(args[0], linksFilter)
    },
}
linksCmd.Flags().StringVarP(&linksFilter, "filter", "f", "", "filter links by regex pattern")
rootCmd.AddCommand(linksCmd)
```

**Step 5: Run all tests**

Run: `export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add links.go links_test.go main.go
git commit -m "feat: add links subcommand for link extraction"
```

---

### Task 7: Build, test end-to-end, fix issues

Build the binary and test all three subcommands against real sites.

**Files:**
- No new files (testing only)

**Step 1: Build**

```bash
export PATH="/home/x/go/bin:$PATH"
cd /home/x/projects/brwoser
go build -o brwoser .
```

**Step 2: Test single fetch (backward compatibility)**

```bash
./brwoser -m https://httpbin.org/get
```
Expected: Markdown output of httpbin response

**Step 3: Test fetch subcommand**

```bash
./brwoser fetch -m https://httpbin.org/get
```
Expected: Same as above

**Step 4: Test parallel fetch**

```bash
./brwoser fetch -m https://httpbin.org/get https://httpbin.org/ip
```
Expected: Two results separated by `---` headers

**Step 5: Test search**

```bash
./brwoser search "golang concurrency patterns"
```
Expected: Numbered markdown list of Google results

**Step 6: Test search with different engine**

```bash
./brwoser search -e duckduckgo "golang concurrency patterns"
```
Expected: DuckDuckGo results

**Step 7: Test links**

```bash
./brwoser links https://httpbin.org
```
Expected: Markdown list of links found on the page

**Step 8: Test links with filter**

```bash
./brwoser links -f "github" https://httpbin.org
```
Expected: Only links matching "github"

**Step 9: Fix any issues found during testing**

If any subcommand fails, debug and fix. Common issues:
- Search engine HTML structure changed (update selectors)
- Import conflicts (remove duplicate `textContent`)
- Flag conflicts between root and subcommands

**Step 10: Run full test suite**

```bash
export PATH="/home/x/go/bin:$PATH" && cd /home/x/projects/brwoser && go test ./... -v -count=1
```
Expected: All PASS

**Step 11: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve issues found during e2e testing of subcommands"
```
