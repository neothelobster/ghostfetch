package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestE2EFetchHTTPBin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("fetch httpbin with chrome profile shows chrome user-agent", func(t *testing.T) {
		profile := getProfile("chrome")
		tr, err := newTransport(profile)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		resp, body, err := doFetch(ctx, tr, profile, "GET", "https://httpbin.org/user-agent", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "Chrome") {
			t.Fatalf("expected Chrome in user-agent response, got: %s", body)
		}
	})

	t.Run("fetch httpbin with firefox profile shows firefox user-agent", func(t *testing.T) {
		profile := getProfile("firefox")
		tr, err := newTransport(profile)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		resp, body, err := doFetch(ctx, tr, profile, "GET", "https://httpbin.org/user-agent", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "Firefox") {
			t.Fatalf("expected Firefox in user-agent response, got: %s", body)
		}
	})

	t.Run("custom headers are sent", func(t *testing.T) {
		profile := getProfile("chrome")
		tr, err := newTransport(profile)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		extra := [][2]string{{"X-Custom-Test", "web-search-test-value"}}
		resp, body, err := doFetch(ctx, tr, profile, "GET", "https://httpbin.org/headers", extra, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "web-search-test-value") {
			t.Fatalf("expected custom header in response, got: %s", body)
		}
	})
}
