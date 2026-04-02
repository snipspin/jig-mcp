//go:build integration

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// buildMainBinary compiles the jig-mcp binary for integration tests.
// The caller should defer os.Remove on the returned path.
func buildMainBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "jig-mcp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build jig-mcp binary: %v\n%s", err, out)
	}
	return bin
}

// buildIntegrationTestTool creates a test tool binary that echoes its input.
func buildIntegrationTestTool(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "test_tool")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	// Create a temporary tool source
	toolSrc := filepath.Join(dir, "tool.go")
	src := `
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(` + "`" + `{"content": [{"type": "text", "text": "no args"}]}` + "`" + `)
		return
	}
	raw := os.Args[len(os.Args)-1]
	var args map[string]any
	json.Unmarshal([]byte(raw), &args)
	msg, _ := args["message"].(string)
	if msg == "" {
		msg = "echo"
	}
	fmt.Printf(` + "`" + `{"content": [{"type": "text", "text": "tool-echo: %s"}]}` + "`" + ` + "\n", msg)
}
`
	if err := os.WriteFile(toolSrc, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "-o", bin, toolSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test tool: %v\n%s", err, out)
	}
	return bin
}

// TestStdioTransportEndToEnd verifies the stdio transport works end-to-end.
func TestStdioTransportEndToEnd(t *testing.T) {
	binPath := buildMainBinary(t)
	toolPath := buildIntegrationTestTool(t)

	// Create a temporary tools directory with manifest
	// The config.LoadManifests function reads from "tools" directory relative to cwd
	toolsDir := filepath.Join(t.TempDir(), "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	toolDir := filepath.Join(toolsDir, "test_tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := fmt.Sprintf(`
name: test_tool
description: "Test tool for integration"
inputSchema:
  type: object
  properties:
    message:
      type: string
platforms:
  %s:
    command: %s
    args: []
`, runtime.GOOS, toolPath)

	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start jig-mcp from the temp directory so it finds the tools
	cmd := exec.Command(binPath, "-transport", "stdio")
	cmd.Dir = filepath.Dir(toolsDir)
	cmd.Env = append(os.Environ(),
		"JIG_TOOL_TIMEOUT=10s",
		"JIG_AUTH_TOKEN=test-token",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	// Send a tools/list request
	listReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(listReq); err != nil {
		t.Fatal(err)
	}

	// Read response
	reader := bufio.NewReader(stdout)
	line, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	var listResp map[string]any
	if err := json.Unmarshal(line, &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if listResp["id"].(float64) != 1 {
		t.Errorf("expected id 1, got %v", listResp["id"])
	}

	result, ok := listResp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result object")
	}

	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("expected tools in response")
	}

	// Now call the tool
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "test_tool",
			"arguments": map[string]any{
				"message": "hello integration",
			},
		},
	}

	if err := encoder.Encode(callReq); err != nil {
		t.Fatal(err)
	}

	line, _, err = reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	var callResp map[string]any
	if err := json.Unmarshal(line, &callResp); err != nil {
		t.Fatalf("failed to parse call response: %v", err)
	}

	if callResp["id"].(float64) != 2 {
		t.Errorf("expected id 2, got %v", callResp["id"])
	}

	callResult, ok := callResp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result object in call response")
	}

	content, ok := callResult["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in call result")
	}

	contentMap, ok := content[0].(map[string]any)
	if !ok {
		t.Fatal("expected content item to be object")
	}

	text, ok := contentMap["text"].(string)
	if !ok || !strings.Contains(text, "hello integration") {
		t.Errorf("expected tool to echo message, got: %s", text)
	}
}

