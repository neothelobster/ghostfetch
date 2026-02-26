# ghostfetch

LLM-focused web search and fetch tool — invisible to bot detection.

`ghostfetch` is a read-only CLI tool designed for LLM agents and AI tools like OpenClaw, LangChain, AutoGPT, and custom agents that need to search and read the web. It bypasses Cloudflare, bot detection, and anti-scraping measures using browser-like TLS fingerprints — no headless browser required.

## Why ghostfetch?

LLMs need web access but most tools get blocked. `ghostfetch` solves this:

- **Search** — Query DuckDuckGo, Brave, Bing, or Google and get clean, structured results
- **Fetch** — Get any page as markdown (LLM-ready) or JSON
- **Parallel** — Fetch multiple URLs at once for fast research
- **Links** — Extract and filter links from any page
- **Unblocked** — TLS fingerprint spoofing bypasses bot detection on most sites
- **No browser** — Single binary, no Chromium, no Playwright, no Selenium

## Security

ghostfetch is designed to be safe for LLM agent use:

- **Read-only** — GET requests only, no POST/PUT/DELETE, no request body
- **Stdout-only** — All output goes to stdout, no file write capability
- **No custom headers** — Cannot be used to exfiltrate data via HTTP headers
- **No credentials in CLI** — Captcha services configured via environment variables only
- **No arbitrary requests** — No custom HTTP methods or request bodies

## Install

```bash
git clone https://github.com/vyakovishak/ghostfetch.git
cd ghostfetch
go build -o ghostfetch .
```

## Quick start

```bash
# Search the web (default action)
ghostfetch "how to deploy a Go app"

# Fetch a page as markdown (perfect for LLM context)
ghostfetch fetch https://docs.example.com -m

# Search + fetch workflow (what LLM agents typically do)
ghostfetch "best Go web frameworks 2025"          # step 1: search
ghostfetch fetch https://result-url.com -m         # step 2: read result
```

## Usage

### Search

```bash
ghostfetch "golang tutorial"
ghostfetch -e brave "rust programming"
ghostfetch -e bing "python flask" -n 5
ghostfetch --json "linux kernel"
```

Engines: `duckduckgo` (default), `brave`, `bing`, `google`

### Fetch

```bash
ghostfetch https://example.com                        # raw HTML
ghostfetch fetch https://example.com -m               # markdown (reader mode)
ghostfetch fetch https://example.com --markdown-full  # full page markdown
ghostfetch fetch https://example.com --json           # JSON with headers/status
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

## LLM integration

ghostfetch outputs are designed to be consumed by LLMs:

- **Search results** — Clean markdown with numbered results, titles, URLs, and snippets
- **Page content** — Reader-mode markdown strips nav, ads, and boilerplate
- **JSON mode** — Structured output with status, headers, body, and URL
- **Links** — Simple list for follow-up fetching

### Example: tool definition for an LLM agent

```json
{
  "name": "web_search",
  "command": "ghostfetch --json \"{{query}}\"",
  "description": "Search the web and return results as JSON"
}
```

```json
{
  "name": "read_page",
  "command": "ghostfetch fetch -m \"{{url}}\"",
  "description": "Fetch a web page and return content as markdown"
}
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
| `--timeout` | `-t` | Request timeout (default 30s) |
| `--max-parallel` | `-p` | Max parallel fetches (default 5) |
| `--filter` | `-f` | Filter links by regex |
| `--verbose` | `-v` | Verbose output |
| `--no-cookies` | | Disable cookie jar |

## How it works

- **TLS fingerprinting** — Uses [uTLS](https://github.com/refraction-networking/utls) to mimic Chrome 133 or Firefox 134 TLS handshakes
- **HTTP/2** — Full HTTP/2 support with browser-like ALPN negotiation
- **JS challenge solving** — Solves JavaScript challenges using an embedded JS runtime
- **Persistent cookies** — Cookie jar persisted across requests
- **Content decoding** — Handles gzip and brotli compression

## License

MIT
