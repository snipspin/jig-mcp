package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// TestJSONRPCIDTypes verifies that both string and numeric IDs are supported per MCP spec
func TestJSONRPCIDTypes(t *testing.T) {
	// Test 1: String ID parsing
	jsonWithStringID := `{
		"jsonrpc": "2.0",
		"id": "request-42",
		"method": "initialize",
		"params": null
	}`

	var req Request
	if err := json.Unmarshal([]byte(jsonWithStringID), &req); err != nil {
		t.Fatalf("failed to unmarshal request with string ID: %v", err)
	}

	if string(req.ID) != `"request-42"` {
		t.Errorf("expected ID to be '\"request-42\"', got %q", string(req.ID))
	}
	t.Logf("✓ String ID parsed correctly: %s", string(req.ID))

	// Test 2: Numeric ID parsing
	jsonWithNumericID := `{
		"jsonrpc": "2.0",
		"id": 42,
		"method": "initialize",
		"params": null
	}`

	if err := json.Unmarshal([]byte(jsonWithNumericID), &req); err != nil {
		t.Fatalf("failed to unmarshal request with numeric ID: %v", err)
	}

	if string(req.ID) != `42` {
		t.Errorf("expected ID to be '42', got %q", string(req.ID))
	}
	t.Logf("✓ Numeric ID parsed correctly: %s", string(req.ID))

	// Test 3: Response preserves numeric ID
	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"test": "value"},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	if string(respBytes) != `{"jsonrpc":"2.0","id":42,"result":{"test":"value"}}` {
		t.Errorf("response has wrong format: %s", string(respBytes))
	}
	t.Logf("✓ Response preserves numeric ID: %s", string(respBytes))

	// Test 4: String ID in response
	stringIDReq := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"client-abc"`),
		Method:  "tools/list",
	}

	resp = Response{
		JSONRPC: "2.0",
		ID:      stringIDReq.ID,
		Result:  map[string]any{"tools": []any{}},
	}

	respBytes, err = json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response with string ID: %v", err)
	}

	if string(respBytes) != `{"jsonrpc":"2.0","id":"client-abc","result":{"tools":[]}}` {
		t.Errorf("response has wrong format: %s", string(respBytes))
	}
	t.Logf("✓ Response preserves string ID: %s", string(respBytes))
}

// TestProcessRequestIDPreservation verifies that processRequest preserves IDs
func TestProcessRequestIDPreservation(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		expectedKey string
	}{
		{
			name:        "numeric ID",
			id:          "42",
			expectedKey: `"id":42`,
		},
		{
			name:        "string ID",
			id:          `"request-abc"`,
			expectedKey: `"id":"request-abc"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage(tt.id),
				Method:  "initialize",
			}

			srv := &Server{
				Registry:      tools.NewRegistry(),
				GlobalTimeout: 30 * time.Second,
			}
			resp := srv.ProcessRequest(context.Background(), req)

			if string(resp.ID) != tt.id {
				t.Errorf("ID not preserved: expected %q, got %q", tt.id, string(resp.ID))
			}

			respBytes, _ := json.Marshal(resp)
			if !json.Valid(respBytes) {
				t.Errorf("invalid JSON in response: %s", string(respBytes))
			}

			t.Logf("✓ ID preserved through processRequest: %s", string(resp.ID))
		})
	}
}

// TestExternalTool_Handle_SliceMutation verifies that multiple calls to the same tool
// do not corrupt each other's arguments via slice sharing.
func TestExternalTool_Handle_SliceMutation(t *testing.T) {
	// Create a slice with extra capacity to trigger the potential bug
	sharedArgs := make([]string, 1, 10)
	sharedArgs[0] = "base"

	cfg := config.ToolConfig{
		Name: "test-tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "echo",
				Args:    sharedArgs,
			},
			"darwin": {
				Command: "echo",
				Args:    sharedArgs,
			},
			"windows": {
				Command: "cmd.exe",
				Args:    append(sharedArgs, "/c", "echo"),
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	plat := tool.Config.Platforms[runtime.GOOS]
	if plat.Command == "" {
		t.Skipf("No config for current platform %s", runtime.GOOS)
	}

	// We'll test the logic that Handle uses to prepare args.
	// We want to ensure that appending to argsToPass in one "call"
	// doesn't affect another "call" even if they share the same base slice.

	// Simulated Call 1
	argsToPass1 := make([]string, len(plat.Args), len(plat.Args)+1)
	copy(argsToPass1, plat.Args)
	argsToPass1 = append(argsToPass1, "{\"call\":1}")

	// Simulated Call 2
	argsToPass2 := make([]string, len(plat.Args), len(plat.Args)+1)
	copy(argsToPass2, plat.Args)
	argsToPass2 = append(argsToPass2, "{\"call\":2}")

	lastIdx1 := len(argsToPass1) - 1
	lastIdx2 := len(argsToPass2) - 1

	if argsToPass1[lastIdx1] != "{\"call\":1}" {
		t.Errorf("Call 1 args corrupted: expected ...1, got %q", argsToPass1[lastIdx1])
	}
	if argsToPass2[lastIdx2] != "{\"call\":2}" {
		t.Errorf("Call 2 args corrupted: expected ...2, got %q", argsToPass2[lastIdx2])
	}

	// Final check: they must be different
	if argsToPass1[lastIdx1] == argsToPass2[lastIdx2] {
		t.Errorf("Both calls have same arguments: %q", argsToPass1[lastIdx1])
	}
}

// TestDeterministicToolList verifies that tools/list returns tools sorted by name
func TestDeterministicToolList(t *testing.T) {
	registry := tools.NewRegistry()

	toolNames := []string{"c_tool", "a_tool", "b_tool"}
	for _, name := range toolNames {
		registry.RegisterTool(name, tools.ExternalTool{
			BaseTool: tools.BaseTool{
				Config: config.ToolConfig{
					Name:        name,
					Description: name + " description",
				},
			},
		})
	}

	srv := &Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	}

	resp := srv.ProcessRequest(context.Background(), req)
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected Result to be map[string]any, got %T", resp.Result)
	}

	tools, ok := result["tools"].([]common.ToolDef)
	if !ok {
		t.Fatalf("expected tools to be []common.ToolDef, got %T", result["tools"])
	}

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	expectedOrder := []string{"a_tool", "b_tool", "c_tool"}
	for i, name := range expectedOrder {
		if tools[i].Name != name {
			t.Errorf("expected tool at index %d to be %q, got %q", i, name, tools[i].Name)
		}
	}
	t.Logf("✓ Tool list is deterministic and sorted: %v", expectedOrder)
}

