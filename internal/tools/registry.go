package tools

import (
	"sync"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/config"
)

// GetConfig extracts the ToolConfig from a Tool if it's one of our concrete types.
// Returns a zero-value config and false if the tool type is unknown.
func GetConfig(tool common.Tool) (config.ToolConfig, bool) {
	switch t := tool.(type) {
	case ExternalTool:
		return t.Config, true
	case HTTPTool:
		return t.Config, true
	case TerminalTool:
		return t.Config, true
	default:
		return config.ToolConfig{}, false
	}
}

// Registry provides concurrency-safe access to the active tool map.
// In production, writes happen only during init (loadConfig/probeScripts)
// before any server goroutine starts, but tests mutate the map concurrently
// which triggers the race detector without synchronization.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]common.Tool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]common.Tool)}
}

// GetTools returns a snapshot copy of all registered tools.
func (r *Registry) GetTools() map[string]common.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := make(map[string]common.Tool, len(r.tools))
	for k, v := range r.tools {
		snapshot[k] = v
	}
	return snapshot
}

// GetToolByName looks up a single tool by name.
func (r *Registry) GetToolByName(name string) (common.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// RegisterTool adds or replaces a tool in the registry.
func (r *Registry) RegisterTool(name string, tool common.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
}

// RemoveTool deletes a tool from the registry.
func (r *Registry) RemoveTool(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// ReplaceAll swaps the entire tool map (used in tests). Returns the old map.
func (r *Registry) ReplaceAll(newTools map[string]common.Tool) map[string]common.Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.tools
	r.tools = newTools
	return old
}

// RestoreAll restores a previously saved tool map (used in test cleanup).
func (r *Registry) RestoreAll(old map[string]common.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = old
}
