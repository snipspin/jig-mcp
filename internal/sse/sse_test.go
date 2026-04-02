package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/server"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// setTestTokens is a helper that sets env vars and re-initializes the token
// registry. Callers must defer the returned cleanup function.
func setTestTokens(envName, envValue string) func() {
	os.Setenv(envName, envValue)
	auth.InitTokenRegistry()
	return func() {
		os.Unsetenv(envName)
		// Clear the registry.
		auth.GlobalTokens().Clear()
	}
}

func TestAuthenticateNoToken(t *testing.T) {
	// When no token env is set, all requests pass through.
	os.Unsetenv("JIG_AUTH_TOKEN")
	os.Unsetenv("JIG_AUTH_TOKENS")
	auth.GlobalTokens().Clear()

	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/sse", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthenticateBearerToken(t *testing.T) {
	defer setTestTokens("JIG_AUTH_TOKEN", "secret123")()
	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Valid bearer token
	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Invalid bearer token
	req2 := httptest.NewRequest("GET", "/sse", nil)
	req2.Header.Set("Authorization", "Bearer wrong")
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec2.Code)
	}
}

func TestAuthenticateQueryToken(t *testing.T) {
	defer setTestTokens("JIG_AUTH_TOKEN", "secret123")()
	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/sse?token=secret123", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthenticateNoCredentials(t *testing.T) {
	defer setTestTokens("JIG_AUTH_TOKEN", "secret123")()
	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/sse", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticateMultipleTokens(t *testing.T) {
	defer setTestTokens("JIG_AUTH_TOKENS", "agent1:tok_aaa,agent2:tok_bbb")()
	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		caller := auth.CallerFrom(r.Context())
		w.Header().Set("X-Caller", caller.Name)
		w.WriteHeader(http.StatusOK)
	})

	// Agent 1 token
	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Authorization", "Bearer tok_aaa")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("agent1: expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Caller") != "agent1" {
		t.Errorf("expected caller 'agent1', got %q", rec.Header().Get("X-Caller"))
	}

	// Agent 2 token
	req2 := httptest.NewRequest("GET", "/sse", nil)
	req2.Header.Set("Authorization", "Bearer tok_bbb")
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("agent2: expected 200, got %d", rec2.Code)
	}
	if rec2.Header().Get("X-Caller") != "agent2" {
		t.Errorf("expected caller 'agent2', got %q", rec2.Header().Get("X-Caller"))
	}

	// Invalid token
	req3 := httptest.NewRequest("GET", "/sse", nil)
	req3.Header.Set("Authorization", "Bearer tok_ccc")
	rec3 := httptest.NewRecorder()
	handler(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("invalid token: expected 401, got %d", rec3.Code)
	}
}

