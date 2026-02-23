package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestFormatOutput(t *testing.T) {
	t.Run("raw output is just the body", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
		}
		body := []byte("<html>hello</html>")
		var buf bytes.Buffer
		formatOutput(&buf, resp, body, false)
		if buf.String() != "<html>hello</html>" {
			t.Fatalf("unexpected output: %q", buf.String())
		}
	})

	t.Run("json output includes metadata", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Request:    &http.Request{},
		}
		body := []byte("<html>hello</html>")
		var buf bytes.Buffer
		formatOutput(&buf, resp, body, true)

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if result["status"].(float64) != 200 {
			t.Fatalf("expected status 200, got %v", result["status"])
		}
		if result["body"].(string) != "<html>hello</html>" {
			t.Fatalf("unexpected body: %v", result["body"])
		}
	})
}
