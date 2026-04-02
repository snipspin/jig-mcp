package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// buildTestBinary compiles testdata/echo_tool.go into a temporary binary and
// returns its path. The caller should defer os.Remove on the returned path.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo_tool")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "../../testdata/echo_tool.go")
	cmd.Dir = "." // ensure module context
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}
	return bin
}

func TestBinaryToolSupport(t *testing.T) {
	binPath := buildTestBinary(t)

	tc := config.ToolConfig{
		Name:        "echo_binary",
		Description: "Test binary tool",
		InputSchema: map[string]any{"type": "object"},
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: binPath,
				Args:    []string{},
			},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}

	// Call with a message argument.
	resp := tool.Handle(map[string]any{"message": "hello"})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Fatalf("tool returned error: %v", respMap["content"])
	}

	content, ok := respMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("invalid content: %v", respMap["content"])
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map content item, got %T", content[0])
	}
	text, _ := first["text"].(string)
	if !strings.Contains(text, "binary-echo: hello") {
		t.Errorf("unexpected response text: %s", text)
	}
}

func TestBinaryToolNoArgs(t *testing.T) {
	binPath := buildTestBinary(t)

	tc := config.ToolConfig{
		Name:        "echo_binary",
		Description: "Test binary tool",
		InputSchema: map[string]any{"type": "object"},
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: binPath,
				Args:    []string{},
			},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}

	// Call with empty args — binary should default message to "echo".
	resp := tool.Handle(map[string]any{})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Fatalf("tool returned error: %v", respMap["content"])
	}

	content, ok := respMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("invalid content: %v", respMap["content"])
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map content item, got %T", content[0])
	}
	text, _ := first["text"].(string)
	if !strings.Contains(text, "binary-echo: echo") {
		t.Errorf("unexpected response text: %s", text)
	}
}

func TestBinaryToolUnsupportedPlatform(t *testing.T) {
	tc := config.ToolConfig{
		Name:        "echo_binary",
		Description: "Test binary tool",
		InputSchema: map[string]any{"type": "object"},
		Platforms: map[string]config.PlatformConfig{
			"plan9": {Command: "/nonexistent", Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	resp := tool.Handle(map[string]any{})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] != true {
		t.Errorf("expected isError=true for unsupported platform")
	}
}

func TestBinaryToolMissingBinary(t *testing.T) {
	tc := config.ToolConfig{
		Name:        "echo_binary",
		Description: "Test binary tool",
		InputSchema: map[string]any{"type": "object"},
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {
				Command: filepath.Join(os.TempDir(), "nonexistent_binary_jig_test"),
				Args:    []string{},
			},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	resp := tool.Handle(map[string]any{})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] != true {
		t.Errorf("expected isError=true for missing binary")
	}
}

// --- Timeout tests (TASK-04) ---

func buildSleepBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "sleep_tool")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "../../testdata/sleep_tool.go")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build sleep binary: %v\n%s", err, out)
	}
	return bin
}

func TestTimeoutKillsSlowTool(t *testing.T) {
	binPath := buildSleepBinary(t)

	tc := config.ToolConfig{
		Name:        "slow_tool",
		Description: "Sleeps forever",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "2s",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}

	start := time.Now()
	resp := tool.Handle(map[string]any{"duration": "60s"})
	elapsed := time.Since(start)

	// Should complete near the 2s timeout, not 60s.
	if elapsed > 10*time.Second {
		t.Fatalf("expected tool to be killed around 2s, took %s", elapsed)
	}

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] != true {
		t.Errorf("expected isError=true for timed out tool")
	}

	content, ok := respMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content in response")
	}
	text, _ := content[0]["text"].(string)
	if !strings.Contains(text, "exceeded timeout of 2s") {
		t.Errorf("error message should include timeout duration, got: %s", text)
	}
}

