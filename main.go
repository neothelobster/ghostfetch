package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Package-level flag variables shared across subcommands.
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
	flagRaw              bool
	flagMaxParallel      int
	searchEngineName     string
	searchMaxResults     int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "brwoser [flags] [command]",
		Short: "Fetch web pages like curl, but bypass bot detection",
		Long: `brwoser fetches web pages with browser-like TLS fingerprints,
solves JavaScript challenges, and handles captchas via external services.
It bypasses bot detection without running a full browser.`,
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare `brwoser <url>` is a shortcut for `brwoser fetch <url>`.
			if len(args) > 0 && looksLikeURL(args[0]) {
				return runFetch(args)
			}
			return cmd.Help()
		},
	}

	// Persistent flags — shared across all subcommands.
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagOutputFile, "output", "o", "", "write response to file")
	pf.StringArrayVarP(&flagHeaders, "header", "H", nil, "add custom header (repeatable)")
	pf.StringVarP(&flagBrowser, "browser", "b", "chrome", "browser to impersonate: chrome, firefox")
	pf.BoolVarP(&flagJSONOutput, "json", "j", false, "output JSON with body, status, headers, cookies")
	pf.BoolVarP(&flagFollowRedirs, "follow", "L", true, "follow redirects (up to 10)")
	pf.StringVarP(&flagCookieJarPath, "cookie-jar", "c", "", "cookie jar file path (default: ~/.brwoser/cookies.json)")
	pf.BoolVar(&flagNoCookies, "no-cookies", false, "don't load/save cookies")
	pf.StringVarP(&flagTimeout, "timeout", "t", "30s", "request timeout")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "print request/response details to stderr")
	pf.StringVarP(&flagMethod, "method", "X", "GET", "HTTP method")
	pf.StringVarP(&flagData, "data", "d", "", "request body")
	pf.StringVar(&flagCaptchaService, "captcha-service", "", "captcha service: 2captcha, anticaptcha")
	pf.StringVar(&flagCaptchaKey, "captcha-key", "", "captcha service API key")
	pf.BoolVarP(&flagMarkdown, "markdown", "m", false, "convert to markdown (reader mode: extracts main content)")
	pf.BoolVar(&flagMarkdownFull, "markdown-full", false, "convert full page HTML to markdown")
	pf.BoolVar(&flagRaw, "raw", false, "output raw HTML without any processing")

	// Subcommands.
	rootCmd.AddCommand(newFetchCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newLinksCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// looksLikeURL returns true if the argument looks like a URL rather than
// a subcommand name. It checks for "://" or the presence of a dot.
func looksLikeURL(s string) bool {
	return strings.Contains(s, "://") || strings.Contains(s, ".")
}

// newFetchCmd creates the "fetch" subcommand.
func newFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch <url> [url2] [url3...]",
		Short: "Fetch one or more URLs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(args)
		},
	}
	cmd.Flags().IntVarP(&flagMaxParallel, "max-parallel", "p", 5, "max parallel fetches")
	return cmd
}

// newSearchCmd creates the "search" subcommand.
func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the web",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(args[0], searchEngineName, searchMaxResults)
		},
	}
	cmd.Flags().StringVarP(&searchEngineName, "engine", "e", "google", "search engine to use")
	cmd.Flags().IntVarP(&searchMaxResults, "results", "n", 10, "number of results")
	return cmd
}

// newLinksCmd creates the "links" subcommand (placeholder).
func newLinksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links <url>",
		Short: "Extract links from a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("links not yet implemented")
		},
	}
	cmd.Flags().StringP("filter", "f", "", "filter links by pattern")
	return cmd
}

// runFetch dispatches to runSingleFetch for a single URL or
// runParallelFetch for multiple URLs.
func runFetch(urls []string) error {
	if len(urls) == 1 {
		return runSingleFetch(urls[0])
	}
	return runParallelFetch(urls)
}

// runSingleFetch fetches a single URL and writes the formatted output.
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

	// Write output.
	writer := os.Stdout
	if flagOutputFile != "" {
		f, err := os.Create(flagOutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}
	formatOutput(writer, result.resp, result.Body, outputOptions{
		asJSON:       flagJSONOutput,
		markdown:     flagMarkdown,
		markdownFull: flagMarkdownFull,
		pageURL:      result.URL,
	})

	return nil
}

// parseHeaders splits raw header strings on the first ":" into key-value pairs.
// Entries without a colon are skipped.
func parseHeaders(raw []string) [][2]string {
	var headers [][2]string
	for _, h := range raw {
		idx := strings.Index(h, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(h[:idx])
		value := strings.TrimSpace(h[idx+1:])
		headers = append(headers, [2]string{key, value})
	}
	return headers
}

// defaultCookieJarPath returns the default path for the persistent cookie jar:
// ~/.brwoser/cookies.json
func defaultCookieJarPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".brwoser", "cookies.json")
}

// scriptTagRe matches <script ...>...</script> blocks, capturing the tag
// attributes and the content between tags.
var scriptTagRe = regexp.MustCompile(`(?is)<script[^>]*>(.*?)</script>`)

// scriptSrcRe matches script tags that have a src attribute (external scripts).
var scriptSrcRe = regexp.MustCompile(`(?i)src\s*=`)

// extractScriptContent finds all inline <script>...</script> blocks in the
// HTML body and concatenates their content. External scripts (those with a
// src attribute) are skipped.
func extractScriptContent(body []byte) string {
	matches := scriptTagRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return ""
	}

	var scripts []string
	for _, m := range matches {
		// m[0] is the full match including tags, m[1] is the content.
		// Check if the opening tag has a src= attribute (external script).
		fullTag := string(m[0])
		openTagEnd := strings.Index(fullTag, ">")
		if openTagEnd > 0 {
			openTag := fullTag[:openTagEnd]
			if scriptSrcRe.MatchString(openTag) {
				continue
			}
		}

		content := strings.TrimSpace(string(m[1]))
		if content != "" {
			scripts = append(scripts, content)
		}
	}

	return strings.Join(scripts, "\n")
}
