package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var sitekeyRe = regexp.MustCompile(`data-sitekey=["']([^"']+)["']`)

// extractSitekey scans an HTML body for a data-sitekey attribute and
// determines the captcha type by looking for known class markers.
// It returns the sitekey and the captcha type ("turnstile", "hcaptcha",
// "recaptcha") or empty strings if none is found.
func extractSitekey(body []byte) (sitekey string, captchaType string) {
	m := sitekeyRe.FindSubmatch(body)
	if m == nil {
		return "", ""
	}
	sitekey = string(m[1])

	switch {
	case bytes.Contains(body, []byte("cf-turnstile")) || bytes.Contains(body, []byte("turnstile")):
		captchaType = "turnstile"
	case bytes.Contains(body, []byte("h-captcha")):
		captchaType = "hcaptcha"
	case bytes.Contains(body, []byte("g-recaptcha")):
		captchaType = "recaptcha"
	default:
		captchaType = "unknown"
	}
	return sitekey, captchaType
}

// CaptchaSolver dispatches captcha-solving requests to an external service
// such as 2captcha or anticaptcha, then polls for the result.
type CaptchaSolver struct {
	service string
	apiKey  string
	baseURL string
	client  *http.Client
}

// newCaptchaSolver creates a CaptchaSolver for the given service name.
// Supported services are "2captcha" and "anticaptcha".
func newCaptchaSolver(service, apiKey string) (*CaptchaSolver, error) {
	s := &CaptchaSolver{
		service: service,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}

	switch service {
	case "2captcha":
		s.baseURL = "https://2captcha.com"
	case "anticaptcha":
		s.baseURL = "https://api.anti-captcha.com"
	default:
		return nil, fmt.Errorf("unsupported captcha service: %q (supported: 2captcha, anticaptcha)", service)
	}

	return s, nil
}

// Solve submits a captcha challenge to the configured service and polls
// until the solution is available or the context is cancelled. It returns
// the solved token string.
func (s *CaptchaSolver) Solve(ctx context.Context, sitekey, pageURL, captchaType string) (string, error) {
	switch s.service {
	case "2captcha":
		return s.solve2Captcha(ctx, sitekey, pageURL, captchaType)
	case "anticaptcha":
		return s.solveAntiCaptcha(ctx, sitekey, pageURL, captchaType)
	default:
		return "", fmt.Errorf("unsupported captcha service: %q", s.service)
	}
}

// solve2Captcha implements the 2captcha submit-then-poll flow.
// Submit: POST to /in.php with method, key, sitekey, pageurl, json=1
// Poll:   GET /res.php?action=get&id=<id>&key=<key>&json=1
func (s *CaptchaSolver) solve2Captcha(ctx context.Context, sitekey, pageURL, captchaType string) (string, error) {
	method := twoCaptchaMethod(captchaType)

	// Submit the captcha task.
	form := url.Values{
		"key":     {s.apiKey},
		"method":  {method},
		"sitekey": {sitekey},
		"pageurl": {pageURL},
		"json":    {"1"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/in.php", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("2captcha: build submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("2captcha: submit request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("2captcha: read submit response: %w", err)
	}

	var submitResp struct {
		Status  int    `json:"status"`
		Request string `json:"request"`
	}
	if err := json.Unmarshal(body, &submitResp); err != nil {
		return "", fmt.Errorf("2captcha: parse submit response: %w", err)
	}
	if submitResp.Status != 1 {
		return "", fmt.Errorf("2captcha: submit failed: %s", submitResp.Request)
	}

	taskID := submitResp.Request

	// Poll for the result.
	pollURL := fmt.Sprintf("%s/res.php?key=%s&action=get&id=%s&json=1",
		s.baseURL, url.QueryEscape(s.apiKey), url.QueryEscape(taskID))

	const maxPolls = 60
	const pollInterval = 2 * time.Second

	for i := 0; i < maxPolls; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}

		pollReq, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		if err != nil {
			return "", fmt.Errorf("2captcha: build poll request: %w", err)
		}

		pollResp, err := s.client.Do(pollReq)
		if err != nil {
			return "", fmt.Errorf("2captcha: poll request: %w", err)
		}

		pollBody, err := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("2captcha: read poll response: %w", err)
		}

		var result struct {
			Status  int    `json:"status"`
			Request string `json:"request"`
		}
		if err := json.Unmarshal(pollBody, &result); err != nil {
			return "", fmt.Errorf("2captcha: parse poll response: %w", err)
		}

		if result.Status == 1 {
			return result.Request, nil
		}

		if result.Request != "CAPCHA_NOT_READY" {
			return "", fmt.Errorf("2captcha: solve failed: %s", result.Request)
		}
	}

	return "", fmt.Errorf("2captcha: timed out after %d polls", maxPolls)
}

