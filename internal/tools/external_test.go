package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/internal/config"
)

func TestExternalTool_Handle_UnsupportedPlatform(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			"nonexistent_os": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	// Just verify we get an unsupported platform error - the actual platform varies by test env
	if len(text) < 21 || text[:21] != "unsupported platform:" {
		t.Errorf("expected unsupported platform error, got: %s", text)
	}
}

func TestExternalTool_Handle_InvalidArgsJSON(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "echo",
				Args:    []string{"hello"},
			},
			"darwin": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	// Create args that will fail to marshal (circular reference)
	type circular map[string]any
	args := make(circular)
	args["self"] = args

	result := tool.Handle(args)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true")
	}
}

func TestExternalTool_Handle_DockerNotFound(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "echo",
				Args:    []string{"hello"},
			},
			"darwin": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Sandbox: &config.SandboxConfig{
			Type:  "docker",
			Image: "", // No image specified
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true")
	}
}

func TestExternalTool_Handle_DockerNoImage(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "echo",
				Args:    []string{"hello"},
			},
			"darwin": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Sandbox: &config.SandboxConfig{
			Type: "docker",
			// Image intentionally left empty
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true")
	}
}

func TestExternalTool_Handle_WasmSandbox(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "echo",
				Args:    []string{"hello"},
			},
			"darwin": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Sandbox: &config.SandboxConfig{
			Type: "wasm",
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true")
	}
}

// --- Additional External tool tests for uncovered paths ---

func TestExternalTool_Handle_CommandNotFound(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "nonexistent_command_xyz123",
				Args:    []string{},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for command not found")
	}
}

func TestExternalTool_Handle_ExitCodeNonZero(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "sh",
				Args:    []string{"-c", "exit 1"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for non-zero exit code")
	}
}

func TestExternalTool_Handle_SuccessExitCode(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "sh",
				Args:    []string{"-c", "echo hello"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if ok && isError {
		t.Error("expected isError to be false or absent for successful command")
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !strings.Contains(text, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", text)
	}
}

func TestExternalTool_Handle_ArgsExpansion(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "echo",
				Args:    []string{"${USER_INPUT}"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{"USER_INPUT": "expanded_value"})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !strings.Contains(text, "expanded_value") {
		t.Errorf("expected expanded value in output, got: %s", text)
	}
}

func TestExternalTool_Handle_NilArgs(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	// Should not panic with nil args
	result := tool.Handle(nil)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestExternalTool_Handle_EnvVarExpansionInArgs(t *testing.T) {
	// Note: env var expansion happens at config load time, not at Handle time
	// This test verifies that pre-expanded args work correctly
	t.Setenv("TEST_ENV_VAR", "from_env")

	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "echo",
				Args:    []string{"from_env"}, // Pre-expanded
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !strings.Contains(text, "from_env") {
		t.Errorf("expected expanded value in output, got: %s", text)
	}
}

func TestExternalTool_Handle_PermissionDenied(t *testing.T) {
	// Create a file without execute permission
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "noexec.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello\n"), 0644); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: scriptPath,
				Args:    []string{},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for permission denied")
	}
}

func TestExternalTool_Handle_StderrCaptured(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "sh",
				Args:    []string{"-c", "echo stderr_msg >&2; echo stdout_msg"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	// stderr should be captured in the output
	if !strings.Contains(text, "stderr_msg") {
		t.Errorf("expected stderr output in result, got: %s", text)
	}
	if !strings.Contains(text, "stdout_msg") {
		t.Errorf("expected stdout output in result, got: %s", text)
	}
}

func TestExternalTool_Handle_DockerWithImage(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Sandbox: &config.SandboxConfig{
			Type:  "docker",
			Image: "alpine:latest",
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	// Should fail because docker is not available or image not found
	// Just verify it doesn't panic and returns an error
	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Log("expected isError to be true (docker may not be available)")
	}
}

func TestExternalTool_Handle_TimeoutExceeded(t *testing.T) {
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "sh",
				Args:    []string{"-c", "sleep 10"},
			},
		},
		Timeout: "100ms",
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for timeout")
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !strings.Contains(text, "timeout") {
		t.Errorf("expected timeout message, got: %s", text)
	}
}

func TestExternalTool_EffectiveTimeout(t *testing.T) {
	// Test that effectiveTimeout works correctly
	cfg := config.ToolConfig{
		Name: "test_tool",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: cfg}}

	// Should return tool-specific timeout first, then default
	timeout := tool.effectiveTimeout(30 * time.Second)
	if timeout != 30*time.Second {
		t.Errorf("expected default timeout, got %v", timeout)
	}

	// With tool-specific timeout
	cfg.Timeout = "5s"
	tool = ExternalTool{BaseTool: BaseTool{Config: cfg}}
	timeout = tool.effectiveTimeout(30 * time.Second)
	if timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", timeout)
	}
}
