package main

import (
	"bytes"
	"net/http"
	"strings"
)

type ChallengeType int

const (
	ChallengeNone    ChallengeType = iota
	ChallengeJS
	ChallengeCaptcha
)

func (c ChallengeType) String() string {
	switch c {
	case ChallengeNone:
		return "none"
	case ChallengeJS:
		return "js"
	case ChallengeCaptcha:
		return "captcha"
	default:
		return "unknown"
	}
}

func detectChallenge(resp *http.Response, body []byte) ChallengeType {
	server := strings.ToLower(resp.Header.Get("Server"))
	isCloudflare := strings.Contains(server, "cloudflare")

	// Check for captcha challenges first (higher priority)
	if containsAny(body, [][]byte{
		[]byte("turnstile"),
		[]byte("challenges.cloudflare.com"),
		[]byte("h-captcha"),
		[]byte("data-sitekey"),
		[]byte("g-recaptcha"),
		[]byte("www.google.com/recaptcha"),
	}) {
		return ChallengeCaptcha
	}

	// Check for Cloudflare JS challenge
	if isCloudflare && (resp.StatusCode == 503 || resp.StatusCode == 403) {
		if containsAny(body, [][]byte{
			[]byte("Just a moment"),
			[]byte("_cf_chl"),
			[]byte("cf-challenge"),
			[]byte("jschl_vc"),
			[]byte("jschl_answer"),
		}) {
			return ChallengeJS
		}
	}

	// Generic JS redirect detection
	if resp.StatusCode == 503 && containsAny(body, [][]byte{
		[]byte("<noscript>"),
		[]byte("document.cookie"),
	}) && len(body) < 10000 {
		return ChallengeJS
	}

	return ChallengeNone
}

func containsAny(body []byte, patterns [][]byte) bool {
	for _, p := range patterns {
		if bytes.Contains(body, p) {
			return true
		}
	}
	return false
}