func TestAuthenticateCallerIdentity(t *testing.T) {
	// With JIG_AUTH_TOKEN (singular), caller name should be "default".
	defer setTestTokens("JIG_AUTH_TOKEN", "mysecret")()
	handler := authenticate(func(w http.ResponseWriter, r *http.Request) {
		caller := auth.CallerFrom(r.Context())
		w.Header().Set("X-Caller", caller.Name)
		w.Header().Set("X-Transport", caller.Transport)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Authorization", "Bearer mysecret")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Caller") != "default" {
		t.Errorf("expected caller 'default', got %q", rec.Header().Get("X-Caller"))
	}
	if rec.Header().Get("X-Transport") != "sse" {
		t.Errorf("expected transport 'sse', got %q", rec.Header().Get("X-Transport"))
	}
}

// mockTool is a simple mock tool for testing
type mockTool struct {
	def   common.ToolDef
	reply map[string]any
}

func (m mockTool) Definition() common.ToolDef { return m.def }
func (m mockTool) Handle(args map[string]any) any {
	return m.reply
}

// TestSSESessionLifecycle tests that connecting to /sse creates a session
// and returns an endpoint event.
func TestSSESessionLifecycle(t *testing.T) {
	// Save and restore sessions map
	sessionsMu.Lock()
	oldSessions := sessions
	sessions = make(map[string]*sseSession)
	sessionsMu.Unlock()
	defer func() {
		sessionsMu.Lock()
		sessions = oldSessions
		sessionsMu.Unlock()
	}()

	// Set up SSE handler with httptest
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", authenticate(handleSSE))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Connect to /sse endpoint
	resp, err := http.Get(ts.URL + "/sse")
	if err != nil {
		t.Fatalf("failed to connect to /sse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Read the first SSE events
	reader := bufio.NewReader(resp.Body)
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}

	// Verify endpoint event
	if !strings.Contains(line1, "event: endpoint") {
		t.Errorf("expected 'event: endpoint', got %q", line1)
	}

	// Parse session ID from data line: "data: /message?session_id=session-...\n"
	dataStr := strings.TrimPrefix(strings.TrimSpace(line2), "data: ")
	u, err := url.Parse(dataStr)
	if err != nil {
		t.Fatalf("failed to parse endpoint URL: %v", err)
	}
	sessionID := u.Query().Get("session_id")
	if sessionID == "" {
		t.Fatal("session_id not found in endpoint event")
	}

	// Verify session exists in the sessions map
	sessionsMu.RLock()
	_, exists := sessions[sessionID]
	sessionsMu.RUnlock()
	if !exists {
		t.Errorf("session %q not found in sessions map", sessionID)
	}
}

// TestSSEMessageRouting tests that posting a JSON-RPC request to /message
// returns a response via the SSE stream.
func TestSSEMessageRouting(t *testing.T) {
	// Save and restore sessions map
	sessionsMu.Lock()
	oldSessions := sessions
	sessions = make(map[string]*sseSession)
	sessionsMu.Unlock()
	defer func() {
		sessionsMu.Lock()
		sessions = oldSessions
		sessionsMu.Unlock()
	}()

	// Set up SSE handlers with httptest
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", authenticate(handleSSE))

	// Create a mock server with a test tool
	registry := tools.NewRegistry()
	registry.RegisterTool("echo_test", mockTool{
		def: common.ToolDef{
			Name:        "echo_test",
			Description: "Test echo tool",
			InputSchema: map[string]any{},
		},
		reply: map[string]any{
			"content": []map[string]any{{"type": "text", "text": "hello"}},
		},
	})

	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: 30 * time.Second,
		Semaphore:     make(chan struct{}, 10),
	}

	mux.HandleFunc("/message", authenticate(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, srv)
	}))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Connect to /sse and extract session_id
	resp, err := http.Get(ts.URL + "/sse")
	if err != nil {
		t.Fatalf("failed to connect to /sse: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read endpoint event: "event: endpoint\ndata: /message?session_id=...\n\n"
	reader.ReadString('\n')             // "event: endpoint\n"
	line2, _ := reader.ReadString('\n') // "data: /message?session_id=...\n"
	reader.ReadString('\n')             // empty line separator

	dataStr := strings.TrimPrefix(strings.TrimSpace(line2), "data: ")
	messageURL := ts.URL + dataStr

	// POST a tools/call JSON-RPC request
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo_test","arguments":{}}}`
	postResp, err := http.Post(messageURL, "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to POST message: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202, got %d", postResp.StatusCode)
	}

	// Read the response from SSE stream (event: message\ndata: {...}\n\n)
	eventLine, _ := reader.ReadString('\n')    // "event: message\n"
	responseLine, _ := reader.ReadString('\n') // "data: {...}\n"

	if !strings.Contains(eventLine, "event: message") {
		t.Errorf("expected 'event: message', got %q", eventLine)
	}

	responseData := strings.TrimPrefix(strings.TrimSpace(responseLine), "data: ")
	var rpcResp map[string]any
	if err := json.Unmarshal([]byte(responseData), &rpcResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v (raw: %q)", err, responseData)
	}

	if rpcResp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", rpcResp["jsonrpc"])
	}
}

// TestSSEInvalidSessionID tests that an invalid session ID returns 404.
func TestSSEInvalidSessionID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/message", authenticate(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, &server.Server{
			Registry:      tools.NewRegistry(),
			GlobalTimeout: 30 * time.Second,
		})
	}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	resp, err := http.Post(ts.URL+"/message?session_id=nonexistent", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestSSEMethodNotAllowed tests that GET requests to /message return 405.
func TestSSEMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/message", authenticate(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, &server.Server{
			Registry:      tools.NewRegistry(),
			GlobalTimeout: 30 * time.Second,
		})
	}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/message?session_id=test")
	if err != nil {
		t.Fatalf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestSSECloseAllSessions tests that CloseAllSessions cleans up all sessions.
func TestSSECloseAllSessions(t *testing.T) {
	// Save and restore sessions map
	sessionsMu.Lock()
	oldSessions := sessions
	sessions = make(map[string]*sseSession)
	sessionsMu.Unlock()
	defer func() {
		sessionsMu.Lock()
		sessions = oldSessions
		sessionsMu.Unlock()
	}()

	// Create some fake sessions directly
	sess1 := &sseSession{id: "test-1", messages: make(chan []byte, 10), done: make(chan struct{})}
	sess2 := &sseSession{id: "test-2", messages: make(chan []byte, 10), done: make(chan struct{})}

	sessionsMu.Lock()
	sessions["test-1"] = sess1
	sessions["test-2"] = sess2
	sessionsMu.Unlock()

	CloseAllSessions()

	// Verify sessions map is empty
	sessionsMu.RLock()
	count := len(sessions)
	sessionsMu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 sessions after CloseAllSessions, got %d", count)
	}

	// Verify done channels are closed
	select {
	case <-sess1.done:
		// OK - channel was closed
	default:
		t.Error("sess1.done was not closed")
	}

	select {
	case <-sess2.done:
		// OK - channel was closed
	default:
		t.Error("sess2.done was not closed")
	}
}

