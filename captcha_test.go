package main

import (
	"testing"
)

func TestExtractSitekey(t *testing.T) {
	t.Run("extract turnstile sitekey", func(t *testing.T) {
		body := []byte(`<div class="cf-turnstile" data-sitekey="0x4AAAAAAAB1234"></div>`)
		key, ct := extractSitekey(body)
		if key != "0x4AAAAAAAB1234" {
			t.Fatalf("expected sitekey '0x4AAAAAAAB1234', got %q", key)
		}
		if ct != "turnstile" {
			t.Fatalf("expected type 'turnstile', got %q", ct)
		}
	})

	t.Run("extract hcaptcha sitekey", func(t *testing.T) {
		body := []byte(`<div class="h-captcha" data-sitekey="abcdef-123456"></div>`)
		key, ct := extractSitekey(body)
		if key != "abcdef-123456" {
			t.Fatalf("expected sitekey 'abcdef-123456', got %q", key)
		}
		if ct != "hcaptcha" {
			t.Fatalf("expected type 'hcaptcha', got %q", ct)
		}
	})

	t.Run("no sitekey found", func(t *testing.T) {
		body := []byte(`<html><body>No captcha here</body></html>`)
		key, _ := extractSitekey(body)
		if key != "" {
			t.Fatalf("expected empty sitekey, got %q", key)
		}
	})
}

func TestCaptchaSolverNew(t *testing.T) {
	t.Run("creates 2captcha solver", func(t *testing.T) {
		s, err := newCaptchaSolver("2captcha", "fake-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil solver")
		}
	})

	t.Run("rejects unknown service", func(t *testing.T) {
		_, err := newCaptchaSolver("unknown", "key")
		if err == nil {
			t.Fatal("expected error for unknown service")
		}
	})
}
