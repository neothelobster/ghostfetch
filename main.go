package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	var (
		outputFile     string
		headers        []string
		browser        string
		jsonOutput     bool
		followRedirs   bool
		cookieJarPath  string
		noCookies      bool
		timeout        string
		verbose        bool
		method         string
		data           string
		captchaService string
		captchaKey     string
		markdown       bool
		markdownFull   bool
	)

	rootCmd := &cobra.Command{
		Use:   "brwoser [flags] <url>",
		Short: "Fetch web pages like curl, but bypass bot detection",
		Long: `brwoser fetches web pages with browser-like TLS fingerprints,
solves JavaScript challenges, and handles captchas via external services.
It bypasses bot detection without running a full browser.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args[0], runOptions{
				outputFile:     outputFile,
				headers:        headers,
				browser:        browser,
				jsonOutput:     jsonOutput,
				followRedirs:   followRedirs,
				cookieJarPath:  cookieJarPath,
				noCookies:      noCookies,
				timeout:        timeout,
				verbose:        verbose,
				method:         method,
				data:           data,
				captchaService: captchaService,
				captchaKey:     captchaKey,
				markdown:       markdown,
				markdownFull:   markdownFull,
			})
		},
	}

	f := rootCmd.Flags()
	f.StringVarP(&outputFile, "output", "o", "", "write response to file")
	f.StringArrayVarP(&headers, "header", "H", nil, "add custom header (repeatable)")
	f.StringVarP(&browser, "browser", "b", "chrome", "browser to impersonate: chrome, firefox")
	f.BoolVarP(&jsonOutput, "json", "j", false, "output JSON with body, status, headers, cookies")
	f.BoolVarP(&followRedirs, "follow", "L", true, "follow redirects (up to 10)")
	f.StringVarP(&cookieJarPath, "cookie-jar", "c", "", "cookie jar file path (default: ~/.brwoser/cookies.json)")
	f.BoolVar(&noCookies, "no-cookies", false, "don't load/save cookies")
	f.StringVarP(&timeout, "timeout", "t", "30s", "request timeout")
	f.BoolVarP(&verbose, "verbose", "v", false, "print request/response details to stderr")
	f.StringVarP(&method, "method", "X", "GET", "HTTP method")
	f.StringVarP(&data, "data", "d", "", "request body")
	f.StringVar(&captchaService, "captcha-service", "", "captcha service: 2captcha, anticaptcha")
	f.StringVar(&captchaKey, "captcha-key", "", "captcha service API key")
	f.BoolVarP(&markdown, "markdown", "m", false, "convert to markdown (reader mode: extracts main content)")
	f.BoolVar(&markdownFull, "markdown-full", false, "convert full page HTML to markdown")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type runOptions struct {
	outputFile     string
	headers        []string
	browser        string
	jsonOutput     bool
	followRedirs   bool
	cookieJarPath  string
	noCookies      bool
	timeout        string
	verbose        bool
	method         string
	data           string
	captchaService string
	captchaKey     string
	markdown       bool
	markdownFull   bool
}

func run(rawURL string, opts runOptions) error {
	// Delegate the full fetch pipeline to fetchOne().
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

	// Write output.
	writer := os.Stdout
	if opts.outputFile != "" {
		f, err := os.Create(opts.outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}
	formatOutput(writer, result.resp, result.Body, outputOptions{
		asJSON:       opts.jsonOutput,
		markdown:     opts.markdown,
		markdownFull: opts.markdownFull,
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