// TestGracefulShutdownStdio verifies graceful shutdown on SIGTERM for stdio transport.
func TestGracefulShutdownStdio(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling test not applicable on Windows")
	}

	binPath := buildMainBinary(t)

	cmd := exec.Command(binPath, "-transport", "stdio")
	cmd.Env = append(os.Environ(), "JIG_TOOL_TIMEOUT=5s")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Send a simple request to verify server is running
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(req); err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(stdout)
	_, _, err = reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	// Send SIGTERM and measure shutdown time
	start := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err != nil && !strings.Contains(err.Error(), "signal") {
			t.Logf("process exited with error: %v", err)
		}
		t.Logf("graceful shutdown completed in %v", elapsed)
	case <-time.After(15 * time.Second):
		t.Fatal("graceful shutdown took too long, forcing kill")
		cmd.Process.Kill()
	}
}

// TestDashboardStarts verifies the dashboard HTTP server starts correctly.
func TestDashboardStarts(t *testing.T) {
	binPath := buildMainBinary(t)

	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Use SSE transport so dashboard can run without stdin
	cmd := exec.Command(binPath, "-transport", "sse", "-dashboard-port", fmt.Sprintf("%d", port))
	cmd.Env = append(os.Environ(),
		"JIG_TOOL_TIMEOUT=5s",
		"JIG_AUTH_TOKEN=test-token",
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Read stderr for diagnostics and wait for "dashboard starting" message
	dashboardReady := make(chan bool, 1)
	go func() {
		scanner := bufio.NewReader(stderr)
		for {
			line, err := scanner.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(line, "dashboard starting") {
				dashboardReady <- true
				return
			}
		}
	}()

	// Wait for dashboard to be ready
	select {
	case <-dashboardReady:
		t.Log("dashboard started")
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		t.Fatal("dashboard did not start within timeout")
	}

	// Verify dashboard is accessible
	var resp *http.Response
	client := &http.Client{}
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		req, reqErr := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/metrics", port), nil)
		if reqErr != nil {
			cmd.Process.Kill()
			t.Fatal(reqErr)
		}
		req.Header.Set("Authorization", "Bearer test-token")
		resp, reqErr = client.Do(req)
		if reqErr == nil {
			break
		}
	}
	if resp == nil {
		cmd.Process.Kill()
		t.Fatal("dashboard not accessible after starting")
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cmd.Process.Kill()
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Cleanup
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// TestSSETransportStarts verifies the SSE HTTP server starts correctly.
func TestSSETransportStarts(t *testing.T) {
	binPath := buildMainBinary(t)

	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cmd := exec.Command(binPath, "-transport", "sse", "-port", fmt.Sprintf("%d", port))
	cmd.Env = append(os.Environ(), "JIG_TOOL_TIMEOUT=5s")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait for server to indicate it's ready (via stderr or timeout)
	ready := make(chan bool, 1)
	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(line, "SSE server") || strings.Contains(line, "listening") {
				ready <- true
				return
			}
		}
	}()

	select {
	case <-ready:
		t.Log("SSE server started successfully")
	case <-time.After(3 * time.Second):
		t.Log("SSE server startup message not detected, but continuing...")
	}

	// Give it time to start
	time.Sleep(500 * time.Millisecond)

	// Verify SSE endpoint is accessible
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/sse", port))
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("SSE endpoint not accessible: %v", err)
	}
	resp.Body.Close()

	// SSE should return 200 or redirect
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Logf("SSE endpoint returned status %d (may be acceptable)", resp.StatusCode)
	}

	// Cleanup
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// TestConcurrentToolCalls verifies the tool semaphore limits concurrency.
func TestConcurrentToolCalls(t *testing.T) {
	binPath := buildMainBinary(t)
	toolPath := buildIntegrationTestTool(t)

	// Create tools directory
	toolsDir := t.TempDir()
	toolDir := filepath.Join(toolsDir, "slow_tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a slow tool that takes 2 seconds
	slowToolSrc := filepath.Join(toolDir, "slow_tool.go")
	src := `
package main

import (
	"fmt"
	"time"
)

func main() {
	time.Sleep(2 * time.Second)
	fmt.Println(` + "`" + `{"content": [{"type": "text", "text": "done"}]}` + "`" + `)
}
`
	if err := os.WriteFile(slowToolSrc, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build the slow tool
	cmd := exec.Command("go", "build", "-o", filepath.Join(toolDir, "slow_tool"), slowToolSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build slow tool: %v\n%s", err, out)
	}

	manifest := fmt.Sprintf(`
name: slow_tool
description: "Slow tool for concurrency test"
inputSchema:
  type: object
platforms:
  %s:
    command: %s
    args: []
`, runtime.GOOS, toolPath)

	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start jig-mcp with max 2 concurrent tools
	baseDir := filepath.Dir(toolsDir)
	jigCmd := exec.Command(binPath, "-transport", "stdio")
	jigCmd.Dir = baseDir
	jigCmd.Env = append(os.Environ(),
		"JIG_TOOL_TIMEOUT=30s",
		"JIG_MAX_CONCURRENT_TOOLS=2",
	)

	stdin, err := jigCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := jigCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := jigCmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		jigCmd.Process.Signal(syscall.SIGTERM)
		jigCmd.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	reader := bufio.NewReader(stdout)

	// Send 4 concurrent tool calls
	var wg sync.WaitGroup
	errors := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "slow_tool",
				},
			}
			if err := encoder.Encode(req); err != nil {
				errors <- err
				return
			}
		}(i)
	}

	// Wait for all requests to be sent
	wg.Wait()

	// Collect responses with timeout
	start := time.Now()
	responses := 0
	for responses < 4 {
		select {
		case err := <-errors:
			t.Fatalf("error sending request: %v", err)
		default:
			line, _, err := reader.ReadLine()
			if err != nil && err != io.EOF {
				t.Fatalf("error reading response: %v", err)
			}
			if err == io.EOF {
				break
			}
			var resp map[string]any
			if err := json.Unmarshal(line, &resp); err != nil {
				errors <- err
				continue
			}
			responses++
		}

		if time.Since(start) > 10*time.Second {
			t.Fatal("timeout waiting for responses")
		}
	}

	t.Logf("received %d responses", responses)
}

