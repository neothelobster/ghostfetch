package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type JSONOutput struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
	URL     string              `json:"url,omitempty"`
}

type outputOptions struct {
	asJSON       bool
	markdown     bool // reader mode: extract main content + convert to markdown
	markdownFull bool // full page HTML-to-markdown
	pageURL      string
}

func formatOutput(w io.Writer, resp *http.Response, body []byte, opts outputOptions) {
	content := string(body)

	// Apply markdown conversion if requested.
	if opts.markdown || opts.markdownFull {
		readerMode := opts.markdown // --markdown uses reader mode, --markdown-full does not
		md, err := htmlToMarkdown(content, opts.pageURL, readerMode)
		if err == nil {
			content = md
		}
		// On error, fall through with raw HTML.
	}

	if !opts.asJSON {
		w.Write([]byte(content))
		return
	}

	out := JSONOutput{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    content,
	}
	if resp.Request != nil && resp.Request.URL != nil {
		out.URL = resp.Request.URL.String()
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
