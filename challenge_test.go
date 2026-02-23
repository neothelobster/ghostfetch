package main

import (
	"net/http"
	"testing"
)

func TestDetectChallenge(t *testing.T) {
	t.Run("no challenge on normal 200 response", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
		}
		ct := detectChallenge(resp, []byte("<html><body>Hello</body></html>"))
		if ct != ChallengeNone {
			t.Fatalf("expected ChallengeNone, got %v", ct)
		}
	})

	t.Run("detects cloudflare JS challenge", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 503,
			Header: http.Header{
				"Server": []string{"cloudflare"},
			},
		}
		body := []byte(`<html><head><title>Just a moment...</title></head><body><script>var s,t,o,p,a,C,H,challenge</script></body></html>`)
		ct := detectChallenge(resp, body)
		if ct != ChallengeJS {
			t.Fatalf("expected ChallengeJS, got %v", ct)
		}
	})

	t.Run("detects cloudflare managed challenge (turnstile)", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 403,
			Header: http.Header{
				"Server": []string{"cloudflare"},
			},
		}
		body := []byte(`<html><body><div id="turnstile-wrapper"><iframe src="https://challenges.cloudflare.com/turnstile"></iframe></div></body></html>`)
		ct := detectChallenge(resp, body)
		if ct != ChallengeCaptcha {
			t.Fatalf("expected ChallengeCaptcha, got %v", ct)
		}
	})

	t.Run("detects hcaptcha", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 403,
			Header:     http.Header{},
		}
		body := []byte(`<html><body><div class="h-captcha" data-sitekey="abcdef"></div></body></html>`)
		ct := detectChallenge(resp, body)
		if ct != ChallengeCaptcha {
			t.Fatalf("expected ChallengeCaptcha, got %v", ct)
		}
	})
}
