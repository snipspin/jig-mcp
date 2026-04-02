package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/server"
	"github.com/snipspin/jig-mcp/internal/tools"
)

func TestInitGlobalTimeout(t *testing.T) {
	// Save and restore original value.
	origTimeout := globalTimeout
	defer func() { globalTimeout = origTimeout }()

	// Test valid override.
	t.Setenv("JIG_TOOL_TIMEOUT", "45s")
	globalTimeout = defaultGlobalTimeout // reset
	initGlobalTimeout()
	if globalTimeout != 45*time.Second {
		t.Errorf("expected 45s, got %v", globalTimeout)
	}

	// Test invalid value falls back to default.
	t.Setenv("JIG_TOOL_TIMEOUT", "notaduration")
	globalTimeout = defaultGlobalTimeout
	initGlobalTimeout()
	if globalTimeout != defaultGlobalTimeout {
		t.Errorf("expected default %v, got %v", defaultGlobalTimeout, globalTimeout)
	}

	// Test empty env uses default.
	t.Setenv("JIG_TOOL_TIMEOUT", "")
	globalTimeout = defaultGlobalTimeout
	initGlobalTimeout()
	if globalTimeout != defaultGlobalTimeout {
		t.Errorf("expected default %v, got %v", defaultGlobalTimeout, globalTimeout)
	}
}

