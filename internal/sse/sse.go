// Package sse implements the HTTP/SSE transport for jig-mcp, providing
// Server-Sent Events streaming with session management and token authentication.
package sse

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/logging"
	"github.com/snipspin/jig-mcp/internal/server"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// sseSessionBufferSize is the buffer size for SSE session message channels.
const sseSessionBufferSize = 100

// sseKeepAliveInterval is the interval for SSE keep-alive ticks.
const sseKeepAliveInterval = 15 * time.Second

// sseMsgSendTimeout is the timeout for sending messages to SSE session buffer.
const sseMsgSendTimeout = 5 * time.Second

type sseSession struct {
	id       string
	messages chan []byte
	done     chan struct{}
}

var (
	sessions   = make(map[string]*sseSession)
	sessionsMu sync.RWMutex
)

const maxBodySize = 1 << 20 // 1 MB

// CloseAllSessions sends a close event to every active SSE session and
// removes it from the session map.
func CloseAllSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	for id, sess := range sessions {
		// Best-effort close event — the client may already be gone.
		select {
		case sess.messages <- []byte(`{"jsonrpc":"2.0","method":"notifications/shutdown"}`):
		default:
		}
		close(sess.done)
		delete(sessions, id)
		slog.Info("session closed (shutdown)", "session", id)
	}
}

func authenticate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tr := auth.GlobalTokens()
		if !tr.AuthRequired() {
			// No tokens configured — pass through with anonymous identity.
			r = r.WithContext(auth.WithCaller(r.Context(), auth.CallerIdentity{
				Name:      "anonymous",
				Transport: "sse",
			}))
			h(w, r)
			return
		}

		// Extract candidate token from Authorization header or query parameter.
		var candidate string
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				candidate = parts[1]
			}
		}
		if candidate == "" {
			candidate = r.URL.Query().Get("token")
		}

		if name, ok := tr.Lookup(candidate); ok {
			r = r.WithContext(auth.WithCaller(r.Context(), auth.CallerIdentity{
				Name:      name,
				Transport: "sse",
			}))
			h(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Bearer realm="jig-mcp"`)
		http.Error(w, "Unauthorized: missing or invalid token", http.StatusUnauthorized)
		slog.Warn("unauthorized connection attempt", logging.Sanitize("remote_addr", r.RemoteAddr))
	}
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// CORS is not set by default — operators should configure this via a
	// reverse proxy if cross-origin access is needed. Setting Access-Control-Allow-Origin: *
	// on a tool execution endpoint would let any website trigger tool calls.

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Create a new session with cryptographically random ID
	idBytes := make([]byte, 16)
	if _, err := crypto_rand.Read(idBytes); err != nil {
		http.Error(w, "internal error generating session", http.StatusInternalServerError)
		return
	}
	sessionID := fmt.Sprintf("session-%x", idBytes)
	session := &sseSession{
		id:       sessionID,
		messages: make(chan []byte, sseSessionBufferSize),
		done:     make(chan struct{}),
	}

	sessionsMu.Lock()
	sessions[sessionID] = session
	sessionsMu.Unlock()

	defer func() {
		sessionsMu.Lock()
		// Only clean up if not already removed by CloseAllSessions.
		delete(sessions, sessionID)
		sessionsMu.Unlock()
		slog.Info("session closed", "session", sessionID)
	}()

	slog.Info("new SSE session", "session", sessionID)

	// Send the endpoint event as per MCP spec
	// The endpoint should be the message URL with the session ID
	fmt.Fprintf(w, "event: endpoint\ndata: /message?session_id=%s\n\n", sessionID)
	flusher.Flush()

	// Keep-alive ticker
	ticker := time.NewTicker(sseKeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-session.messages:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		case <-ticker.C:
			// Send a comment as keep-alive
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-session.done:
			// Server is shutting down — drain any remaining messages.
			for {
				select {
				case msg := <-session.messages:
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
					flusher.Flush()
				default:
					return
				}
			}
		case <-r.Context().Done():
			return
		}
	}
}

func handleMessage(w http.ResponseWriter, r *http.Request, srv *server.Server) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	sessionsMu.RLock()
	session, ok := sessions[sessionID]
	sessionsMu.RUnlock()

	if !ok {
		http.Error(w, "Invalid session_id", http.StatusNotFound)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	var req server.Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	// JSON-RPC notifications have no "id" field — don't send a response.
	if req.ID == nil || string(req.ID) == "null" {
		slog.Debug("SSE notification received, no response sent", "method", req.Method)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Process the request with caller identity from auth middleware.
	resp := srv.ProcessRequest(r.Context(), req)

	// Send the response back through the SSE stream
	respBytes, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal response", "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	select {
	case session.messages <- respBytes:
		w.WriteHeader(http.StatusAccepted)
	case <-time.After(sseMsgSendTimeout):
		http.Error(w, "Session buffer full or timed out", http.StatusServiceUnavailable)
	}
}

// StartSSEServer starts an HTTP server for SSE transport.
func StartSSEServer(port int, registry *tools.Registry, globalTimeout time.Duration, semaphore chan struct{}, semaphoreSize int, version string, onToolCall func(string, int64)) (*http.Server, <-chan error) {
	mux := http.NewServeMux()

	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: globalTimeout,
		Semaphore:     semaphore,
		SemaphoreSize: semaphoreSize,
		Version:       version,
		OnToolCall:    onToolCall,
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	})
	mux.HandleFunc("/sse", authenticate(handleSSE))
	// Wrap /message with TimeoutHandler to protect against slow clients.
	// The /sse endpoint is intentionally unwrapped to allow long-lived streaming.
	messageHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleMessage(w, r, srv)
	})
	mux.Handle("/message", http.TimeoutHandler(messageHandler, 30*time.Second, "request timeout"))

	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting MCP SSE server", "addr", addr)

	// WriteTimeout is intentionally omitted: the /sse endpoint is a long-lived
	// stream that would be killed by a global write deadline. The /message
	// endpoint is wrapped in http.TimeoutHandler instead.
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("SSE server failed", "err", err)
			errCh <- err
		}
		close(errCh)
	}()

	return httpServer, errCh
}