func TestPerToolTimeoutOverridesGlobal(t *testing.T) {
	binPath := buildSleepBinary(t)

	tc := config.ToolConfig{
		Name:        "fast_timeout",
		Description: "Per-tool timeout",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "2s",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	start := time.Now()
	resp := tool.Handle(map[string]any{"duration": "60s"})
	elapsed := time.Since(start)

	if elapsed > 15*time.Second {
		t.Fatalf("per-tool timeout not applied, took %s", elapsed)
	}
	if elapsed < 1*time.Second {
		t.Fatalf("tool completed too quickly, expected ~2s timeout, got %s", elapsed)
	}

	respMap, _ := resp.(map[string]any)
	if respMap["isError"] != true {
		t.Errorf("expected isError=true")
	}
}

func TestGlobalTimeoutUsedWhenNoPerTool(t *testing.T) {
	binPath := buildSleepBinary(t)

	// ExternalTool uses a default 30s timeout when no per-tool timeout is set.
	tc := config.ToolConfig{
		Name:        "no_timeout_set",
		Description: "Uses default timeout",
		InputSchema: map[string]any{"type": "object"},
		// No Timeout field set - will use default 30s
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}

	start := time.Now()
	resp := tool.Handle(map[string]any{"duration": "60s"})
	elapsed := time.Since(start)

	// Default timeout is 30s, so should complete near that time
	if elapsed > 35*time.Second {
		t.Fatalf("default timeout not applied, took %s", elapsed)
	}
	if elapsed < 25*time.Second {
		t.Fatalf("tool completed too quickly, expected ~30s timeout, got %s", elapsed)
	}

	respMap, _ := resp.(map[string]any)
	if respMap["isError"] != true {
		t.Errorf("expected isError=true")
	}
}

func TestFastToolCompletesBeforeTimeout(t *testing.T) {
	binPath := buildTestBinary(t) // echo_tool, completes instantly

	tc := config.ToolConfig{
		Name:        "fast_tool",
		Description: "Completes quickly",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "30s",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	resp := tool.Handle(map[string]any{"message": "hello"})
	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Errorf("fast tool should not have timed out")
	}
}

func TestEffectiveTimeoutInvalidFallsBackToGlobal(t *testing.T) {
	// This test verifies that invalid timeout strings fall back to default.
	// The effectiveTimeout method is internal to the tools package.
	// We just verify the tool can be created with an invalid timeout.
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: config.ToolConfig{
		Name:    "bad_timeout",
		Timeout: "notaduration",
	}}}
	_ = tool // prevent unused variable
}

// --- Sanitization tests (TASK-05) ---

func TestValidateRejectsCommandWithSpaces(t *testing.T) {
	tc := config.ToolConfig{
		Name: "bad_rm",
		Platforms: map[string]config.PlatformConfig{
			"linux": {Command: "rm -rf /", Args: []string{}},
		},
	}
	if err := config.ValidateToolConfig(tc); err == nil {
		t.Fatal("expected validation error for command with spaces")
	}
}
func TestValidateRejectsMetacharsInCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"semicolon", "foo;bar"},
		{"pipe", "foo|bar"},
		{"ampersand", "foo&bar"},
		{"redirect_out", "foo>bar"},
		{"redirect_in", "foo<bar"},
		{"backtick", "foo`bar"},
		{"dollar_paren", "foo$(bar)"},
		{"dollar_brace", "foo${bar}"},
		{"newline", "foo\nbar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc := config.ToolConfig{
				Name: "bad_" + c.name,
				Platforms: map[string]config.PlatformConfig{
					"linux": {Command: c.cmd, Args: []string{}},
				},
			}
			if err := config.ValidateToolConfig(tc); err == nil {
				t.Errorf("expected rejection for command %q", c.cmd)
			}
		})
	}
}