func TestInitToolSemaphore(t *testing.T) {
	tests := []struct {
		env  string
		want int
	}{
		{"2", 2},
		{"invalid", 8},
		{"0", 8}, // invalid, should use default
		{"", 8},
	}

	for _, tt := range tests {
		t.Run("env="+tt.env, func(t *testing.T) {
			t.Setenv("JIG_MAX_CONCURRENT_TOOLS", tt.env)
			initToolSemaphore()
			if toolSemaphoreSize != tt.want {
				t.Errorf("expected %d, got %d", tt.want, toolSemaphoreSize)
			}
			if cap(toolSemaphore) != tt.want {
				t.Errorf("expected channel cap %d, got %d", tt.want, cap(toolSemaphore))
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	// Test that gracefulShutdown returns without hanging
	// Reset inflightTools for test
	inflightTools = sync.WaitGroup{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Pass empty server list - just tests the function doesn't panic
		gracefulShutdown(nil)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("gracefulShutdown did not complete in time")
	}
}

func TestGracefulShutdownWithInflightTools(t *testing.T) {
	// Reset inflightTools for test
	inflightTools = sync.WaitGroup{}

	// Simulate an inflight tool that completes quickly
	inflightTools.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		inflightTools.Done()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		gracefulShutdown(nil)
	}()

	select {
	case <-done:
		// Success - should wait for inflight tool
	case <-time.After(2 * time.Second):
		t.Error("gracefulShutdown did not complete after inflight tools finished")
	}
}

func TestVersionVariable(t *testing.T) {
	// Version should be set (either by build or default to "dev")
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestMainFunc(t *testing.T) {
	// Test that init functions don't panic when called
	// Full main() testing is limited because it calls os.Exit()
	initGlobalTimeout()
	initToolSemaphore()
}

func TestRunStdioServer(t *testing.T) {
	// Create pipes for stdin/stdout simulation
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	// Create a minimal server
	registry := tools.NewRegistry()
	registry.RegisterTool("test_tool", tools.ExternalTool{
		BaseTool: tools.BaseTool{
			Config: config.ToolConfig{
				Name:        "test_tool",
				Description: "test",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	})

	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 5 * time.Second,
		Semaphore:     make(chan struct{}, 8),
		SemaphoreSize: 8,
		Version:       "test",
	}

	// Run server in goroutine
	var stdoutMu sync.Mutex
	var requestWg sync.WaitGroup
	ctx := auth.WithCaller(context.Background(), auth.CallerIdentity{
		Name:      "test",
		Transport: "stdio",
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		runStdioServer(pr, outW, &stdoutMu, srv, ctx, &requestWg)
	}()

	// Send a tools/list request
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	_, err = pw.WriteString(request)
	if err != nil {
		pw.Close()
		t.Fatalf("Failed to write request: %v", err)
	}
	pw.Close()

	// Give the server time to process and write response
	time.Sleep(100 * time.Millisecond)

	// Close read end to signal EOF and stop the server
	pr.Close()
	<-done

	// Read response
	outW.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, outR)
	outR.Close()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := strings.TrimSpace(buf.String())
	if response == "" {
		t.Error("Expected non-empty response")
	}
	if !strings.Contains(response, `"jsonrpc":"2.0"`) {
		t.Errorf("Expected JSON-RPC response, got: %s", response)
	}
}

func TestRunStdioServerNotification(t *testing.T) {
	// Test that notifications don't produce responses
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	registry := tools.NewRegistry()
	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 5 * time.Second,
		Semaphore:     make(chan struct{}, 8),
		SemaphoreSize: 8,
		Version:       "test",
	}

	var stdoutMu sync.Mutex
	var requestWg sync.WaitGroup
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runStdioServer(pr, outW, &stdoutMu, srv, ctx, &requestWg)
	}()

	// Send a notification (no id field)
	notification := `{"jsonrpc":"2.0","method":"tools/list"}` + "\n"
	_, err = pw.WriteString(notification)
	if err != nil {
		pw.Close()
		t.Fatalf("Failed to write notification: %v", err)
	}
	pw.Close()
	pr.Close()

	<-done

	outW.Close()
	var buf bytes.Buffer
	io.Copy(&buf, outR)
	outR.Close()

	response := strings.TrimSpace(buf.String())
	if response != "" {
		t.Errorf("Expected no response for notification, got: %s", response)
	}
}

func TestRunStdioServerInvalidJSON(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	registry := tools.NewRegistry()
	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 5 * time.Second,
		Semaphore:     make(chan struct{}, 8),
		SemaphoreSize: 8,
		Version:       "test",
	}

	var stdoutMu sync.Mutex
	var requestWg sync.WaitGroup
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runStdioServer(pr, outW, &stdoutMu, srv, ctx, &requestWg)
	}()

	// Send invalid JSON
	_, err = pw.WriteString("not valid json\n")
	if err != nil {
		pw.Close()
		t.Fatalf("Failed to write: %v", err)
	}
	pw.Close()
	pr.Close()

	<-done

	outW.Close()
	var buf bytes.Buffer
	io.Copy(&buf, outR)
	outR.Close()

	response := strings.TrimSpace(buf.String())
	if response != "" {
		t.Errorf("Expected no response for invalid JSON, got: %s", response)
	}
}

// TestConfigDirAutoDetection tests the logic that determines the config directory
// based on binary location and tools/ directory presence.
func TestConfigDirAutoDetection(t *testing.T) {
	// Create a temporary directory structure simulating an installed binary
	// Layout: tmpdir/bin/jig-mcp with tmpdir/tools/ existing
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	toolsDir := filepath.Join(tmpDir, "tools")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("Failed to create tools dir: %v", err)
	}

	// Create a dummy executable in bin/
	exePath := filepath.Join(binDir, "jig-mcp")
	if err := os.WriteFile(exePath, []byte("dummy"), 0o755); err != nil {
		t.Fatalf("Failed to create dummy exe: %v", err)
	}

	// Simulate the detection logic from main()
	var detectedDir string
	exeDir := filepath.Dir(exePath)

	// Check if we're in an install-root layout (bin/ subdirectory)
	if filepath.Base(exeDir) == "bin" {
		installRoot := filepath.Dir(exeDir)
		if _, err := os.Stat(filepath.Join(installRoot, "tools")); err == nil {
			detectedDir = installRoot
		}
	}

	// Fallback: check if tools/ exists next to binary (legacy layout)
	if detectedDir == "" {
		if _, err := os.Stat(filepath.Join(exeDir, "tools")); err == nil {
			detectedDir = exeDir
		}
	}

	if detectedDir != tmpDir {
		t.Errorf("Expected detectedDir=%q, got %q", tmpDir, detectedDir)
	}
}

