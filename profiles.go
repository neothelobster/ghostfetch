package main

import (
	tls "github.com/refraction-networking/utls"
)

type BrowserProfile struct {
	Name      string
	TLSHello  tls.ClientHelloID
	Headers   [][2]string
}

func getProfile(name string) BrowserProfile {
	switch name {
	case "firefox":
		return firefoxProfile()
	case "chrome":
		return chromeProfile()
	default:
		return chromeProfile()
	}
}

func chromeProfile() BrowserProfile {
	return BrowserProfile{
		Name:     "chrome",
		TLSHello: tls.HelloChrome_Auto,
		Headers: [][2]string{
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
			{"Accept-Language", "en-US,en;q=0.9"},
			{"Accept-Encoding", "gzip, deflate, br, zstd"},
			{"Sec-Ch-Ua", `"Chromium";v="133", "Not(A:Brand";v="99", "Google Chrome";v="133"`},
			{"Sec-Ch-Ua-Mobile", "?0"},
			{"Sec-Ch-Ua-Platform", `"Windows"`},
			{"Sec-Fetch-Site", "none"},
			{"Sec-Fetch-Mode", "navigate"},
			{"Sec-Fetch-User", "?1"},
			{"Sec-Fetch-Dest", "document"},
			{"Upgrade-Insecure-Requests", "1"},
		},
	}
}

func firefoxProfile() BrowserProfile {
	return BrowserProfile{
		Name:     "firefox",
		TLSHello: tls.HelloFirefox_Auto,
		Headers: [][2]string{
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:134.0) Gecko/20100101 Firefox/134.0"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
			{"Accept-Language", "en-US,en;q=0.5"},
			{"Accept-Encoding", "gzip, deflate, br, zstd"},
			{"Sec-Fetch-Dest", "document"},
			{"Sec-Fetch-Mode", "navigate"},
			{"Sec-Fetch-Site", "none"},
			{"Sec-Fetch-User", "?1"},
			{"Upgrade-Insecure-Requests", "1"},
		},
	}
}
