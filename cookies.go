package main

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

// PersistentJar wraps Go's cookiejar.Jar with JSON persistence.
// Since cookiejar.Jar doesn't expose enumeration of stored cookies,
// we maintain a parallel tracking slice that records every cookie
// set via SetCookies, allowing us to serialize them to disk.
type PersistentJar struct {
	jar     *cookiejar.Jar
	path    string
	mu      sync.Mutex
	tracked []savedCookie
}

type savedCookie struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Domain  string    `json:"domain"`
	Path    string    `json:"path"`
	Expires time.Time `json:"expires"`
	Secure  bool      `json:"secure"`
	URL     string    `json:"url"`
}

func newPersistentJar(path string) *PersistentJar {
	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	return &PersistentJar{jar: jar, path: path}
}

func (p *PersistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jar.SetCookies(u, cookies)
	for _, c := range cookies {
		urlKey := u.Scheme + "://" + u.Host
		// Remove existing entry for same name+url to avoid duplicates
		for i, tc := range p.tracked {
			if tc.Name == c.Name && tc.URL == urlKey {
				p.tracked = append(p.tracked[:i], p.tracked[i+1:]...)
				break
			}
		}
		p.tracked = append(p.tracked, savedCookie{
			Name:    c.Name,
			Value:   c.Value,
			Domain:  c.Domain,
			Path:    c.Path,
			Expires: c.Expires,
			Secure:  c.Secure,
			URL:     urlKey,
		})
	}
}

func (p *PersistentJar) Cookies(u *url.URL) []*http.Cookie {
	return p.jar.Cookies(u)
}

// Save writes all non-expired tracked cookies to the JSON file on disk.
func (p *PersistentJar) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return err
	}
	now := time.Now()
	var active []savedCookie
	for _, sc := range p.tracked {
		if !sc.Expires.IsZero() && sc.Expires.Before(now) {
			continue
		}
		active = append(active, sc)
	}
	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.path, data, 0600)
}

// Load reads cookies from the JSON file on disk, skipping expired entries.
// If the file does not exist, Load returns nil (no error).
func (p *PersistentJar) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved []savedCookie
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	now := time.Now()
	for _, sc := range saved {
		if !sc.Expires.IsZero() && sc.Expires.Before(now) {
			continue
		}
		u, err := url.Parse(sc.URL)
		if err != nil {
			continue
		}
		p.jar.SetCookies(u, []*http.Cookie{
			{Name: sc.Name, Value: sc.Value, Domain: sc.Domain, Path: sc.Path, Expires: sc.Expires, Secure: sc.Secure},
		})
		p.tracked = append(p.tracked, sc)
	}
	return nil
}
