# brwoser - Browser-Like CLI Fetcher

## Overview

A Go CLI tool that fetches web pages like curl but bypasses bot detection. Single binary, no browser dependency.

## Architecture

```
brwoser [flags] <url>
    |
    v
CLI (cobra) -> TLS Impersonator (uTLS) -> Request Engine -> Challenge Detector
                                                                |
                                              JS Solver (goja) <-+-> Captcha Service (2captcha/anticaptcha)
                                                                |
                                                          Cookie Jar -> Retry with token -> stdout
```

## Components

| Component | Technology | Purpose |
|-----------|-----------|---------|
| CLI framework | cobra | Flag parsing, help text |
| TLS impersonation | uTLS | Mimic Chrome/Firefox TLS fingerprint (JA3/JA4) |
| HTTP/2 framing | Custom | Match browser SETTINGS, WINDOW_UPDATE, header order |
| JS challenge solver | goja | Evaluate Cloudflare-style JS challenges (pure Go, no CGo) |
| Captcha solver | 2captcha/anticaptcha API | Solve Turnstile/hCaptcha/reCAPTCHA |
| Cookie persistence | JSON file (~/.brwoser/cookies.json) | Skip re-solving challenges on repeat visits |
| Output | Raw HTML or JSON (--json flag) | Composable with other CLI tools |

## CLI Interface

```
brwoser [flags] <url>

Flags:
  -o, --output <file>         Write response to file instead of stdout
  -H, --header <k:v>          Add custom header (repeatable)
  -b, --browser <name>        Browser to impersonate: chrome, firefox (default: chrome)
  -j, --json                  Output JSON with body, status, headers, cookies
  -L, --follow                Follow redirects (default: true, up to 10)
  -c, --cookie-jar <file>     Cookie jar file path (default: ~/.brwoser/cookies.json)
      --no-cookies            Don't load/save cookies
  -t, --timeout <dur>         Request timeout (default: 30s)
  -v, --verbose               Print request/response details to stderr
  -X, --method <method>       HTTP method (default: GET)
  -d, --data <data>           Request body (for POST/PUT)
      --captcha-service <svc> Captcha service: 2captcha, anticaptcha
      --captcha-key <key>     Captcha service API key (or BRWOSER_CAPTCHA_KEY env var)
```

## TLS Impersonation

- uTLS with Chrome 120+ and Firefox 121+ profiles
- Matches cipher suites, extensions, ALPN, key share groups, signature algorithms
- HTTP/2: correct SETTINGS frame values, WINDOW_UPDATE, pseudo-header order
- Full browser header set in correct order (User-Agent, Accept, Sec-Ch-Ua, Sec-Fetch-*, etc.)

## Challenge Detection & Solving

1. Detect: HTTP 403/503 + Cloudflare server header + known challenge patterns in body
2. JS challenges: Execute in goja with minimal DOM stubs (document.createElement, window.location, navigator, atob/btoa, setTimeout)
3. Captcha challenges: Extract sitekey, submit to captcha service, poll for solution, submit token
4. Re-request with solved cookie/token

## Cookie Persistence

- Default jar: ~/.brwoser/cookies.json
- Stores clearance cookies per domain
- Skips challenge solving on repeat fetches until cookie expires

## Scope

**Handles:** TLS fingerprint checks, HTTP/2 fingerprint checks, JS challenges, managed Cloudflare challenges, Turnstile/hCaptcha (via captcha service).

**Does not handle:** Full SPA rendering, canvas/WebGL fingerprinting, interactive challenges without a captcha service.

## Config

Optional config file at `~/.brwoser/config.json`:
```json
{
  "default_browser": "chrome",
  "captcha_service": "2captcha",
  "captcha_key": "...",
  "cookie_jar": "~/.brwoser/cookies.json",
  "timeout": "30s"
}
```

Environment variables: `BRWOSER_CAPTCHA_KEY`, `BRWOSER_CAPTCHA_SERVICE`.
