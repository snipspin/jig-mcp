// Package tools implements the concrete tool types (ExternalTool, HTTPTool,
// TerminalTool) and a thread-safe tool registry for jig-mcp.
package tools

import (
	"log/slog"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/config"
)

// DefaultGlobalTimeout is the fallback timeout for tools (30s).
const DefaultGlobalTimeout = 30 * time.Second

// BaseTool provides the shared Definition() implementation for all tool types.
type BaseTool struct {
	Config config.ToolConfig
}

// Definition returns the MCP tool definition derived from the tool's configuration.
func (b BaseTool) Definition() common.ToolDef {
	return common.ToolDef{
		Name:        b.Config.Name,
		Description: b.Config.Description,
		InputSchema: b.Config.InputSchema,
	}
}

// effectiveTimeout returns the per-tool timeout if set, otherwise the provided fallback.
func (b BaseTool) effectiveTimeout(fallback time.Duration) time.Duration {
	if b.Config.Timeout != "" {
		if d, err := time.ParseDuration(b.Config.Timeout); err == nil && d > 0 {
			return d
		}
		slog.Warn("invalid per-tool timeout, using default", "timeout", b.Config.Timeout, "tool", b.Config.Name)
	}
	return fallback
}

// errorResult builds an MCP error response with a text message.
func errorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": msg,
		}},
		"isError": true,
	}
}

// textResult builds an MCP success response with a text message.
func textResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
	}
}
