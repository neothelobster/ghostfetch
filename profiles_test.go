package main

import (
	"testing"
)

func TestGetProfile(t *testing.T) {
	t.Run("chrome profile has correct user agent prefix", func(t *testing.T) {
		p := getProfile("chrome")
		if p.Name != "chrome" {
			t.Fatalf("expected name 'chrome', got %q", p.Name)
		}
		if len(p.Headers) == 0 {
			t.Fatal("expected non-empty headers")
		}
		ua := ""
		for _, h := range p.Headers {
			if h[0] == "User-Agent" {
				ua = h[1]
			}
		}
		if ua == "" {
			t.Fatal("expected User-Agent header")
		}
	})

	t.Run("firefox profile exists", func(t *testing.T) {
		p := getProfile("firefox")
		if p.Name != "firefox" {
			t.Fatalf("expected name 'firefox', got %q", p.Name)
		}
	})

	t.Run("unknown profile falls back to chrome", func(t *testing.T) {
		p := getProfile("unknown")
		if p.Name != "chrome" {
			t.Fatalf("expected fallback to 'chrome', got %q", p.Name)
		}
	})
}
