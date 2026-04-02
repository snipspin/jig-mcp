// Package common defines the shared types used across jig-mcp packages.
package common

// ToolDef describes a tool's name, description, and JSON Schema for input validation.
// It is returned by Tool.Definition and serialized in the MCP tools/list response.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Tool is the interface that all jig-mcp tool types must implement.
// Definition returns the tool's metadata for MCP discovery.
// Handle executes the tool with the given arguments and returns an MCP result.
type Tool interface {
	Definition() ToolDef
	Handle(args map[string]any) any
}
