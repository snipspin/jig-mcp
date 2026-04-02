package dashboard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/server"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// resetMetricsStore clears the global metrics store for test isolation.
func resetMetricsStore() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	metricsStore = make(map[string]*toolMetrics)
}

func TestRecordMetric(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	RecordMetric("tool_a", 100)
	RecordMetric("tool_a", 200)
	RecordMetric("tool_b", 50)

	m := getMetrics()

	if m["tool_a"]["count"] != 2 {
		t.Errorf("expected tool_a count=2, got %v", m["tool_a"]["count"])
	}
	if m["tool_a"]["avg_duration_ms"] != 150.0 {
		t.Errorf("expected tool_a avg=150, got %v", m["tool_a"]["avg_duration_ms"])
	}
	if m["tool_b"]["count"] != 1 {
		t.Errorf("expected tool_b count=1, got %v", m["tool_b"]["count"])
	}
}

func TestRecordMetricConcurrent(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	const goroutines = 50
	const callsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				RecordMetric("concurrent_tool", 10)
			}
		}()
	}
	wg.Wait()

	m := getMetrics()
	expected := goroutines * callsPerGoroutine
	if m["concurrent_tool"]["count"] != expected {
		t.Errorf("expected count=%d, got %v", expected, m["concurrent_tool"]["count"])
	}
}

func TestSeedMetricsFromLog(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")

	// Write a fake audit log with known entries.
	logPath := filepath.Join(tmpDir, "audit.jsonl")
	entries := []auditLine{
		{Timestamp: "2026-01-01T00:00:00Z", Tool: "seed_a", DurationMS: 100, Success: true},
		{Timestamp: "2026-01-01T00:00:01Z", Tool: "seed_a", DurationMS: 200, Success: true},
		{Timestamp: "2026-01-01T00:00:02Z", Tool: "seed_b", DurationMS: 50, Success: true},
	}
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n'))
	}
	f.Close()

	SeedMetricsFromLog()

	m := getMetrics()

	if m["seed_a"]["count"] != 2 {
		t.Errorf("expected seed_a count=2, got %v", m["seed_a"]["count"])
	}
	if m["seed_a"]["avg_duration_ms"] != 150.0 {
		t.Errorf("expected seed_a avg=150, got %v", m["seed_a"]["avg_duration_ms"])
	}
	if m["seed_b"]["count"] != 1 {
		t.Errorf("expected seed_b count=1, got %v", m["seed_b"]["count"])
	}
}

func TestSeedMetricsNoFile(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")

	// No audit.jsonl exists — should not panic or error.
	SeedMetricsFromLog()

	m := getMetrics()
	if len(m) != 0 {
		t.Errorf("expected empty metrics, got %v", m)
	}
}

// mockTool is a simple tool that returns a fixed response for testing.
type mockTool struct {
	def common.ToolDef
	res map[string]any
}

func (m mockTool) Definition() common.ToolDef     { return m.def }
func (m mockTool) Handle(args map[string]any) any { return m.res }

func TestHandleToolCallUpdatesMetrics(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")
	defer audit.Close()

	registry := tools.NewRegistry()
	toolName := "metrics_integration_tool"
	registry.RegisterTool(toolName, mockTool{
		def: common.ToolDef{Name: toolName, InputSchema: map[string]any{}},
		res: map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
		},
	})

	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
	}

	params := map[string]any{"name": toolName, "arguments": map[string]any{}}
	raw, _ := json.Marshal(params)

	srv.HandleToolCall(context.Background(), raw)
	srv.HandleToolCall(context.Background(), raw)
	audit.Close()

	// Seed metrics from the audit log that was just written
	SeedMetricsFromLog()

	m := getMetrics()
	if m[toolName]["count"] != 2 {
		t.Errorf("expected count=2 after 2 tool calls, got %v", m[toolName]["count"])
	}

	// Verify the duration is non-negative.
	avg, ok := m[toolName]["avg_duration_ms"].(float64)
	if !ok || avg < 0 {
		t.Errorf("expected non-negative avg_duration_ms, got %v", m[toolName]["avg_duration_ms"])
	}
}

func TestGetMetricsSnapshot(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	RecordMetric("snap_tool", 100)

	// Get a snapshot and then modify the store — snapshot should be independent.
	snap := getMetrics()

	RecordMetric("snap_tool", 300)

	if snap["snap_tool"]["count"] != 1 {
		t.Errorf("snapshot should not reflect later writes, got count=%v", snap["snap_tool"]["count"])
	}
}

// Ensure mockTool satisfies common.Tool for the Handle timing assertion.
type slowMockTool struct {
	def   common.ToolDef
	delay time.Duration
}

func (s slowMockTool) Definition() common.ToolDef { return s.def }
func (s slowMockTool) Handle(args map[string]any) any {
	time.Sleep(s.delay)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
}

func TestMetricsDurationAccuracy(t *testing.T) {
	resetMetricsStore()
	defer resetMetricsStore()

	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")
	defer audit.Close()

	registry := tools.NewRegistry()
	toolName := "slow_tool"
	registry.RegisterTool(toolName, slowMockTool{
		def:   common.ToolDef{Name: toolName, InputSchema: map[string]any{}},
		delay: 50 * time.Millisecond,
	})

	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
	}

	params := map[string]any{"name": toolName, "arguments": map[string]any{}}
	raw, _ := json.Marshal(params)
	srv.HandleToolCall(context.Background(), raw)
	audit.Close()

	// Seed metrics from the audit log that was just written
	SeedMetricsFromLog()

	m := getMetrics()
	avg, ok := m[toolName]["avg_duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected avg_duration_ms to be set, got %v", m[toolName])
	}
	if avg < 40 {
		t.Errorf("expected avg >= 40ms for 50ms sleep tool, got %.1f", avg)
	}
}
