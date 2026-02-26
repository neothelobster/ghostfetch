package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Package-level flag variables shared across subcommands.
var (
	flagBrowser      string
	flagJSONOutput   bool
	flagFollowRedirs bool
	flagNoCookies    bool
	flagTimeout      string
	flagVerbose      bool
	flagMarkdown     bool
	flagMarkdownFull bool
	flagRaw          bool
	flagMaxParallel  int
	searchEngineName string
	searchMaxResults int
	linksFilter      string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ghostfetch [flags] <query>",
		Short: "Search the web and fetch pages with bot detection bypass",
		Long: `ghostfetch searches and fetches the web like a ghost — browser-like
TLS fingerprints, invisible to bot detection, no full browser needed.

By default, running ghostfetch with a query performs a web search.
Use subcommands (fetch, links) for other operations.`,
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// If argument looks like a URL, fetch it.
			if looksLikeURL(args[0]) {
				return runFetch(args)
			}
			// Otherwise, treat it as a search query.
			query := strings.Join(args, " ")
			return runSearch(query, searchEngineName, searchMaxResults)
		},
	}

	// Persistent flags — shared across all subcommands.
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagBrowser, "browser", "b", "chrome", "browser to impersonate: chrome, firefox")
	pf.BoolVarP(&flagJSONOutput, "json", "j", false, "output JSON with body, status, headers, cookies")
	pf.BoolVarP(&flagFollowRedirs, "follow", "L", true, "follow redirects (up to 10)")
	pf.BoolVar(&flagNoCookies, "no-cookies", false, "don't load/save cookies")
	pf.StringVarP(&flagTimeout, "timeout", "t", "30s", "request timeout")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "print request/response details to stderr")
	pf.BoolVarP(&flagMarkdown, "markdown", "m", false, "convert to markdown (reader mode: extracts main content)")
	pf.BoolVar(&flagMarkdownFull, "markdown-full", false, "convert full page HTML to markdown")
	pf.BoolVar(&flagRaw, "raw", false, "output raw HTML without any processing")

	// Search flags on root command (so `web_search -e brave "query"` works).
	rootCmd.Flags().StringVarP(&searchEngineName, "engine", "e", "duckduckgo", "search engine: duckduckgo, bing, brave, google")
	rootCmd.Flags().IntVarP(&searchMaxResults, "results", "n", 10, "number of results")

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
	cmd.Flags().StringVarP(&searchEngineName, "engine", "e", "duckduckgo", "search engine: duckduckgo, bing, brave, google")
	cmd.Flags().IntVarP(&searchMaxResults, "results", "n", 10, "number of results")
	return cmd
}

// newLinksCmd creates the "links" subcommand.
func newLinksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links <url>",
		Short: "Extract links from a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinks(args[0], linksFilter)
		},
	}
	cmd.Flags().StringVarP(&linksFilter, "filter", "f", "", "filter links by regex pattern")
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

// runSingleFetch fetches a single URL and writes the formatted output to stdout.
func runSingleFetch(rawURL string) error {
	result, err := fetchOne(fetchOptions{
		url:       rawURL,
		browser:   flagBrowser,
		timeout:   flagTimeout,
		noCookies: flagNoCookies,
		verbose:   flagVerbose,
	})
	if err != nil {
		return err
	}

	formatOutput(os.Stdout, result.resp, result.Body, outputOptions{
		asJSON:       flagJSONOutput,
		markdown:     flagMarkdown,
		markdownFull: flagMarkdownFull,
		pageURL:      result.URL,
	})

	return nil
}

// defaultCookieJarPath returns the default path for the persistent cookie jar:
// ~/.ghostfetch/cookies.json
func defaultCookieJarPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ghostfetch", "cookies.json")
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