// solveAntiCaptcha implements the anti-captcha createTask/getTaskResult flow.
func (s *CaptchaSolver) solveAntiCaptcha(ctx context.Context, sitekey, pageURL, captchaType string) (string, error) {
	taskType := antiCaptchaTaskType(captchaType)

	// Submit the captcha task.
	createPayload := map[string]interface{}{
		"clientKey": s.apiKey,
		"task": map[string]interface{}{
			"type":       taskType,
			"websiteURL": pageURL,
			"websiteKey": sitekey,
		},
	}

	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		return "", fmt.Errorf("anticaptcha: marshal create request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/createTask", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("anticaptcha: build create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anticaptcha: create request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("anticaptcha: read create response: %w", err)
	}

	var createResp struct {
		ErrorID          int    `json:"errorId"`
		ErrorCode        string `json:"errorCode"`
		ErrorDescription string `json:"errorDescription"`
		TaskID           int    `json:"taskId"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("anticaptcha: parse create response: %w", err)
	}
	if createResp.ErrorID != 0 {
		return "", fmt.Errorf("anticaptcha: create failed: %s (%s)", createResp.ErrorCode, createResp.ErrorDescription)
	}

	// Poll for the result.
	const maxPolls = 60
	const pollInterval = 2 * time.Second

	for i := 0; i < maxPolls; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}

		pollPayload, err := json.Marshal(map[string]interface{}{
			"clientKey": s.apiKey,
			"taskId":    createResp.TaskID,
		})
		if err != nil {
			return "", fmt.Errorf("anticaptcha: marshal poll request: %w", err)
		}

		pollReq, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/getTaskResult", bytes.NewReader(pollPayload))
		if err != nil {
			return "", fmt.Errorf("anticaptcha: build poll request: %w", err)
		}
		pollReq.Header.Set("Content-Type", "application/json")

		pollResp, err := s.client.Do(pollReq)
		if err != nil {
			return "", fmt.Errorf("anticaptcha: poll request: %w", err)
		}

		pollBody, err := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("anticaptcha: read poll response: %w", err)
		}

		var result struct {
			ErrorID  int    `json:"errorId"`
			Status   string `json:"status"`
			Solution struct {
				Token          string `json:"token"`
				GRecaptchaResp string `json:"gRecaptchaResponse"`
			} `json:"solution"`
			ErrorCode        string `json:"errorCode"`
			ErrorDescription string `json:"errorDescription"`
		}
		if err := json.Unmarshal(pollBody, &result); err != nil {
			return "", fmt.Errorf("anticaptcha: parse poll response: %w", err)
		}

		if result.ErrorID != 0 {
			return "", fmt.Errorf("anticaptcha: solve failed: %s (%s)", result.ErrorCode, result.ErrorDescription)
		}

		if result.Status == "ready" {
			token := result.Solution.Token
			if token == "" {
				token = result.Solution.GRecaptchaResp
			}
			return token, nil
		}

		// status == "processing", keep polling
	}

	return "", fmt.Errorf("anticaptcha: timed out after %d polls", maxPolls)
}

// twoCaptchaMethod maps captcha types to 2captcha method parameters.
func twoCaptchaMethod(captchaType string) string {
	switch captchaType {
	case "turnstile":
		return "turnstile"
	case "hcaptcha":
		return "hcaptcha"
	case "recaptcha":
		return "userrecaptcha"
	default:
		return "userrecaptcha"
	}
}

// antiCaptchaTaskType maps captcha types to anti-captcha task type strings.
func antiCaptchaTaskType(captchaType string) string {
	switch captchaType {
	case "turnstile":
		return "TurnstileTaskProxyless"
	case "hcaptcha":
		return "HCaptchaTaskProxyless"
	case "recaptcha":
		return "RecaptchaV2TaskProxyless"
	default:
		return "RecaptchaV2TaskProxyless"
	}
}
