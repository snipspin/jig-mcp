package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// defaultHTTPTimeout is the default timeout for HTTP tool requests.
const defaultHTTPTimeout = 15 * time.Second

// HTTPTool bridges MCP tool calls to HTTP/REST APIs, with SSRF prevention
// via configurable URL prefix allowlists.
type HTTPTool struct {
	BaseTool
}

// Handle executes the HTTP request and returns an MCP result with the response body and status.
func (t HTTPTool) Handle(args map[string]any) any {
	urlStr, _ := args["url"].(string)
	if urlStr == "" && t.Config.HTTP != nil {
		urlStr = t.Config.HTTP.URL
	}
	if urlStr == "" {
		return errorResult("missing mandatory parameter: url")
	}

	// Expand environment variables in URL
	urlStr = os.ExpandEnv(urlStr)

	// Check URL against allowed prefixes if configured
	if t.Config.HTTP != nil && len(t.Config.HTTP.AllowedURLPrefixes) > 0 {
		allowed := false
		hasNonEmptyPrefix := false
		for _, prefix := range t.Config.HTTP.AllowedURLPrefixes {
			// Expand env vars in prefix too for comparison
			expandedPrefix := os.ExpandEnv(prefix)
			// Track if there's at least one non-empty prefix
			if expandedPrefix != "" {
				hasNonEmptyPrefix = true
			}
			// Skip empty prefixes (allows optional SSRF protection)
			if expandedPrefix == "" && strings.Contains(prefix, "${") {
				allowed = true
				break
			}
			if expandedPrefix != "" && strings.HasPrefix(urlStr, expandedPrefix) {
				allowed = true
				break
			}
		}
		// Only fail if there are non-empty prefixes and none matched
		if !allowed && hasNonEmptyPrefix {
			return errorResult(fmt.Sprintf("URL %q does not match any allowed prefix", urlStr))
		}
	}

	// Determine HTTP method early (needed for query param mapping)
	method, _ := args["method"].(string)
	if method == "" && t.Config.HTTP != nil {
		method = os.ExpandEnv(t.Config.HTTP.Method)
	}
	if method == "" {
		method = "GET"
	}

	// Build query parameters from args
	// First, add explicit query_params if provided
	// Then, add all other non-reserved args as query params (for GET requests)
	u, err := url.Parse(urlStr)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid URL: %v", err))
	}
	values := u.Query()

	// Add default query params from manifest config first
	if t.Config.HTTP != nil && len(t.Config.HTTP.QueryParams) > 0 {
		for k, v := range t.Config.HTTP.QueryParams {
			values.Set(k, os.ExpandEnv(v))
		}
	}

	// Add explicit query_params from args
	if qp, ok := args["query_params"].(map[string]any); ok && len(qp) > 0 {
		for k, v := range qp {
			switch val := v.(type) {
			case string:
				values.Set(k, val)
			case []any:
				for _, item := range val {
					values.Add(k, fmt.Sprint(item))
				}
			default:
				values.Set(k, fmt.Sprint(val))
			}
		}
	}

	// Auto-map other input args to query params (excluding reserved keys)
	reservedKeys := map[string]bool{
		"url": true, "method": true, "headers": true, "body": true, "query_params": true,
	}
	if method == "GET" {
		for k, v := range args {
			if reservedKeys[k] {
				continue
			}
			// Skip null/empty values
			switch val := v.(type) {
			case string:
				if val != "" {
					values.Set(k, val)
				}
			case int, int64, float64, bool:
				values.Set(k, fmt.Sprint(val))
			case nil:
				// Skip nil values
			default:
				// For complex types, try JSON serialization
				if data, err := json.Marshal(val); err == nil {
					values.Set(k, string(data))
				}
			}
		}
	}

	u.RawQuery = values.Encode()
	urlStr = u.String()

	// Build request body
	var bodyReader io.Reader
	if b, ok := args["body"]; ok {
		// Explicit body from args takes precedence
		switch v := b.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			data, _ := json.Marshal(v)
			bodyReader = bytes.NewReader(data)
		}
	} else if method == "POST" || method == "PUT" || method == "PATCH" {
		// Auto-build JSON body from input args (excluding reserved keys)
		bodyMap := make(map[string]any)
		reservedKeys := map[string]bool{
			"url": true, "method": true, "headers": true, "body": true, "query_params": true,
		}
		for k, v := range args {
			if reservedKeys[k] {
				continue
			}
			// Skip nil values
			if v == nil {
				continue
			}
			bodyMap[k] = v
		}
		if len(bodyMap) > 0 {
			data, _ := json.Marshal(bodyMap)
			bodyReader = bytes.NewReader(data)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.effectiveTimeout(defaultHTTPTimeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	// Apply headers from manifest (expanding env vars)
	if t.Config.HTTP != nil {
		for k, v := range t.Config.HTTP.Headers {
			expanded := os.ExpandEnv(v)
			// Skip headers that expand to empty (allows optional auth)
			// Only skip if the original value contained an env var reference
			if expanded == "" && strings.Contains(v, "${") {
				continue
			}
			req.Header.Set(k, expanded)
		}
	}

	// Override with headers from tool call
	if h, ok := args["headers"].(map[string]any); ok {
		for k, v := range h {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}

	client := &http.Client{
		// Do not follow redirects — a redirect from an allowed URL prefix
		// to an internal service would bypass allowedURLPrefixes (SSRF).
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return errorResult(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	// Limit response body to 10 MB to prevent OOM from large responses.
	const maxRespBody = 10 << 20
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBody))
	if err != nil {
		return errorResult(fmt.Sprintf("failed to read response body: %v", err))
	}

	// Combine everything into the response
	result := map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": string(respBody),
		}},
		"status":  resp.StatusCode,
		"headers": resp.Header,
	}
	if resp.StatusCode >= 400 {
		result["isError"] = true
	}

	return result
}
