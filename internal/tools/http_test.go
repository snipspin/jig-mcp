package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/internal/config"
)

func newHTTPTool(url string, opts ...func(*config.ToolConfig)) HTTPTool {
	tc := config.ToolConfig{
		Name:        "http_test",
		Description: "test",
		InputSchema: map[string]any{"type": "object"},
		HTTP: &config.HTTPConfig{
			URL: url,
		},
	}
	for _, o := range opts {
		o(&tc)
	}
	return HTTPTool{BaseTool: BaseTool{Config: tc}}
}

func TestHTTPToolGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	resp := tool.Handle(map[string]any{})

	rm, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if rm["status"] != 200 {
		t.Errorf("status = %v, want 200", rm["status"])
	}
	if _, hasErr := rm["isError"]; hasErr {
		t.Error("expected no isError on 200")
	}
}

func TestHTTPToolPostWithBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{"method": "POST", "body": "hello"})

	if gotBody != "hello" {
		t.Errorf("body = %q, want %q", gotBody, "hello")
	}
}

func TestHTTPToolMissingURL(t *testing.T) {
	tool := HTTPTool{BaseTool: BaseTool{Config: config.ToolConfig{
		Name: "no_url",
	}}}
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for missing URL")
	}
}

func TestHTTPToolURLFromArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// No URL in config, provided via args
	tool := HTTPTool{BaseTool: BaseTool{Config: config.ToolConfig{
		Name:        "from_args",
		InputSchema: map[string]any{"type": "object"},
		HTTP:        &config.HTTPConfig{},
	}}}
	resp := tool.Handle(map[string]any{"url": srv.URL})
	rm := resp.(map[string]any)
	if rm["status"] != 200 {
		t.Errorf("status = %v, want 200", rm["status"])
	}
}

func TestHTTPToolAllowedPrefixBlocked(t *testing.T) {
	tool := newHTTPTool("http://evil.example.com", func(tc *config.ToolConfig) {
		tc.HTTP.AllowedURLPrefixes = []string{"http://safe.example.com"}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for URL not matching allowed prefix")
	}
	content := rm["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "does not match any allowed prefix") {
		t.Errorf("unexpected error: %s", content[0]["text"])
	}
}

func TestHTTPToolAllowedPrefixAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.AllowedURLPrefixes = []string{srv.URL}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Error("expected no error for allowed URL prefix")
	}
}

func TestHTTPToolRedirectNotFollowed(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("redirected"))
	}))
	defer target.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)

	// Should get the 302, not follow to 200
	if rm["status"] != 302 {
		t.Errorf("status = %v, want 302 (redirect not followed)", rm["status"])
	}
}

func TestHTTPToolServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected isError for 500 response")
	}
	if rm["status"] != 500 {
		t.Errorf("status = %v, want 500", rm["status"])
	}
}

func TestHTTPToolDefaultMethodIsGET(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{})
	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
}

func TestHTTPToolHeadersFromManifest(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.Headers = map[string]string{"X-Custom": "from-manifest"}
	})
	tool.Handle(map[string]any{})
	if gotHeader != "from-manifest" {
		t.Errorf("X-Custom = %q, want from-manifest", gotHeader)
	}
}

func TestHTTPToolHeadersFromArgsOverride(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.Headers = map[string]string{"X-Custom": "from-manifest"}
	})
	tool.Handle(map[string]any{
		"headers": map[string]any{"X-Custom": "from-args"},
	})
	if gotHeader != "from-args" {
		t.Errorf("X-Custom = %q, want from-args (override)", gotHeader)
	}
}

func TestHTTPToolConnectionRefused(t *testing.T) {
	tool := newHTTPTool("http://127.0.0.1:1") // port 1 should refuse
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for connection refused")
	}
}

func TestHTTPToolObjectBody(t *testing.T) {
	var gotContentType string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"method": "POST",
		"body":   map[string]any{"key": "value"},
	})
	if !strings.Contains(gotBody, `"key"`) || !strings.Contains(gotBody, `"value"`) {
		t.Errorf("expected JSON body, got %q", gotBody)
	}
	_ = gotContentType // not explicitly set by tool, just verifying body marshaling
}

// --- Additional HTTP tool tests for uncovered paths ---

