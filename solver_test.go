package main

import (
	"testing"
)

func TestJSSolver(t *testing.T) {
	t.Run("solves simple arithmetic challenge", func(t *testing.T) {
		script := `
			var a = 10;
			var b = 5;
			var answer = a + b;
			document.cookie = "cf_clearance=" + answer + "; path=/";
		`
		solver := newJSSolver("https://example.com")
		result, err := solver.Solve(script)
		if err != nil {
			t.Fatalf("solve error: %v", err)
		}
		if result.CookieName != "cf_clearance" {
			t.Fatalf("expected cookie name 'cf_clearance', got %q", result.CookieName)
		}
		if result.CookieValue != "15" {
			t.Fatalf("expected cookie value '15', got %q", result.CookieValue)
		}
	})

	t.Run("atob and btoa work", func(t *testing.T) {
		script := `
			var encoded = btoa("hello");
			var decoded = atob(encoded);
			document.cookie = "test=" + decoded;
		`
		solver := newJSSolver("https://example.com")
		result, err := solver.Solve(script)
		if err != nil {
			t.Fatalf("solve error: %v", err)
		}
		if result.CookieValue != "hello" {
			t.Fatalf("expected 'hello', got %q", result.CookieValue)
		}
	})

	t.Run("timeout on infinite loop", func(t *testing.T) {
		script := `while(true){}`
		solver := newJSSolver("https://example.com")
		_, err := solver.Solve(script)
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})
}
