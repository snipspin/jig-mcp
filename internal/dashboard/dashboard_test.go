package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/tools"
)

func TestDashboardAPITools(t *testing.T) {
	registry := tools.NewRegistry()

	registry.RegisterTool("mock1", tools.ExternalTool{BaseTool: tools.BaseTool{Config: config.ToolConfig{Name: "mock1", Description: "desc1"}}})
	registry.RegisterTool("mock2", tools.ExternalTool{BaseTool: tools.BaseTool{Config: config.ToolConfig{Name: "mock2", Description: "desc2"}}})

	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/api/tools", nil)
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// activeTools is a map, so order in response might vary, but length should be 2
	if len(resp) != 2 {
		t.Errorf("expected 2 tools, got %d", len(resp))
	}
}

func TestDashboardAPIMetrics(t *testing.T) {
	// Reset metricsStore after test
	metricsMu.Lock()
	oldMetrics := metricsStore
	metricsStore = make(map[string]*toolMetrics)
	metricsMu.Unlock()

	defer func() {
		metricsMu.Lock()
		metricsStore = oldMetrics
		metricsMu.Unlock()
	}()

	RecordMetric("test_tool", 100)
	RecordMetric("test_tool", 200)

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/api/metrics", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	tools, ok := resp["tools"].(map[string]any)
	if !ok {
		t.Fatal("expected 'tools' key in metrics response")
	}

	m, ok := tools["test_tool"].(map[string]any)
	if !ok {
		t.Fatal("expected test_tool in metrics")
	}

	if m["count"].(float64) != 2 {
		t.Errorf("expected count 2, got %v", m["count"])
	}

	concurrency, ok := resp["concurrency"].(map[string]any)
	if !ok {
		t.Fatal("expected 'concurrency' key in metrics response")
	}
	if _, ok := concurrency["max_concurrent_tools"]; !ok {
		t.Error("expected max_concurrent_tools in concurrency")
	}
}

func TestDashboardAPILogs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jig-logs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldLogDir := os.Getenv("JIG_LOG_DIR")
	os.Setenv("JIG_LOG_DIR", tempDir)
	defer os.Setenv("JIG_LOG_DIR", oldLogDir)

	line := auditLine{
		Timestamp:  "2026-03-31T06:00:00Z",
		Tool:       "test-logger",
		DurationMS: 123,
		Success:    true,
	}
	data, _ := json.Marshal(line)
	_ = os.WriteFile(filepath.Join(tempDir, "audit.jsonl"), append(data, '\n'), 0644)

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []auditLine
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) == 0 || resp[0].Tool != "test-logger" {
		t.Errorf("expected log for test-logger, got %+v", resp)
	}
}

func TestDashboardHTMLServed(t *testing.T) {
	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("expected Content-Type text/html, got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<title>Jig-MCP Dashboard</title>") {
		t.Error("body does not contain expected title")
	}
}

func TestDashboardAuthRequired(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKEN", "dashboard-token")
	defer os.Unsetenv("JIG_AUTH_TOKEN")
	// Clear any existing tokens and reinitialize
	auth.GlobalTokens().Clear()
	auth.InitTokenRegistry()
	defer func() {
		os.Unsetenv("JIG_AUTH_TOKEN")
		auth.GlobalTokens().Clear()
	}()

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	// With correct token
	req = httptest.NewRequest("GET", "/?token=dashboard-token", nil)
	w = httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with valid token, got %d", w.Code)
	}
}

func TestDashboardAuthHeader(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKEN", "bearer-token")
	defer os.Unsetenv("JIG_AUTH_TOKEN")
	auth.GlobalTokens().Clear()
	auth.InitTokenRegistry()
	defer func() {
		os.Unsetenv("JIG_AUTH_TOKEN")
		auth.GlobalTokens().Clear()
	}()

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bearer-token")
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with valid bearer token, got %d", w.Code)
	}
}

func TestDashboardLogsEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jig-logs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldLogDir := os.Getenv("JIG_LOG_DIR")
	os.Setenv("JIG_LOG_DIR", tempDir)
	defer os.Setenv("JIG_LOG_DIR", oldLogDir)

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []auditLine
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected empty logs, got %d entries", len(resp))
	}
}

func TestDashboardMetricsEmpty(t *testing.T) {
	metricsMu.Lock()
	oldMetrics := metricsStore
	metricsStore = make(map[string]*toolMetrics)
	metricsMu.Unlock()
	defer func() {
		metricsMu.Lock()
		metricsStore = oldMetrics
		metricsMu.Unlock()
	}()

	registry := tools.NewRegistry()
	server := StartDashboard(0, registry, nil, 0)
	defer server.Close()

	req := httptest.NewRequest("GET", "/api/metrics", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	tools, ok := resp["tools"].(map[string]any)
	if !ok {
		t.Fatal("expected 'tools' key in metrics response")
	}

	if len(tools) != 0 {
		t.Errorf("expected empty metrics, got %d entries", len(tools))
	}
}

func TestGetAuditLogsFileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jig-logs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldLogDir := os.Getenv("JIG_LOG_DIR")
	os.Setenv("JIG_LOG_DIR", tempDir)
	defer os.Setenv("JIG_LOG_DIR", oldLogDir)

	logs, err := getAuditLogs(100)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(logs))
	}
}

func TestGetAuditLogsPartialLine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jig-logs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldLogDir := os.Getenv("JIG_LOG_DIR")
	os.Setenv("JIG_LOG_DIR", tempDir)
	defer os.Setenv("JIG_LOG_DIR", oldLogDir)

	// Write a file with a partial first line (simulating seek mid-line)
	data := []byte("partial\n{\"timestamp\":\"2026-03-31T06:00:00Z\",\"tool\":\"test\",\"duration_ms\":100,\"success\":true}\n")
	os.WriteFile(filepath.Join(tempDir, "audit.jsonl"), data, 0644)

	logs, err := getAuditLogs(100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	}
}
