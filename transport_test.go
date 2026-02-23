package main

import (
	"context"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	t.Run("creates transport without error", func(t *testing.T) {
		profile := getProfile("chrome")
		tr, err := newTransport(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tr == nil {
			t.Fatal("expected non-nil transport")
		}
	})
}

func TestFetchBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	t.Run("fetches a real page with chrome profile", func(t *testing.T) {
		profile := getProfile("chrome")
		tr, err := newTransport(profile)
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		resp, body, err := doFetch(ctx, tr, profile, "GET", "https://httpbin.org/get", nil, nil)
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if len(body) == 0 {
			t.Fatal("expected non-empty body")
		}
	})
}
