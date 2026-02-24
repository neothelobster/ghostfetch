# LLM-Friendly Browsing Subcommands Design

## Goal

Add subcommands to brwoser so LLMs can search the web, fetch multiple pages in parallel, and extract links -- all using the existing anti-detection stack.

## Subcommands

### `brwoser fetch <url> [url2] [url3...]`

Fetch one or more URLs concurrently. Replaces bare-URL usage as the primary command.

- `--max-parallel` / `-p`: concurrency limit (default 5)
- `--markdown` / `-m`: convert to markdown (default for multi-URL)
- `--markdown-full`: full page markdown (no reader mode extraction)
- `--json` / `-j`: structured JSON output (array for multiple URLs)
- `--raw`: plain HTML output
- Errors per-URL reported inline, don't fail the batch
- Existing flags (--browser, --cookie-jar, --header, --timeout, etc.) all apply

Output format for multi-URL:
```
---
# Page: <title>
url: <url>
---

<markdown content>
```

### `brwoser search [--engine] <query>`

Search the web and return results as a numbered markdown list.

- `--engine` / `-e`: google (default), bing, duckduckgo, brave
- `--results` / `-n`: max results to return (default 10)
- Uses same TLS spoofing and cookie jar

Output format:
```
## Search: "<query>"

1. **[Title](url)**
   Snippet text...

2. **[Title](url)**
   Snippet text...
```

Each engine has its own HTML parser since result markup differs.

### `brwoser links <url>`

Fetch a page and extract all links.

- `--filter` / `-f`: filter links by pattern (regex)
- `--absolute`: resolve relative URLs to absolute (default true)
- Output: one link per line, or markdown list with link text

### Backward Compatibility

`brwoser <url>` (no subcommand, bare URL) continues to work as shortcut for `brwoser fetch <url>`.

## Search Engine Parsers

| Engine | Search URL pattern | Result selector strategy |
|--------|-------------------|------------------------|
| Google | `google.com/search?q=` | Parse `<div class="g">` result blocks |
| Bing | `bing.com/search?q=` | Parse `<li class="b_algo">` blocks |
| DuckDuckGo | `html.duckduckgo.com/html/?q=` | Parse `<div class="result">` blocks (HTML-only endpoint) |
| Brave | `search.brave.com/search?q=` | Parse `<div class="snippet">` blocks |

## Parallel Fetch

- Uses goroutines with `sync.WaitGroup` and a semaphore channel for concurrency limit
- Results collected into a slice, output in input-URL order (not completion order)
- Each fetch uses its own challenge detection/solving cycle
- Shared cookie jar (thread-safe via mutex in PersistentJar)

## Architecture

All subcommands share:
- TLS transport (profiles.go, transport.go)
- Cookie jar (cookies.go)
- Challenge detection + solving (challenge.go, solver.go, captcha.go)
- Markdown conversion (markdown.go)
- Output formatting (output.go)

New files:
- `search.go` - search engine abstraction + parsers
- `search_test.go` - search parser tests
- `links.go` - link extraction
- `links_test.go` - link extraction tests
- `parallel.go` - parallel fetch orchestration
- `parallel_test.go` - parallel fetch tests
- `main.go` - restructured with cobra subcommands