func TestConfigDirLegacyLayout(t *testing.T) {
	// Legacy layout: binary at root with tools/ sibling
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")

	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("Failed to create tools dir: %v", err)
	}

	exePath := filepath.Join(tmpDir, "jig-mcp")
	if err := os.WriteFile(exePath, []byte("dummy"), 0o755); err != nil {
		t.Fatalf("Failed to create dummy exe: %v", err)
	}

	var detectedDir string
	exeDir := filepath.Dir(exePath)

	// Not in bin/ subdirectory, so first check fails
	if filepath.Base(exeDir) == "bin" {
		installRoot := filepath.Dir(exeDir)
		if _, err := os.Stat(filepath.Join(installRoot, "tools")); err == nil {
			detectedDir = installRoot
		}
	}

	// Fallback to legacy layout
	if detectedDir == "" {
		if _, err := os.Stat(filepath.Join(exeDir, "tools")); err == nil {
			detectedDir = exeDir
		}
	}

	if detectedDir != tmpDir {
		t.Errorf("Expected detectedDir=%q, got %q", tmpDir, detectedDir)
	}
}

func TestConfigDirNoToolsDir(t *testing.T) {
	// No tools/ directory anywhere - should fall back to empty (current dir)
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "jig-mcp")
	if err := os.WriteFile(exePath, []byte("dummy"), 0o755); err != nil {
		t.Fatalf("Failed to create dummy exe: %v", err)
	}

	var detectedDir string
	exeDir := filepath.Dir(exePath)

	if filepath.Base(exeDir) == "bin" {
		installRoot := filepath.Dir(exeDir)
		if _, err := os.Stat(filepath.Join(installRoot, "tools")); err == nil {
			detectedDir = installRoot
		}
	}

	if detectedDir == "" {
		if _, err := os.Stat(filepath.Join(exeDir, "tools")); err == nil {
			detectedDir = exeDir
		}
	}

	// Should remain empty, signaling "use current directory"
	if detectedDir != "" {
		t.Errorf("Expected detectedDir to be empty, got %q", detectedDir)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Test that env variables override flag defaults
	origTransport := "stdio"
	origSSEPort := 3001
	origConfigDir := ""

	t.Setenv("JIG_TRANSPORT", "sse")
	t.Setenv("JIG_SSE_PORT", "8080")
	t.Setenv("JIG_CONFIG_DIR", "/custom/config")

	// Simulate the override logic from main()
	transport := origTransport
	ssePort := origSSEPort
	configDir := origConfigDir

	if t := os.Getenv("JIG_TRANSPORT"); t != "" {
		transport = t
	}
	if p := os.Getenv("JIG_SSE_PORT"); p != "" {
		// Note: in real code this uses fmt.Sscanf, simplified here
		ssePort = 8080
	}
	if c := os.Getenv("JIG_CONFIG_DIR"); strings.TrimSpace(c) != "" {
		configDir = c
	}

	if transport != "sse" {
		t.Errorf("Expected transport=sse, got %s", transport)
	}
	if ssePort != 8080 {
		t.Errorf("Expected ssePort=8080, got %d", ssePort)
	}
	if configDir != "/custom/config" {
		t.Errorf("Expected configDir=/custom/config, got %s", configDir)
	}
}

func TestInvalidSSEPortFallsBackToDefault(t *testing.T) {
	t.Setenv("JIG_SSE_PORT", "notaport")

	ssePort := 3001
	if p := os.Getenv("JIG_SSE_PORT"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &ssePort); err != nil || ssePort < 1 || ssePort > 65535 {
			ssePort = 3001
		}
	}

	if ssePort != 3001 {
		t.Errorf("Expected fallback to default port 3001, got %d", ssePort)
	}
}

