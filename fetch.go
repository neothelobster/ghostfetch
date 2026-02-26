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

// fetchOptions holds the parameters for a single fetch operation.
// This is a read-only tool: only GET requests, no custom headers,
// no file writes, no request body — safe for LLM agent use.
type fetchOptions struct {
	url       string
	browser   string
	timeout   string
	noCookies bool
	verbose   bool
}

// fetchResult holds the outcome of a fetch operation.
type fetchResult struct {
	URL        string
	StatusCode int
	Headers    http.Header
	Body       []byte
	// Error is set by parallel fetch callers, not by fetchOne().
	// fetchOne returns errors via its second return value.
	Error error
	// resp is the original *http.Response, retained so callers like run()
	// can pass it to formatOutput without reconstructing one.
	resp *http.Response
}

// fetchOne executes the full fetch pipeline: URL parsing, timeout, transport
// creation, cookie jar loading, initial fetch, challenge detection/solving,
// captcha handling, and cookie saving. It returns a fetchResult or an error.
func fetchOne(opts fetchOptions) (*fetchResult, error) {
	// 1. Parse the URL (prepend "https://" if no scheme).
	targetURL := opts.url
	if !strings.Contains(targetURL, "://") {
		targetURL = "https://" + targetURL
	}

	// 2. Parse the timeout duration.
	timeout := opts.timeout
	if timeout == "" {
		timeout = "30s"
	}
	dur, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout %q: %w", timeout, err)
	}

	// 3. Create context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	// 4. Get browser profile.
	browser := opts.browser
	if browser == "" {
		browser = "chrome"
	}
	profile := getProfile(browser)
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Using %s profile\n", profile.Name)
	}

	// 5. Create transport.
	tr, err := newTransport(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// 6. Load cookie jar if cookies are enabled.
	var jar *PersistentJar
	if !opts.noCookies {
		jarPath := defaultCookieJarPath()
		jar = newPersistentJar(jarPath)
		if err := jar.Load(); err != nil {
			return nil, fmt.Errorf("failed to load cookie jar: %w", err)
		}
	}

	// 7. Build initial cookies from jar.
	var cookies []*http.Cookie
	if jar != nil {
		if u, err := url.Parse(targetURL); err == nil {
			cookies = jar.Cookies(u)
		}
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "[*] Fetching %s\n", targetURL)
	}

	// 8. Perform the fetch (read-only GET request, no custom headers).
	resp, body, err := doFetch(ctx, tr, profile, "GET", targetURL, nil, cookies)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
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
				resp, body, err = doFetch(ctx, tr, profile, "GET", targetURL, nil, cookies)
				if err != nil {
					return nil, fmt.Errorf("retry fetch failed: %w", err)
				}
			}
		}
	}

	// 12. Handle captcha challenge (env-based config only, no CLI flags).
	if challenge == ChallengeCaptcha {
		sitekey, captchaType := extractSitekey(body)
		if sitekey != "" {
			svc := os.Getenv("GHOSTFETCH_CAPTCHA_SERVICE")
			key := os.Getenv("GHOSTFETCH_CAPTCHA_KEY")

			if svc == "" || key == "" {
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Captcha detected but no service/key configured\n")
				}
			} else {
				captchaSolver, err := newCaptchaSolver(svc, key)
				if err != nil {
					return nil, fmt.Errorf("captcha solver init failed: %w", err)
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Solving %s captcha via %s\n", captchaType, svc)
				}
				token, err := captchaSolver.Solve(ctx, sitekey, targetURL, captchaType)
				if err != nil {
					return nil, fmt.Errorf("captcha solve failed: %w", err)
				}
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "[*] Captcha solved, retrying fetch\n")
				}
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

				resp, body, err = doFetch(ctx, tr, profile, "GET", targetURL, nil, cookies)
				if err != nil {
					return nil, fmt.Errorf("retry fetch after captcha failed: %w", err)
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

	return &fetchResult{
		URL:        targetURL,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
		resp:       resp,
	}, nil
}
