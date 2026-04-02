package tools

import (
	"runtime"
	"strings"
	"testing"

	"github.com/snipspin/jig-mcp/internal/config"
)

func newTerminalTool(allowlist []string, opts ...func(*config.ToolConfig)) TerminalTool {
	tc := config.ToolConfig{
		Name:        "term_test",
		Description: "test",
		InputSchema: map[string]any{"type": "object"},
		Terminal: &config.TerminalConfig{
			Enabled:   true,
			Allowlist: allowlist,
		},
	}
	for _, o := range opts {
		o(&tc)
	}
	return TerminalTool{BaseTool: BaseTool{Config: tc}}
}

func TestTerminalToolMissingCommand(t *testing.T) {
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for missing command")
	}
	content := rm["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "missing mandatory parameter") {
		t.Errorf("unexpected error: %s", content[0]["text"])
	}
}

func TestTerminalToolEmptyCommand(t *testing.T) {
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{"command": ""})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for empty command")
	}
}

func TestTerminalToolAllowlistBlocked(t *testing.T) {
	tool := newTerminalTool([]string{"echo", "ls"})
	resp := tool.Handle(map[string]any{"command": "rm -rf /"})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for blocked command")
	}
	content := rm["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "security check failed") {
		t.Errorf("unexpected error: %s", content[0]["text"])
	}
}

func TestTerminalToolAllowlistExactMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{"command": "echo"})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		content := rm["content"].([]map[string]any)
		t.Errorf("expected success for exact match, got error: %s", content[0]["text"])
	}
}

func TestTerminalToolAllowlistPrefixWithSpace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{"command": "echo hello"})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		content := rm["content"].([]map[string]any)
		t.Errorf("expected success, got error: %s", content[0]["text"])
	}
	content := rm["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if !strings.Contains(text, "hello") {
		t.Errorf("output = %q, expected to contain 'hello'", text)
	}
}

func TestTerminalToolAllowlistRejectsPartialMatch(t *testing.T) {
	// "echo" in allowlist should NOT allow "echomalicious"
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{"command": "echomalicious"})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected echomalicious to be blocked (not a prefix+space match)")
	}
}

func TestTerminalToolEmptyAllowlist(t *testing.T) {
	tool := newTerminalTool([]string{})
	resp := tool.Handle(map[string]any{"command": "echo hi"})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error with empty allowlist")
	}
}

func TestTerminalToolSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	tool := newTerminalTool([]string{"echo"})
	resp := tool.Handle(map[string]any{"command": "echo test_output"})
	rm := resp.(map[string]any)
	if rm["isError"] == true {
		t.Fatal("expected success")
	}
	content := rm["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "test_output") {
		t.Errorf("output missing expected text")
	}
}

func TestTerminalToolOutputTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	tool := newTerminalTool([]string{"dd"}, func(tc *config.ToolConfig) {
		tc.Terminal.MaxOutputSize = 100 // very small limit
	})
	// Generate output larger than 100 bytes
	resp := tool.Handle(map[string]any{"command": "dd if=/dev/zero bs=200 count=1 2>/dev/null | tr '\\0' 'A'"})
	rm := resp.(map[string]any)
	content := rm["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if !strings.Contains(text, "[OUTPUT TRUNCATED]") {
		t.Errorf("expected truncation marker, got %d bytes: %q", len(text), text[:min(len(text), 120)])
	}
}

func TestTerminalToolNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	tool := newTerminalTool([]string{"false"})
	resp := tool.Handle(map[string]any{"command": "false"})
	rm := resp.(map[string]any)
	if rm["isError"] != true {
		t.Error("expected error for non-zero exit")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
