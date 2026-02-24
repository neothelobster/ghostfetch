package main

import (
	"testing"
)

func TestFetchOneInvalidTimeout(t *testing.T) {
	t.Run("invalid timeout returns error", func(t *testing.T) {
		_, err := fetchOne(fetchOptions{
			url:       "https://example.com",
			timeout:   "not-a-duration",
			noCookies: true,
		})
		if err == nil {
			t.Fatal("expected error for invalid timeout, got nil")
		}
		if got := err.Error(); got == "" {
			t.Fatal("expected non-empty error message")
		}
	})
}

func TestFetchOptionsDefaults(t *testing.T) {
	t.Run("zero-value fetchOptions has expected defaults", func(t *testing.T) {
		opts := fetchOptions{}
		if opts.url != "" {
			t.Fatalf("expected empty url, got %q", opts.url)
		}
		if opts.browser != "" {
			t.Fatalf("expected empty browser (defaults applied in fetchOne), got %q", opts.browser)
		}
		if opts.timeout != "" {
			t.Fatalf("expected empty timeout (defaults applied in fetchOne), got %q", opts.timeout)
		}
		if opts.method != "" {
			t.Fatalf("expected empty method (defaults applied in fetchOne), got %q", opts.method)
		}
		if opts.noCookies {
			t.Fatal("expected noCookies to be false by default")
		}
		if opts.verbose {
			t.Fatal("expected verbose to be false by default")
		}
		if opts.data != "" {
			t.Fatalf("expected empty data, got %q", opts.data)
		}
		if opts.captchaService != "" {
			t.Fatalf("expected empty captchaService, got %q", opts.captchaService)
		}
		if opts.captchaKey != "" {
			t.Fatalf("expected empty captchaKey, got %q", opts.captchaKey)
		}
		if len(opts.headers) != 0 {
			t.Fatalf("expected nil headers, got %v", opts.headers)
		}
		if opts.cookieJarPath != "" {
			t.Fatalf("expected empty cookieJarPath, got %q", opts.cookieJarPath)
		}
	})
}

func TestFetchResultFields(t *testing.T) {
	t.Run("fetchResult can hold all expected fields", func(t *testing.T) {
		r := &fetchResult{
			URL:        "https://example.com",
			StatusCode: 200,
			Body:       []byte("hello"),
		}
		if r.URL != "https://example.com" {
			t.Fatalf("unexpected URL: %s", r.URL)
		}
		if r.StatusCode != 200 {
			t.Fatalf("unexpected StatusCode: %d", r.StatusCode)
		}
		if string(r.Body) != "hello" {
			t.Fatalf("unexpected Body: %s", r.Body)
		}
	})
}
