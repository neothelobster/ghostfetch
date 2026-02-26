# ghostfetch

Search and fetch the web like a ghost — invisible to bot detection.

`ghostfetch` (or `gf`) is a CLI tool that performs web searches and fetches pages using browser-like TLS fingerprints. It bypasses Cloudflare, bot detection, and anti-scraping measures without running a full browser.

Built for LLMs and automation.

## Install

```bash
go install github.com/x/ghostfetch@latest
```

Or build from source:

```bash
git clone https://github.com/vyakovishak/ghostfetch.git
cd ghostfetch
go build -o ghostfetch .
```

## Usage

### Search the web

```bash
ghostfetch "golang tutorial"
ghostfetch -e brave "rust programming"
ghostfetch -e bing "python flask" -n 5
ghostfetch --json "linux kernel"
```

Engines: `duckduckgo` (default), `brave`, `bing`, `google`

### Fetch a page

```bash
ghostfetch https://example.com                    # raw HTML
ghostfetch fetch https://example.com -m           # markdown (reader mode)
ghostfetch fetch https://example.com --markdown-full  # full page markdown
ghostfetch fetch https://example.com --json       # JSON with headers/status
```

### Parallel fetch

```bash
ghostfetch fetch url1 url2 url3 -p 3
```

### Extract links

```bash
ghostfetch links https://example.com
ghostfetch links https://example.com -f "github"  # filter by regex
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--engine` | `-e` | Search engine: duckduckgo, bing, brave, google |
| `--results` | `-n` | Number of search results (default 10) |
| `--browser` | `-b` | Browser to impersonate: chrome, firefox |
| `--markdown` | `-m` | Convert to markdown (reader mode) |
| `--markdown-full` | | Full page markdown |
| `--json` | `-j` | JSON output with metadata |
| `--raw` | | Raw HTML output |
| `--header` | `-H` | Custom header (repeatable) |
| `--method` | `-X` | HTTP method (default GET) |
| `--data` | `-d` | Request body |
| `--timeout` | `-t` | Request timeout (default 30s) |
| `--max-parallel` | `-p` | Max parallel fetches (default 5) |
| `--filter` | `-f` | Filter links by regex |
| `--verbose` | `-v` | Verbose output |
| `--no-cookies` | | Disable cookie jar |

## How it works

- **TLS fingerprinting** — Uses [uTLS](https://github.com/refraction-networking/utls) to mimic Chrome 133 or Firefox 134 TLS handshakes
- **HTTP/2** — Full HTTP/2 support with browser-like ALPN negotiation
- **JS challenge solving** — Solves JavaScript challenges using an embedded JS runtime ([goja](https://github.com/nicolo-john/goja))
- **Captcha integration** — Supports 2captcha and anticaptcha services
- **Persistent cookies** — Cookie jar persisted across requests
- **Content decoding** — Handles gzip and brotli compression

## License

MIT