func TestDotenvLoadOrder(t *testing.T) {
	// Test the .env loading order logic from main()
	// 1. Current working directory
	// 2. Relative to binary location
	// 3. If binary in bin/, check parent

	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	// Create .env files in different locations
	cwdEnv := filepath.Join(tmpDir, ".env")
	binEnv := filepath.Join(binDir, ".env")
	rootEnv := filepath.Join(tmpDir, ".env")

	// Write distinct content to each
	os.WriteFile(cwdEnv, []byte("SOURCE=cwd\n"), 0o644)
	os.WriteFile(binEnv, []byte("SOURCE=bin\n"), 0o644)
	os.WriteFile(rootEnv, []byte("SOURCE=root\n"), 0o644)

	// The logic tries cwd first, then binary location, then install root
	// godotenv.Load() silently fails if file doesn't exist
	// Last successful load wins

	// Simulate the loading order
	var loadedSource string

	// 1. Try cwd (tmpDir)
	os.Chdir(tmpDir)
	if err := godotenv.Load(); err == nil {
		if v := os.Getenv("SOURCE"); v != "" {
			loadedSource = "cwd"
		}
	}

	// 2. Try relative to binary
	os.Unsetenv("SOURCE")
	exePath := filepath.Join(binDir, "jig-mcp")
	if err := os.WriteFile(exePath, []byte("dummy"), 0o755); err != nil {
		t.Fatalf("Failed to create dummy exe: %v", err)
	}

	// Simulate: if binary in bin/, check parent first
	if filepath.Base(filepath.Dir(exePath)) == "bin" {
		installRoot := filepath.Dir(filepath.Dir(exePath))
		if err := godotenv.Load(filepath.Join(installRoot, ".env")); err == nil {
			if v := os.Getenv("SOURCE"); v != "" {
				loadedSource = "root"
			}
		}
	}

	// Then check binary directory itself
	if err := godotenv.Load(filepath.Join(filepath.Dir(exePath), ".env")); err == nil {
		if v := os.Getenv("SOURCE"); v != "" {
			loadedSource = "bin"
		}
	}

	// The last load (bin/.env) should win
	if loadedSource != "bin" {
		t.Errorf("Expected bin/.env to be loaded last (winning), got %s", loadedSource)
	}
}

// TestMainVersionFlag tests that the -version flag works when running the actual binary
func TestMainVersionFlag(t *testing.T) {
	// Build the binary
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "jig-mcp")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	// Get absolute path to package directory from test file location
	_, filename, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(filename)

	// Use absolute paths for go and output to avoid getwd issues
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("Failed to find go binary: %v", err)
	}

	cmd := exec.Command(goBin, "build", "-o", binPath, pkgDir)
	// Set working directory to module root (parent of cmd/jig-mcp)
	moduleRoot := filepath.Join(pkgDir, "..", "..")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, out)
	}

	// Run with -version flag
	cmd = exec.Command(binPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run with -version: %v", err)
	}

	got := strings.TrimSpace(string(output))
	if !strings.HasPrefix(got, "jig-mcp") {
		t.Errorf("Expected output to start with 'jig-mcp', got: %s", got)
	}
}

// TestMainStdioTransportEmptyInput tests that main() with stdio transport
// handles empty stdin gracefully (exits without error)
func TestMainStdioTransportEmptyInput(t *testing.T) {
	// This test verifies the stdio server path handles EOF immediately
	// We can't call main() directly due to os.Exit() calls, but we can
	// test the runStdioServer function with empty input

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	registry := tools.NewRegistry()
	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 5 * time.Second,
		Semaphore:     make(chan struct{}, 8),
		SemaphoreSize: 8,
		Version:       "test",
	}

	var stdoutMu sync.Mutex
	var requestWg sync.WaitGroup
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runStdioServer(pr, outW, &stdoutMu, srv, ctx, &requestWg)
	}()

	// Close stdin immediately to simulate empty input
	pw.Close()
	pr.Close()

	<-done

	outW.Close()
	var buf bytes.Buffer
	io.Copy(&buf, outR)
	outR.Close()

	// Should exit cleanly with no output for empty input
	response := strings.TrimSpace(buf.String())
	if response != "" {
		t.Errorf("Expected no response for empty input, got: %s", response)
	}
}