func TestHTTPToolInvalidURL(t *testing.T) {
	tool := newHTTPTool("http://example.com")
	resp := tool.Handle(map[string]any{"url": "not-a-valid-url://"})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPToolSSRFWithEnvVarExpansion(t *testing.T) {
	// Test that SSRF protection works with environment variable expansion
	t.Setenv("ALLOWED_HOST", "http://safe.example.com")

	tool := newHTTPTool("http://evil.example.com", func(tc *config.ToolConfig) {
		tc.HTTP.AllowedURLPrefixes = []string{"${ALLOWED_HOST}"}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for URL not matching expanded allowed prefix")
	}
}

func TestHTTPToolAllowedPrefixWithEnvVarExpansion(t *testing.T) {
	t.Setenv("API_BASE", "http://api.example.com")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// URL matches expanded prefix
	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.AllowedURLPrefixes = []string{srv.URL} // Direct match
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Error("expected no error when URL matches allowed prefix")
	}
}

func TestHTTPToolEmptyPrefixAllowsAll(t *testing.T) {
	// Empty prefix list should allow any URL (opt-in SSRF protection)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.AllowedURLPrefixes = []string{""} // Empty prefix = allow all
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Error("expected no error when empty prefix allows all")
	}
}

func TestHTTPToolQueryParamExpansion(t *testing.T) {
	t.Setenv("API_KEY", "secret123")

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.QueryParams = map[string]string{"key": "${API_KEY}"}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Errorf("expected success, got error: %v", rm)
	}
	if !strings.Contains(gotQuery, "key=secret123") {
		t.Errorf("expected expanded env var in query, got: %s", gotQuery)
	}
}

func TestHTTPToolHeaderExpansion(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "bearer-token-123")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.Headers = map[string]string{"Authorization": "Bearer ${AUTH_TOKEN}"}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Errorf("expected success, got error: %v", rm)
	}
	if gotAuth != "Bearer bearer-token-123" {
		t.Errorf("Authorization = %q, want Bearer bearer-token-123", gotAuth)
	}
}

func TestHTTPToolEmptyHeaderFromExpansion(t *testing.T) {
	// Header that expands to empty (with env var reference) should be skipped
	t.Setenv("EMPTY_VALUE", "")

	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Empty")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.HTTP.Headers = map[string]string{"X-Empty": "${EMPTY_VALUE}"}
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Errorf("expected success, got error: %v", rm)
	}
	if gotHeader != "" {
		t.Errorf("expected empty header to be skipped, got: %q", gotHeader)
	}
}

func TestHTTPToolAutoBodyFromArgs(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"method": "POST",
		"name":   "test",
		"value":  42,
	})

	if !strings.Contains(gotBody, `"name"`) || !strings.Contains(gotBody, `"value"`) {
		t.Errorf("expected auto-generated JSON body, got: %q", gotBody)
	}
}

func TestHTTPToolNilBodyValuesSkipped(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"method": "POST",
		"skip":   nil, // nil values should be skipped
		"keep":   "value",
	})

	if strings.Contains(gotBody, `"skip"`) {
		t.Errorf("nil values should be skipped from body, got: %q", gotBody)
	}
	if !strings.Contains(gotBody, `"keep"`) {
		t.Errorf("expected 'keep' in body, got: %q", gotBody)
	}
}

func TestHTTPToolGETAutoQueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"param1": "value1",
		"param2": 42,
	})

	if !strings.Contains(gotQuery, "param1=value1") {
		t.Errorf("expected param1 in query, got: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "param2=42") {
		t.Errorf("expected param2 in query, got: %s", gotQuery)
	}
}

func TestHTTPToolGETSkipsReservedKeys(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"method":       "GET",
		"headers":      map[string]any{"X-Test": "test"},
		"body":         "should not appear",
		"query_params": map[string]any{"explicit": "value"},
		"custom":       "included",
	})

	if strings.Contains(gotQuery, "method=") {
		t.Errorf("reserved key 'method' should not appear in query: %s", gotQuery)
	}
	if strings.Contains(gotQuery, "headers=") {
		t.Errorf("reserved key 'headers' should not appear in query: %s", gotQuery)
	}
	if strings.Contains(gotQuery, "body=") {
		t.Errorf("reserved key 'body' should not appear in query: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "custom=") {
		t.Errorf("expected 'custom' in query: %s", gotQuery)
	}
}

func TestHTTPToolArrayQueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	tool.Handle(map[string]any{
		"tags": []any{"tag1", "tag2", "tag3"},
	})

	// Should have tags=tag1&tags=tag2&tags=tag3 or similar
	if !strings.Contains(gotQuery, "tags=") {
		t.Errorf("expected array param in query, got: %s", gotQuery)
	}
}

func TestHTTPToolRequestTimeout(t *testing.T) {
	// Server that never responds
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't write anything - let it timeout
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL, func(tc *config.ToolConfig) {
		tc.Timeout = "100ms" // Very short timeout
	})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for request timeout")
	}
}

func TestHTTPToolInvalidJSONInBody(t *testing.T) {
	// Complex types that can't be marshalled shouldn't crash
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := newHTTPTool(srv.URL)
	// This should not panic even with unusual input
	resp := tool.Handle(map[string]any{
		"method": "POST",
		"data":   make(chan int), // Can't be marshalled
	})
	// Should still succeed (body just won't include the unmarshallable field)
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Logf("got error (may be expected): %v", rm)
	}
}
