// Package server implements JSON-RPC request processing for the MCP protocol,
// including tool dispatch, panic recovery, and concurrency limiting.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"log/slog"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/logging"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Server holds the state needed for request processing.
type Server struct {
	Registry      *tools.Registry
	GlobalTimeout time.Duration
	Semaphore     chan struct{}
	SemaphoreSize int
	Version       string
	OnToolCall    func(toolName string, durationMS int64) // optional metrics callback
}

// ProcessRequest handles a JSON-RPC request and returns a response.
func (s *Server) ProcessRequest(ctx context.Context, req Request) Response {
	var resp Response
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{
				"name":    "jig-mcp",
				"version": s.Version,
			},
		}

	case "tools/list":
		var toolsList []common.ToolDef
		for _, tool := range s.Registry.GetTools() {
			toolsList = append(toolsList, tool.Definition())
		}
		sort.Slice(toolsList, func(i, j int) bool {
			return toolsList[i].Name < toolsList[j].Name
		})
		resp.Result = map[string]any{
			"tools": toolsList,
		}

	case "tools/call":
		resp.Result = s.HandleToolCall(ctx, req.Params)

	case "ping":
		resp.Result = map[string]any{}

	default:
		resp.Error = map[string]any{
			"code":    -32601,
			"message": fmt.Sprintf("method not found: %s", req.Method),
		}
	}
	return resp
}

// HandleToolCall executes a tool and returns the result.
func (s *Server) HandleToolCall(ctx context.Context, raw json.RawMessage) (result any) {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": fmt.Sprintf("invalid params: %v", err),
			}},
			"isError": true,
		}
	}

	// Recover from panics in tool handlers to prevent crashing the server.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in tool handler (recovered)", logging.Sanitize("tool", params.Name), slog.Any("panic", r))
			result = map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": fmt.Sprintf("internal error: tool %q panicked", params.Name),
				}},
				"isError": true,
			}
		}
	}()

	tool, ok := s.Registry.GetToolByName(params.Name)
	if !ok {
		return map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": fmt.Sprintf("unknown tool: %s", params.Name),
			}},
			"isError": true,
		}
	}

	// Acquire concurrency semaphore with a timeout to prevent unbounded queuing.
	if s.Semaphore != nil {
		timeout := s.GlobalTimeout + 5*time.Second
		select {
		case s.Semaphore <- struct{}{}:
			defer func() { <-s.Semaphore }()
		case <-time.After(timeout):
			slog.Warn("tool call rejected: concurrency limit reached", logging.Sanitize("tool", params.Name), slog.Int("limit", s.SemaphoreSize))
			return map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": fmt.Sprintf("server busy: all %d tool execution slots are occupied, try again later", s.SemaphoreSize),
				}},
				"isError": true,
			}
		}
	}

	// Extract caller identity from context.
	callerName := auth.CallerFrom(ctx).Name

	startTime := time.Now()
	result = tool.Handle(params.Arguments)
	duration := time.Since(startTime)

	audit.Record(params.Name, params.Arguments, duration, result, tool, callerName)
	if s.OnToolCall != nil {
		s.OnToolCall(params.Name, duration.Milliseconds())
	}

	return result
}