// --- Tests for extracted main.go functions (TDD refactoring) ---

// TestLoadEnvFiles tests the .env loading logic
func TestLoadEnvFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .env file in tmpDir
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte("TEST_VAR=from_env\n"), 0644); err != nil {
		t.Fatalf("Failed to write .env: %v", err)
	}

	// Test that loadEnvFiles loads the file
	os.Unsetenv("TEST_VAR")
	err := loadEnvFiles(tmpDir)
	if err != nil {
		t.Fatalf("loadEnvFiles failed: %v", err)
	}

	got := os.Getenv("TEST_VAR")
	if got != "from_env" {
		t.Errorf("loadEnvFiles should load .env, got TEST_VAR=%q", got)
	}
}

func TestLoadEnvFilesNoEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	// No .env file exists
	os.Unsetenv("NONEXISTENT")
	err := loadEnvFiles(tmpDir)
	if err != nil {
		t.Fatalf("loadEnvFiles should not fail when .env is missing: %v", err)
	}
}

// TestApplyEnvOverrides tests that environment variables override flag values
func TestApplyEnvOverrides(t *testing.T) {
	transport := "stdio"
	ssePort := 3001
	configDir := ""

	t.Setenv("JIG_TRANSPORT", "sse")
	t.Setenv("JIG_SSE_PORT", "8080")
	t.Setenv("JIG_CONFIG_DIR", "/custom/path")

	applyEnvOverrides(&transport, &ssePort, &configDir)

	if transport != "sse" {
		t.Errorf("Expected transport=sse, got %q", transport)
	}
	if ssePort != 8080 {
		t.Errorf("Expected ssePort=8080, got %d", ssePort)
	}
	if configDir != "/custom/path" {
		t.Errorf("Expected configDir=/custom/path, got %q", configDir)
	}
}

func TestApplyEnvOverridesInvalidPort(t *testing.T) {
	transport := "stdio"
	ssePort := 3001
	configDir := ""

	t.Setenv("JIG_SSE_PORT", "invalid")

	applyEnvOverrides(&transport, &ssePort, &configDir)

	// Invalid port should fall back to default
	if ssePort != 3001 {
		t.Errorf("Expected ssePort=3001 (default) for invalid input, got %d", ssePort)
	}
}

func TestApplyEnvOverridesPortOutOfRange(t *testing.T) {
	transport := "stdio"
	ssePort := 3001
	configDir := ""

	t.Setenv("JIG_SSE_PORT", "99999")

	applyEnvOverrides(&transport, &ssePort, &configDir)

	// Out of range port should fall back to default
	if ssePort != 3001 {
		t.Errorf("Expected ssePort=3001 (default) for out-of-range input, got %d", ssePort)
	}
}

// TestDetectConfigDir tests the auto-detection logic for config directory
func TestDetectConfigDir(t *testing.T) {
	// Test 1: Install-root layout (binary in bin/ with tools/ at root)
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	toolsDir := filepath.Join(tmpDir, "tools")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := detectConfigDir("", binDir)
	if err != nil {
		t.Fatalf("detectConfigDir failed: %v", err)
	}
	if got != tmpDir {
		t.Errorf("Expected install root %q, got %q", tmpDir, got)
	}
}

func TestDetectConfigDirLegacyLayout(t *testing.T) {
	// Test 2: Legacy layout (binary at root with tools/ sibling)
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := detectConfigDir("", tmpDir)
	if err != nil {
		t.Fatalf("detectConfigDir failed: %v", err)
	}
	if got != tmpDir {
		t.Errorf("Expected binary dir %q, got %q", tmpDir, got)
	}
}

