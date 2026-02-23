package main

import (
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestCookieJar(t *testing.T) {
	t.Run("save and load cookies", func(t *testing.T) {
		tmpDir := t.TempDir()
		jarPath := filepath.Join(tmpDir, "cookies.json")

		jar := newPersistentJar(jarPath)

		u, _ := url.Parse("https://example.com")
		jar.SetCookies(u, []*http.Cookie{
			{Name: "cf_clearance", Value: "abc123", Domain: "example.com", Expires: time.Now().Add(time.Hour)},
		})

		if err := jar.Save(); err != nil {
			t.Fatalf("save error: %v", err)
		}

		jar2 := newPersistentJar(jarPath)
		if err := jar2.Load(); err != nil {
			t.Fatalf("load error: %v", err)
		}

		cookies := jar2.Cookies(u)
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}
		if cookies[0].Name != "cf_clearance" || cookies[0].Value != "abc123" {
			t.Fatalf("unexpected cookie: %+v", cookies[0])
		}
	})

	t.Run("expired cookies are not loaded", func(t *testing.T) {
		tmpDir := t.TempDir()
		jarPath := filepath.Join(tmpDir, "cookies.json")

		jar := newPersistentJar(jarPath)
		u, _ := url.Parse("https://example.com")
		jar.SetCookies(u, []*http.Cookie{
			{Name: "old", Value: "expired", Domain: "example.com", Expires: time.Now().Add(-time.Hour)},
		})
		jar.Save()

		jar2 := newPersistentJar(jarPath)
		jar2.Load()
		cookies := jar2.Cookies(u)
		if len(cookies) != 0 {
			t.Fatalf("expected 0 cookies (expired), got %d", len(cookies))
		}
	})

	t.Run("load from nonexistent file is not an error", func(t *testing.T) {
		jar := newPersistentJar("/nonexistent/path/cookies.json")
		if err := jar.Load(); err != nil {
			t.Fatalf("expected no error for missing file, got: %v", err)
		}
	})
}