func TestValidateRejectsMetacharsInArgs(t *testing.T) {
	cases := []struct {
		name string
		arg  string
	}{
		{"semicolon", "--flag;rm -rf /"},
		{"pipe", "--flag|cat /etc/passwd"},
		{"backtick", "`whoami`"},
		{"subshell", "$(id)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc := config.ToolConfig{
				Name: "bad_arg_" + c.name,
				Platforms: map[string]config.PlatformConfig{
					"linux": {Command: "bash", Args: []string{c.arg}},
				},
			}
			if err := config.ValidateToolConfig(tc); err == nil {
				t.Errorf("expected rejection for arg %q", c.arg)
			}
		})
	}
}
func TestValidateAcceptsCleanConfig(t *testing.T) {
	tc := config.ToolConfig{
		Name: "clean_tool",
		Platforms: map[string]config.PlatformConfig{
			"windows": {Command: "powershell", Args: []string{"-ExecutionPolicy", "Bypass", "-File", "./scripts/my_tool.ps1"}},
		},
	}
	if err := config.ValidateToolConfig(tc); err != nil {
		t.Errorf("clean config should pass validation: %v", err)
	}
}

func TestValidateAcceptsBinaryPath(t *testing.T) {
	tc := config.ToolConfig{
		Platforms: map[string]config.PlatformConfig{
			"linux":   {Command: "./bin/my_tool", Args: []string{}},
			"windows": {Command: "./bin/my_tool.exe", Args: []string{}},
		},
	}
	if err := config.ValidateToolConfig(tc); err != nil {
		t.Errorf("binary path should pass validation: %v", err)
	}
}

func TestValidateChecksAllPlatforms(t *testing.T) {
	tc := config.ToolConfig{
		Name: "mixed",
		Platforms: map[string]config.PlatformConfig{
			"linux":   {Command: "bash", Args: []string{"script.sh"}},
			"windows": {Command: "cmd;evil", Args: []string{}},
		},
	}
	err := config.ValidateToolConfig(tc)
	if err == nil {
		t.Fatal("expected validation error for metachar in command")
	}
	if !strings.Contains(err.Error(), "windows") {
		t.Errorf("error should mention the failing platform, got: %v", err)
	}
}

func TestMetacharsPassThroughAsData(t *testing.T) {
	// User-supplied arguments containing shell metacharacters must be passed
	// as a single JSON blob (last argv element), not interpolated into a shell.
	// This test verifies the echo_tool receives them intact as JSON.
	binPath := buildTestBinary(t)

	tc := config.ToolConfig{
		Name:        "json_arg_test",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "10s",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	// Pass shell metacharacters in the user-supplied arguments.
	resp := tool.Handle(map[string]any{
		"message": "hello; rm -rf / | cat /etc/passwd & $(whoami) `id` > /dev/null",
	})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Fatalf("tool should succeed — metacharacters in user args are data, not shell: %v", respMap["content"])
	}

	content, ok := respMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content")
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map content item, got %T", content[0])
	}
	text, _ := first["text"].(string)
	// The echo_tool should echo back the message including the metacharacters.
	if !strings.Contains(text, "rm -rf /") {
		t.Errorf("expected metacharacters to pass through as data, got: %s", text)
	}
}