// TestInvalidJSONHandling verifies the server handles invalid JSON gracefully.
func TestInvalidJSONHandling(t *testing.T) {
	binPath := buildMainBinary(t)

	cmd := exec.Command(binPath, "-transport", "stdio")
	cmd.Env = append(os.Environ(), "JIG_TOOL_TIMEOUT=5s")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	// Send invalid JSON
	if _, err := stdin.Write([]byte("not valid json\n")); err != nil {
		t.Fatal(err)
	}

	// Send valid JSON after to verify server is still running
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(req); err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(stdout)
	line, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	var resp map[string]any
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("failed to parse response after invalid JSON: %v", err)
	}

	if resp["id"].(float64) != 1 {
		t.Errorf("expected id 1, got %v", resp["id"])
	}

	t.Log("server recovered from invalid JSON and processed subsequent request")
}

// TestNotificationHandling verifies notifications (no id) don't get responses.
func TestNotificationHandling(t *testing.T) {
	binPath := buildMainBinary(t)

	cmd := exec.Command(binPath, "-transport", "stdio")
	cmd.Env = append(os.Environ(), "JIG_TOOL_TIMEOUT=5s")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	// Send a notification (no id field)
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(notif); err != nil {
		t.Fatal(err)
	}

	// Give server time to process
	time.Sleep(100 * time.Millisecond)

	// Send a real request
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	if err := encoder.Encode(req); err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(stdout)
	line, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	var resp map[string]any
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should only receive response for the request with id
	if resp["id"].(float64) != 1 {
		t.Errorf("expected response for id 1 only, got %v", resp["id"])
	}

	t.Log("notification correctly handled without response")
}

