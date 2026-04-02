package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// panicTool is a mock tool that panics when Handle is called.
type panicTool struct {
	def common.ToolDef
}

func (p panicTool) Definition() common.ToolDef { return p.def }
func (p panicTool) Handle(args map[string]any) any {
	panic("deliberate panic for testing")
}

func TestHandleToolCallPanicRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")
	defer audit.Close()

	registry := tools.NewRegistry()
	toolName := "panic_tool"
	registry.RegisterTool(toolName, panicTool{
		def: common.ToolDef{Name: toolName, InputSchema: map[string]any{}},
	})

	srv := &Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
	}

	params := map[string]any{"name": toolName, "arguments": map[string]any{}}
	raw, _ := json.Marshal(params)

	// This should NOT panic — it should return an error result.
	result := srv.HandleToolCall(context.Background(), raw)

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if isErr, _ := resMap["isError"].(bool); !isErr {
		t.Error("expected isError=true for panicked tool")
	}
	content, _ := resMap["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatal("expected content in error response")
	}
	if !strings.Contains(content[0]["text"].(string), "panicked") {
		t.Errorf("expected panic message, got %q", content[0]["text"])
	}
}

// callerMockTool is a simple tool that returns a fixed response for testing.
type callerMockTool struct {
	def common.ToolDef
	res map[string]any
}

func (m callerMockTool) Definition() common.ToolDef     { return m.def }
func (m callerMockTool) Handle(args map[string]any) any { return m.res }

func TestAuditLogContainsCaller(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")
	defer audit.Close()

	registry := tools.NewRegistry()
	toolName := "caller_test_tool"
	registry.RegisterTool(toolName, callerMockTool{
		def: common.ToolDef{Name: toolName, InputSchema: map[string]any{}},
		res: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
	})

	srv := &Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
	}

	// Call with a named identity.
	ctx := auth.WithCaller(context.Background(), auth.CallerIdentity{Name: "test-agent", Transport: "sse"})
	params := map[string]any{"name": toolName, "arguments": map[string]any{}}
	raw, _ := json.Marshal(params)
	srv.HandleToolCall(ctx, raw)
	audit.Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("failed to unmarshal audit entry: %v", err)
	}

	if entry["caller"] != "test-agent" {
		t.Errorf("expected caller 'test-agent', got %v", entry["caller"])
	}
}