// TestSSEMissingSessionID tests that missing session_id returns 400.
func TestSSEMissingSessionID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/message", authenticate(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, &server.Server{
			Registry:      tools.NewRegistry(),
			GlobalTimeout: 30 * time.Second,
		})
	}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	resp, err := http.Post(ts.URL+"/message", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestSSENotificationNoResponse tests that JSON-RPC notifications (no "id" field)
// return 202 but don't send a response on the SSE stream.
func TestSSENotificationNoResponse(t *testing.T) {
	// Save and restore sessions map
	sessionsMu.Lock()
	oldSessions := sessions
	sessions = make(map[string]*sseSession)
	sessionsMu.Unlock()
	defer func() {
		sessionsMu.Lock()
		sessions = oldSessions
		sessionsMu.Unlock()
	}()

	// Set up SSE handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", authenticate(handleSSE))
	mux.HandleFunc("/message", authenticate(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, &server.Server{
			Registry:      tools.NewRegistry(),
			GlobalTimeout: 30 * time.Second,
		})
	}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Connect to /sse and extract session_id
	resp, err := http.Get(ts.URL + "/sse")
	if err != nil {
		t.Fatalf("failed to connect to /sse: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	reader.ReadString('\n')                // event: endpoint
	dataLine, _ := reader.ReadString('\n') // data: /message?session_id=...

	dataStr := strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")
	messageURL := ts.URL + dataStr

	// POST a notification (no "id" field)
	reqBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	postResp, err := http.Post(messageURL, "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to POST notification: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202, got %d", postResp.StatusCode)
	}

	// Verify no message event appears on the SSE stream within a short timeout
	done := make(chan bool)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- false // timeout or EOF - no message
				return
			}
			if strings.Contains(line, "event: message") {
				done <- true // got unexpected message
				return
			}
			// Skip data line
			reader.ReadString('\n')
		}
	}()

	select {
	case gotMessage := <-done:
		if gotMessage {
			t.Error("unexpected message event for notification request")
		}
	case <-time.After(200 * time.Millisecond):
		// No message received within timeout - this is expected
	}
}

// TestSSEStartSSEServer tests the StartSSEServer function.
func TestSSEStartSSEServer(t *testing.T) {
	// Save and restore sessions map
	sessionsMu.Lock()
	oldSessions := sessions
	sessions = make(map[string]*sseSession)
	sessionsMu.Unlock()
	defer func() {
		sessionsMu.Lock()
		sessions = oldSessions
		sessionsMu.Unlock()
	}()

	registry := tools.NewRegistry()
	semaphore := make(chan struct{}, 4)

	// Start server on port 0 (OS picks free port)
	httpServer, errCh := StartSSEServer(0, registry, 30*time.Second, semaphore, 4, "test", nil)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Verify server is running by making a request
	mux := httpServer.Handler.(*http.ServeMux)
	_ = mux // The handler is set up correctly

	// Shut down the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}

	// Wait for server to stop
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("unexpected server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not shut down in time")
	}
}

// nonFlushableWriter is a mock http.ResponseWriter that doesn't implement http.Flusher
type nonFlushableWriter struct {
	headerMap http.Header
	code      int
	body      []byte
}

func (n *nonFlushableWriter) Header() http.Header {
	if n.headerMap == nil {
		n.headerMap = make(http.Header)
	}
	return n.headerMap
}

func (n *nonFlushableWriter) Write(b []byte) (int, error) {
	n.body = append(n.body, b...)
	return len(b), nil
}

func (n *nonFlushableWriter) WriteHeader(statusCode int) {
	n.code = statusCode
}

// TestHandleSSEStreamingUnsupported tests that handleSSE returns 500 when
// the ResponseWriter doesn't implement http.Flusher.
func TestHandleSSEStreamingUnsupported(t *testing.T) {
	req := httptest.NewRequest("GET", "/sse", nil)
	w := &nonFlushableWriter{}

	handleSSE(w, req)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.code)
	}
	if !strings.Contains(string(w.body), "Streaming unsupported") {
		t.Errorf("expected 'Streaming unsupported' error, got %q", string(w.body))
	}
}
