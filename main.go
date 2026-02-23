package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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
	// 1. Parse the URL (prepend "https://" if no scheme).
	targetURL := rawURL
	if !strings.Contains(targetURL, "://") {
		targetURL = "https://" + targetURL
	}

	// 2. Parse the timeout duration.
	dur, err := time.ParseDuration(opts.timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", opts.timeout, err)
	}

	// 3. Create context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	// 4. Get browser profile.
	profile := getProfile(opts.browser)
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Using %s profile\n", profile.Name)
	}

	// 5. Create transport.
	tr, err := newTransport(profile)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// 6. Load cookie jar if cookies are enabled.
	var jar *PersistentJar
	if !opts.noCookies {
		jarPath := opts.cookieJarPath
		if jarPath == "" {
			jarPath = defaultCookieJarPath()
		}
		jar = newPersistentJar(jarPath)
		if err := jar.Load(); err != nil {
			return fmt.Errorf("failed to load cookie jar: %w", err)
		}
	}

	// 7. Parse custom headers.
	extraHeaders := parseHeaders(opts.headers)

	// 8. Build initial cookies from jar.
	var cookies []*http.Cookie
	if jar != nil {
		if u, err := url.Parse(targetURL); err == nil {
			cookies = jar.Cookies(u)
		}
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Fetching %s\n", targetURL)
	}

	// 9. Perform the fetch.
	var resp *http.Response
	var body []byte
	if opts.data != "" {
		resp, body, err = doFetchWithBody(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies, opts.data)
	} else {
		resp, body, err = doFetch(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies)
	}
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// 10. Detect challenges.
	challenge := detectChallenge(resp, body)
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Challenge: %s\n", challenge)
	}

	// 11. Handle JS challenge.
	if challenge == ChallengeJS {
		script := extractScriptContent(body)
		if script != "" {
			solver := newJSSolver(targetURL)
			result, err := solver.Solve(script)
			if err != nil {
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] JS solver error: %v\n", err)
				}
			} else if result.CookieName != "" {
				// Add the solved cookie and retry.
				solvedCookie := &http.Cookie{
					Name:  result.CookieName,
					Value: result.CookieValue,
				}
				cookies = append(cookies, solvedCookie)

				// Store solved cookie in jar.
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
					return fmt.Errorf("retry fetch failed: %w", err)
				}
			}
		}
	}

	// 12. Handle captcha challenge.
	if challenge == ChallengeCaptcha {
		sitekey, captchaType := extractSitekey(body)
		if sitekey != "" {
			// Resolve captcha service and key from flags or environment.
			svc := opts.captchaService
			if svc == "" {
				svc = os.Getenv("BRWOSER_CAPTCHA_SERVICE")
			}
			key := opts.captchaKey
			if key == "" {
				key = os.Getenv("BRWOSER_CAPTCHA_KEY")
			}

			if svc == "" || key == "" {
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Captcha detected but no service/key configured\n")
				}
			} else {
				captchaSolver, err := newCaptchaSolver(svc, key)
				if err != nil {
					return fmt.Errorf("captcha solver init failed: %w", err)
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Solving %s captcha via %s\n", captchaType, svc)
				}
				token, err := captchaSolver.Solve(ctx, sitekey, targetURL, captchaType)
				if err != nil {
					return fmt.Errorf("captcha solve failed: %w", err)
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Captcha solved, retrying fetch\n")
				}
				// Add captcha token as cookie and retry.
				solvedCookie := &http.Cookie{
					Name:  "cf_clearance",
					Value: token,
				}
				cookies = append(cookies, solvedCookie)

				if jar != nil {
					if u, err := url.Parse(targetURL); err == nil {
						jar.SetCookies(u, []*http.Cookie{solvedCookie})
					}
				}

				resp, body, err = doFetch(ctx, tr, profile, opts.method, targetURL, extraHeaders, cookies)
				if err != nil {
					return fmt.Errorf("retry fetch after captcha failed: %w", err)
				}
			}
		}
	}

	// 13. Save cookies if jar is set.
	if jar != nil {
		// Store response cookies in the jar.
		if resp != nil && resp.Request != nil && resp.Request.URL != nil {
			if respCookies := resp.Cookies(); len(respCookies) > 0 {
				jar.SetCookies(resp.Request.URL, respCookies)
			}
		}
		if err := jar.Save(); err != nil {
			if opts.verbose {
				fmt.Fprintf(os.Stderr, "[*] Warning: failed to save cookies: %v\n", err)
			}
		}
	}

	// 14. Write output.
	writer := os.Stdout
	if opts.outputFile != "" {
		f, err := os.Create(opts.outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		writer = f
	}
	formatOutput(writer, resp, body, outputOptions{
		asJSON:       opts.jsonOutput,
		markdown:     opts.markdown,
		markdownFull: opts.markdownFull,
		pageURL:      targetURL,
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
