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

func formatOutput(w io.Writer, resp *http.Response, body []byte, asJSON bool) {
	if !asJSON {
		w.Write(body)
		return
	}

	out := JSONOutput{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    string(body),
	}
	if resp.Request != nil && resp.Request.URL != nil {
		out.URL = resp.Request.URL.String()
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