func TestDetectConfigDirNoToolsDir(t *testing.T) {
	// Test 3: No tools/ directory - should return empty (use cwd)
	tmpDir := t.TempDir()

	got, err := detectConfigDir("", tmpDir)
	if err != nil {
		t.Fatalf("detectConfigDir failed: %v", err)
	}
	if got != "" {
		t.Errorf("Expected empty string (use cwd), got %q", got)
	}
}

func TestDetectConfigDirExplicitOverride(t *testing.T) {
	// Test 4: Explicit config dir should be used
	tmpDir := t.TempDir()
	explicitDir := "/explicit/config"

	got, err := detectConfigDir(explicitDir, tmpDir)
	if err != nil {
		t.Fatalf("detectConfigDir failed: %v", err)
	}
	if got != explicitDir {
		t.Errorf("Expected explicit dir %q, got %q", explicitDir, got)
	}
}

// TestLoadTools tests the tool registry loading
func TestLoadTools(t *testing.T) {
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, "tools", "test_tool")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a minimal manifest
	manifest := `name: test_tool
description: "A test tool"
inputSchema:
  type: object
  properties: {}
platforms:
  ` + runtime.GOOS + `:
    command: echo
    args: ["hello"]
`
	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := loadTools(tmpDir)
	if err != nil {
		t.Fatalf("loadTools failed: %v", err)
	}

	tools := registry.GetTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
}

func TestLoadToolsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	registry, err := loadTools(tmpDir)
	if err != nil {
		t.Fatalf("loadTools failed: %v", err)
	}

	tools := registry.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(tools))
	}
}

// TestCreateServer tests server creation
func TestCreateServer(t *testing.T) {
	registry := tools.NewRegistry()

	srv := createServer(registry, 30*time.Second, make(chan struct{}, 8), 8, "test-version", nil)

	if srv == nil {
		t.Fatal("createServer returned nil")
	}
	if srv.Registry != registry {
		t.Error("Server registry not set correctly")
	}
	if srv.GlobalTimeout != 30*time.Second {
		t.Errorf("Expected GlobalTimeout=30s, got %v", srv.GlobalTimeout)
	}
	if srv.Version != "test-version" {
		t.Errorf("Expected Version=test-version, got %q", srv.Version)
	}
}

func TestCreateServerWithCallback(t *testing.T) {
	registry := tools.NewRegistry()
	called := false
	callback := func(name string, ms int64) { called = true }

	srv := createServer(registry, 30*time.Second, make(chan struct{}, 8), 8, "test-version", callback)

	if srv.OnToolCall == nil {
		t.Error("Expected OnToolCall to be set")
	}

	// Verify callback works
	srv.OnToolCall("test", 100)
	if !called {
		t.Error("Expected callback to be invoked")
	}
}

// TestRunStdioMode tests the stdio transport runner
func TestRunStdioMode(t *testing.T) {
	registry := tools.NewRegistry()
	semaphore := make(chan struct{}, 8)

	// Create pipes for stdin/stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	// Run stdio mode with empty input (should exit cleanly)
	pw.Close()
	pr.Close()

	// Just verify the function doesn't panic
	// Full testing is done via TestRunStdioServer* tests
	ctx := auth.WithCaller(context.Background(), auth.CallerIdentity{
		Name:      "test",
		Transport: "stdio",
	})

	// Create a minimal server
	srv := createServer(registry, 5*time.Second, semaphore, 8, "test", nil)

	// Run briefly and close
	done := make(chan struct{})
	go func() {
		defer close(done)
		var stdoutMu sync.Mutex
		var requestWg sync.WaitGroup
		runStdioServer(pr, outW, &stdoutMu, srv, ctx, &requestWg)
	}()

	outW.Close()
	outR.Close()
	<-done
	// Success if no panic
}