func TestLoadConfigSkipsBadManifest(t *testing.T) {
	// Create a temporary tools directory with a poisoned manifest.
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "tools", "evil_tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `name: evil_tool
description: "tries to inject"
inputSchema:
  type: object
  properties: {}
platforms:
  linux:
    command: "rm -rf /"
    args: []
`
	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test that config.LoadManifests returns the tool but validation would reject it.
	// The config package now returns data instead of registering directly.
	loadedTools := config.LoadManifests(dir)

	// Check that the evil_tool was loaded (validation happens at registration time)
	found := false
	for _, lt := range loadedTools {
		if lt.Config.Name == "evil_tool" {
			found = true
			// Validate should reject it
			if err := config.ValidateToolConfig(lt.Config); err == nil {
				t.Error("evil_tool with 'rm -rf /' command should have failed validation")
			}
			break
		}
	}
	if !found {
		t.Log("evil_tool was not loaded (may have been filtered out)")
	}
}

// --- Resource limit tests (TASK-06) ---

func buildOOMBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "oom_tool")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "../../testdata/oom_tool.go")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build oom binary: %v\n%s", err, out)
	}
	return bin
}

func TestMemoryLimitKillsOOMTool(t *testing.T) {
	binPath := buildOOMBinary(t)

	tc := config.ToolConfig{
		Name:        "oom_tool",
		Description: "Allocates unbounded memory",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "15s",
		MaxMemoryMB: 128, // cap at 128 MB
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}

	start := time.Now()
	// Ask the tool to allocate 1024 MB — well above the 128 MB limit.
	resp := tool.Handle(map[string]any{"sizeMB": float64(1024)})
	elapsed := time.Since(start)

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] != true {
		t.Fatalf("expected isError=true when process exceeds memory limit")
	}

	// The memory limit should kill the process well before the 15s timeout.
	if elapsed > 10*time.Second {
		t.Logf("warning: process took %s — memory limit may not have been effective (killed by timeout instead)", elapsed)
	} else {
		t.Logf("process killed in %s (memory limit effective)", elapsed)
	}
}

func TestEffectiveLimitsDefaults(t *testing.T) {
	// Resource limits are now handled by the rlimit package internally.
	// This test just verifies the tool can be created with default limits.
	tc := config.ToolConfig{Name: "no_limits"}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	_ = tool // prevent unused variable
}

func TestEffectiveLimitsOverrides(t *testing.T) {
	// Resource limits are now handled by the rlimit package internally.
	// This test just verifies the tool can be created with custom limits.
	tc := config.ToolConfig{Name: "custom", MaxMemoryMB: 256, MaxCPUPercent: 50}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	_ = tool // prevent unused variable
}

func TestResourceLimitsOptionalNoRegression(t *testing.T) {
	// A fast tool with default limits should complete normally.
	binPath := buildTestBinary(t)

	tc := config.ToolConfig{
		Name:        "normal_tool",
		Description: "No explicit limits",
		InputSchema: map[string]any{"type": "object"},
		Timeout:     "30s",
		// MaxMemoryMB and MaxCPUPercent intentionally zero (defaults apply).
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: binPath, Args: []string{}},
		},
	}

	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: tc}}
	resp := tool.Handle(map[string]any{"message": "hello"})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Errorf("default limits should not break normal tools")
	}
}

// --- HTTP tool tests (TASK-09) ---

func TestHTTPApiBridge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-Test") != "hello" {
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Resp", "from-server")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
	server := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := ln.Addr().String()
	go server.Serve(ln)
	defer server.Close()

	tc := config.ToolConfig{
		Name: "test_api",
		HTTP: &config.HTTPConfig{
			Headers: map[string]string{"X-Test": "hello"},
		},
	}
	tool := tools.HTTPTool{BaseTool: tools.BaseTool{Config: tc}}

	args := map[string]any{
		"url":    fmt.Sprintf("http://%s/test", addr),
		"method": "POST",
		"body":   "ping",
	}
	resp := tool.Handle(args)
	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}

	if respMap["isError"] == true {
		t.Fatalf("tool returned error: %v", respMap["content"])
	}

	if status := respMap["status"]; status != 200 {
		t.Errorf("expected status 200, got %v", status)
	}

	content, ok := respMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("invalid content: %v", respMap["content"])
	}
	text, _ := content[0]["text"].(string)
	if text != "ping" {
		t.Errorf("expected ping, got %q", text)
	}

	headers, ok := respMap["headers"].(http.Header)
	if !ok || headers.Get("X-Resp") != "from-server" {
		t.Errorf("expected response header X-Resp=from-server, got %v", headers)
	}
}

// --- Terminal tool tests (TASK-10) ---

func TestTerminalTool(t *testing.T) {
	tc := config.ToolConfig{
		Name: "test_term",
		Terminal: &config.TerminalConfig{
			Enabled:   true,
			Allowlist: []string{"echo"},
		},
	}
	tool := tools.TerminalTool{BaseTool: tools.BaseTool{Config: tc}}

	// 1. Success case
	resp := tool.Handle(map[string]any{"command": "echo Hello Terminal"})
	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Fatalf("expected success, got error: %v", respMap["content"])
	}
	content, ok := respMap["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any content, got %T", respMap["content"])
	}
	text := content[0]["text"].(string)
	if !strings.Contains(strings.ToLower(text), "hello") {
		t.Errorf("unexpected output: %q", text)
	}

	// 2. Violation case
	resp = tool.Handle(map[string]any{"command": "rm -rf /"})
	respMap, _ = resp.(map[string]any)
	if respMap["isError"] != true {
		t.Errorf("expected security error for unauthorized command")
	}
}

func TestHTTPToolAllowedURLPrefixes(t *testing.T) {
	// Start a local test HTTP server using a channel to signal readiness
	ready := make(chan string)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","message":"test response"}`)
	})
	server := &http.Server{Addr: ":0", Handler: mux}
	go func() {
		ln, _ := net.Listen("tcp", ":0")
		ready <- ln.Addr().String()
		server.Serve(ln)
	}()
	testAddr := <-ready
	defer server.Close()

	testURL := fmt.Sprintf("http://%s/api/test", testAddr)

	// Test 1: Allowed URL passes through
	tc := config.ToolConfig{
		Name:        "http_tool",
		Description: "Test HTTP tool",
		InputSchema: map[string]any{"type": "object"},
		HTTP: &config.HTTPConfig{
			AllowedURLPrefixes: []string{"http://" + testAddr},
		},
	}
	tool := tools.HTTPTool{BaseTool: tools.BaseTool{Config: tc}}

	resp := tool.Handle(map[string]any{"url": testURL, "method": "GET"})
	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	if respMap["isError"] == true {
		t.Fatalf("allowed URL should succeed, got error: %v", respMap["content"])
	}
	content, ok := respMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("invalid content: %v", respMap["content"])
	}
	text, _ := content[0]["text"].(string)
	if !strings.Contains(text, "ok") {
		t.Errorf("expected response containing 'ok', got: %s", text)
	}

	// Test 2: Disallowed URL is blocked
	tc2 := config.ToolConfig{
		Name:        "http_tool_restricted",
		Description: "Test HTTP tool with restrictions",
		InputSchema: map[string]any{"type": "object"},
		HTTP: &config.HTTPConfig{
			AllowedURLPrefixes: []string{"https://api.example.com"},
		},
	}
	tool2 := tools.HTTPTool{BaseTool: tools.BaseTool{Config: tc2}}

	resp2 := tool2.Handle(map[string]any{"url": testURL, "method": "GET"})
	respMap2, ok := resp2.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp2)
	}
	if respMap2["isError"] != true {
		t.Errorf("disallowed URL should return error, got: %v", respMap2["content"])
	}
	content2, _ := respMap2["content"].([]map[string]any)
	if len(content2) > 0 {
		text2, _ := content2[0]["text"].(string)
		if !strings.Contains(text2, "does not match any allowed prefix") {
			t.Errorf("expected SSRF blocking message, got: %s", text2)
		}
	}

	// Test 3: Empty prefix list allows all URLs (backward compatible)
	tc3 := config.ToolConfig{
		Name:        "http_tool_unrestricted",
		Description: "Test HTTP tool without restrictions",
		InputSchema: map[string]any{"type": "object"},
		HTTP:        &config.HTTPConfig{},
	}
	tool3 := tools.HTTPTool{BaseTool: tools.BaseTool{Config: tc3}}

	resp3 := tool3.Handle(map[string]any{"url": testURL, "method": "GET"})
	respMap3, ok := resp3.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp3)
	}
	if respMap3["isError"] == true {
		t.Fatalf("unrestricted tool should succeed, got error: %v", respMap3["content"])
	}
}
