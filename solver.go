package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// SolveResult holds the output from evaluating a JS challenge script.
type SolveResult struct {
	CookieName  string
	CookieValue string
	FormAction  string
	FormData    map[string]string
}

// JSSolver evaluates JavaScript challenge scripts in a sandboxed goja runtime
// with minimal DOM stubs, intercepting document.cookie assignments to extract
// solved tokens.
type JSSolver struct {
	pageURL string
}

func newJSSolver(pageURL string) *JSSolver {
	return &JSSolver{pageURL: pageURL}
}

// Solve executes the given JavaScript in a goja VM with DOM stubs.
// It returns the extracted cookie or form data, or an error if execution
// fails or times out.
func (s *JSSolver) Solve(script string) (*SolveResult, error) {
	vm := goja.New()
	result := &SolveResult{}

	// Set up a watchdog goroutine that interrupts the VM after 10 seconds.
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			vm.Interrupt("execution timeout")
		}
	}()
	defer close(done)

	s.setupGlobals(vm, result)

	_, err := vm.RunString(script)
	if err != nil {
		if intErr, ok := err.(*goja.InterruptedError); ok {
			return nil, fmt.Errorf("JS execution timed out: %v", intErr.Value())
		}
		return nil, fmt.Errorf("JS execution error: %w", err)
	}

	return result, nil
}

// setupGlobals registers browser-like globals in the goja VM so that
// typical JS challenge scripts can execute: atob/btoa, setTimeout, console,
// document (with cookie interception), window.location, and navigator.
func (s *JSSolver) setupGlobals(vm *goja.Runtime, result *SolveResult) {
	parsedURL, _ := url.Parse(s.pageURL)

	// atob: decode base64
	vm.Set("atob", func(call goja.FunctionCall) goja.Value {
		encoded := call.Argument(0).String()
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			panic(vm.NewTypeError("invalid base64"))
		}
		return vm.ToValue(string(decoded))
	})

	// btoa: encode base64
	vm.Set("btoa", func(call goja.FunctionCall) goja.Value {
		raw := call.Argument(0).String()
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(raw)))
	})

	// setTimeout: executes the callback immediately (no real async needed)
	vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		if fn, ok := goja.AssertFunction(call.Argument(0)); ok {
			fn(goja.Undefined())
		}
		return vm.ToValue(0)
	})

	// console: no-op stubs
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
	console.Set("error", func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
	vm.Set("console", console)

	// __setCookie: internal helper called from the document.cookie setter
	vm.Set("__setCookie", func(call goja.FunctionCall) goja.Value {
		cookieStr := call.Argument(0).String()
		parts := strings.SplitN(cookieStr, ";", 2)
		if len(parts) > 0 {
			kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
			if len(kv) == 2 {
				result.CookieName = kv[0]
				result.CookieValue = kv[1]
			}
		}
		return goja.Undefined()
	})

	// document object with DOM stubs
	document := vm.NewObject()
	document.Set("createElement", func(call goja.FunctionCall) goja.Value {
		tagName := call.Argument(0).String()
		elem := vm.NewObject()
		elem.Set("tagName", strings.ToUpper(tagName))
		elem.Set("innerHTML", "")
		elem.Set("setAttribute", func(c goja.FunctionCall) goja.Value {
			elem.Set(c.Argument(0).String(), c.Argument(1).String())
			return goja.Undefined()
		})
		elem.Set("getAttribute", func(c goja.FunctionCall) goja.Value {
			v := elem.Get(c.Argument(0).String())
			if v == nil {
				return goja.Null()
			}
			return v
		})
		return elem
	})
	document.Set("getElementById", func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	})
	document.Set("getElementsByTagName", func(call goja.FunctionCall) goja.Value {
		return vm.NewArray()
	})
	vm.Set("document", document)

	// Define document.cookie as a property with getter/setter so that
	// assignments like `document.cookie = "name=value"` are intercepted.
	vm.RunString(`
		Object.defineProperty(document, "cookie", {
			get: function() { return ""; },
			set: function(v) { __setCookie(v); },
			configurable: true
		});
	`)

	// window object
	window := vm.NewObject()
	if parsedURL != nil {
		loc := vm.NewObject()
		loc.Set("href", s.pageURL)
		loc.Set("hostname", parsedURL.Hostname())
		loc.Set("pathname", parsedURL.Path)
		loc.Set("protocol", parsedURL.Scheme+":")
		loc.Set("host", parsedURL.Host)
		window.Set("location", loc)
	}
	vm.Set("window", window)
	vm.Set("location", window.Get("location"))

	// navigator object
	navigator := vm.NewObject()
	navigator.Set("userAgent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	navigator.Set("language", "en-US")
	navigator.Set("languages", vm.NewArray("en-US", "en"))
	navigator.Set("platform", "Win32")
	vm.Set("navigator", navigator)
}
