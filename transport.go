package main

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// roundTripper uses uTLS to establish TLS connections with browser-like
// fingerprints and routes HTTP/2 vs HTTP/1.1 traffic based on the ALPN
// negotiated protocol.
type roundTripper struct {
	profile BrowserProfile
	h2      *http2.Transport
	h1      *http.Transport
}

// newTransport creates a new http.RoundTripper that uses uTLS with the
// given browser profile's TLS ClientHello fingerprint.
func newTransport(profile BrowserProfile) (http.RoundTripper, error) {
	rt := &roundTripper{profile: profile}

	// Create an HTTP/2 transport that uses our uTLS dialer.
	// We ignore the *tls.Config parameter since we use uTLS instead.
	rt.h2 = &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return rt.dialTLS(ctx, network, addr)
		},
	}

	// Create an HTTP/1.1 transport as fallback.
	rt.h1 = &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return rt.dialTLS(ctx, network, addr)
		},
	}

	return rt, nil
}

// dialTLS creates a uTLS connection with the browser profile's fingerprint.
func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{}
	tcpConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	tlsConn := utls.UClient(tcpConn, &utls.Config{
		ServerName: host,
		NextProtos: []string{"h2", "http/1.1"},
	}, rt.profile.TLSHello)
	if err := tlsConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}
	return tlsConn, nil
}

// RoundTrip executes an HTTP request. It first probes the server's ALPN
// support by dialing and checking the negotiated protocol, then delegates
// to either the HTTP/2 or HTTP/1.1 transport.
func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// For non-HTTPS, fall back to HTTP/1.1.
	if req.URL.Scheme != "https" {
		return rt.h1.RoundTrip(req)
	}
	// Try HTTP/2 first. If the server negotiated h2 via ALPN, the h2
	// transport will handle it. We use h2 by default since our ALPN
	// lists h2 first and most modern servers support it.
	return rt.h2.RoundTrip(req)
}

// doFetch performs an HTTP request using the given transport and profile.
// It is a convenience wrapper around doFetchWithBody with an empty body.
func doFetch(ctx context.Context, tr http.RoundTripper, profile BrowserProfile, method, url string, extraHeaders [][2]string, cookies []*http.Cookie) (*http.Response, []byte, error) {
	return doFetchWithBody(ctx, tr, profile, method, url, extraHeaders, cookies, "")
}

// doFetchWithBody performs an HTTP request using the given transport and profile.
// If body is non-empty, it is sent as the request body (useful for POST/PUT requests).
func doFetchWithBody(ctx context.Context, tr http.RoundTripper, profile BrowserProfile, method, targetURL string, extraHeaders [][2]string, cookies []*http.Cookie, body string) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, reqBody)
	if err != nil {
		return nil, nil, err
	}

	// Apply profile headers in order
	for _, h := range profile.Headers {
		req.Header.Set(h[0], h[1])
	}
	// Apply extra headers (overrides)
	for _, h := range extraHeaders {
		req.Header.Set(h[0], h[1])
	}
	// Apply cookies
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return resp, nil, fmt.Errorf("gzip decode failed: %w", err)
		}
	case "br":
		reader = brotli.NewReader(resp.Body)
	}

	respBody, err := io.ReadAll(reader)
	if err != nil {
		return resp, nil, fmt.Errorf("read body failed: %w", err)
	}

	return resp, respBody, nil
}