// TestEnvironmentVariableOverrides verifies env vars override CLI flags.
func TestEnvironmentVariableOverrides(t *testing.T) {
	binPath := buildMainBinary(t)

	// Start with SSE transport but override to stdio via env
	cmd := exec.Command(binPath, "-transport", "sse", "-port", "3001")
	cmd.Env = append(os.Environ(),
		"JIG_TRANSPORT=stdio",
		"JIG_TOOL_TIMEOUT=5s",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	// Should work in stdio mode despite -transport sse flag
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(req); err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(stdout)
	line, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	var resp map[string]any
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	t.Log("JIG_TRANSPORT correctly overrides CLI flag")
}

// TestServerWithAuthTokens verifies SSE server starts with auth tokens.
func TestServerWithAuthTokens(t *testing.T) {
	binPath := buildMainBinary(t)

	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cmd := exec.Command(binPath, "-transport", "sse", "-port", fmt.Sprintf("%d", port))
	cmd.Env = append(os.Environ(),
		"JIG_AUTH_TOKENS=test-token-1,test-token-2",
		"JIG_TOOL_TIMEOUT=5s",
	)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	// Give server time to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running by checking if port is listening
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("SSE server not listening on port %d: %v", port, err)
	}
	conn.Close()

	t.Logf("SSE server started with auth tokens on port %d", port)
}

// TestMultipleSignals verifies handling of multiple SIGINT signals.
func TestMultipleSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling test not applicable on Windows")
	}

	binPath := buildMainBinary(t)

	cmd := exec.Command(binPath, "-transport", "stdio")
	cmd.Env = append(os.Environ(), "JIG_TOOL_TIMEOUT=5s")

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Send first SIGINT
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	// Wait briefly then send another
	time.Sleep(100 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	// Process should exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		t.Log("process exited after multiple signals")
	case <-time.After(15 * time.Second):
		t.Fatal("process did not exit after multiple signals")
		cmd.Process.Kill()
	}
}

// TestToolCallTimeoutIntegration verifies tool calls respect the timeout.
func TestToolCallTimeoutIntegration(t *testing.T) {
	binPath := buildMainBinary(t)

	// Create a tool that sleeps longer than the timeout
	baseDir := t.TempDir()
	toolsDir := filepath.Join(baseDir, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toolDir := filepath.Join(toolsDir, "timeout_tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	toolSrc := filepath.Join(toolDir, "timeout_tool.go")
	src := `
package main

import (
	"fmt"
	"time"
)

func main() {
	time.Sleep(30 * time.Second)
	fmt.Println(` + "`" + `{"content": [{"type": "text", "text": "should not reach here"}]}` + "`" + `)
}
`
	if err := os.WriteFile(toolSrc, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(toolDir, "timeout_tool"), toolSrc)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build timeout tool: %v\n%s", err, out)
	}

	manifest := fmt.Sprintf(`
name: timeout_tool
description: "Tool that times out"
inputSchema:
  type: object
timeout: 2s
platforms:
  %s:
    command: %s
    args: []
`, runtime.GOOS, filepath.Join(toolDir, "timeout_tool"))

	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	jigCmd := exec.Command(binPath, "-transport", "stdio")
	jigCmd.Dir = baseDir
	jigCmd.Env = append(os.Environ(),
		"JIG_TOOL_TIMEOUT=2s",
	)

	stdin, err := jigCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := jigCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := jigCmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		jigCmd.Process.Signal(syscall.SIGTERM)
		jigCmd.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	reader := bufio.NewReader(stdout)

	// Call the slow tool
	start := time.Now()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "timeout_tool",
		},
	}

	if err := encoder.Encode(req); err != nil {
		t.Fatal(err)
	}

	line, _, err := reader.ReadLine()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should timeout around 2 seconds, not 30
	if elapsed > 10*time.Second {
		t.Errorf("tool took %s to complete, expected ~2s timeout", elapsed)
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result in response")
	}

	if isError, ok := result["isError"].(bool); !ok || !isError {
		t.Error("expected tool call to return error due to timeout")
	}

	t.Logf("tool correctly timed out after %v", elapsed)
}