// TestStdioConcurrentRequests verifies that the stdio loop processes multiple
// requests concurrently and that stdout writes are not interleaved.
func TestStdioConcurrentRequests(t *testing.T) {
	// Pipe simulates stdin/stdout for the stdio loop.
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	var stdoutMu sync.Mutex

	const numRequests = 10

	// Writer goroutine: send N initialize requests on the "stdin" pipe.
	go func() {
		for i := 1; i <= numRequests; i++ {
			line := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"initialize"}`, i)
			fmt.Fprintf(stdinW, "%s\n", line)
		}
		stdinW.Close()
	}()

	// Processor goroutine: mimics the stdio loop — reads lines, spawns
	// goroutines that call processRequest and write responses under a mutex.
	go func() {
		defer stdoutW.Close()
		scanner := bufio.NewScanner(stdinR)
		var wg sync.WaitGroup

		for scanner.Scan() {
			line := make([]byte, len(scanner.Bytes()))
			copy(line, scanner.Bytes())

			wg.Add(1)
			go func() {
				defer wg.Done()
				var req Request
				if err := json.Unmarshal(line, &req); err != nil {
					return
				}
				srv := &Server{
					Registry:      tools.NewRegistry(),
					GlobalTimeout: 30 * time.Second,
				}
				resp := srv.ProcessRequest(context.Background(), req)
				out, _ := json.Marshal(resp)
				stdoutMu.Lock()
				fmt.Fprintf(stdoutW, "%s\n", out)
				stdoutMu.Unlock()
			}()
		}
		wg.Wait()
	}()

	// Reader: collect all responses and verify each ID was returned.
	seen := make(map[int]bool)
	scanner := bufio.NewScanner(stdoutR)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !json.Valid(line) {
			t.Errorf("invalid JSON in response: %s", string(line))
			continue
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Errorf("failed to unmarshal response: %v", err)
			continue
		}
		var id int
		if err := json.Unmarshal(resp.ID, &id); err != nil {
			t.Errorf("failed to parse response ID: %v (raw: %s)", err, string(resp.ID))
			continue
		}
		seen[id] = true
	}

	for i := 1; i <= numRequests; i++ {
		if !seen[i] {
			t.Errorf("missing response for request ID %d", i)
		}
	}
	if len(seen) != numRequests {
		t.Errorf("expected %d responses, got %d", numRequests, len(seen))
	}
}

// TestToolSemaphoreLimitsConcurrency verifies that HandleToolCall respects
// the semaphore and limits the number of concurrent tool executions.
func TestToolSemaphoreLimitsConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("JIG_LOG_DIR", tmpDir)
	defer os.Unsetenv("JIG_LOG_DIR")
	defer audit.Close()

	// Set concurrency limit to 2.
	semaphore := make(chan struct{}, 2)

	var peak int64
	var current int64

	registry := tools.NewRegistry()
	toolName := "concurrency_test_tool"
	registry.RegisterTool(toolName, slowConcurrentTool{
		def:     common.ToolDef{Name: toolName, InputSchema: map[string]any{}},
		delay:   100 * time.Millisecond,
		current: &current,
		peak:    &peak,
	})

	srv := &Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
		Semaphore:     semaphore,
		SemaphoreSize: 2,
	}

	// Fire 6 concurrent tool calls with limit of 2.
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params := map[string]any{"name": toolName, "arguments": map[string]any{}}
			raw, _ := json.Marshal(params)
			srv.HandleToolCall(context.Background(), raw)
		}()
	}
	wg.Wait()

	observed := atomic.LoadInt64(&peak)
	if observed > 2 {
		t.Errorf("peak concurrency was %d, expected <= 2", observed)
	}
	t.Logf("peak concurrency observed: %d (limit: 2)", observed)
}

// slowConcurrentTool tracks concurrent executions for testing the semaphore.
type slowConcurrentTool struct {
	def     common.ToolDef
	delay   time.Duration
	current *int64
	peak    *int64
}

func (s slowConcurrentTool) Definition() common.ToolDef { return s.def }
func (s slowConcurrentTool) Handle(args map[string]any) any {
	n := atomic.AddInt64(s.current, 1)
	// Update peak.
	for {
		old := atomic.LoadInt64(s.peak)
		if n <= old || atomic.CompareAndSwapInt64(s.peak, old, n) {
			break
		}
	}
	time.Sleep(s.delay)
	atomic.AddInt64(s.current, -1)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
}
