package tools

import (
	"testing"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/config"
)

// stubTool is a minimal Tool for registry tests.
type stubTool struct {
	name string
}

func (s stubTool) Definition() common.ToolDef {
	return common.ToolDef{Name: s.name}
}
func (s stubTool) Handle(args map[string]any) any { return nil }

func TestRegistryRegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("alpha", stubTool{name: "alpha"})

	tool, ok := r.GetToolByName("alpha")
	if !ok {
		t.Fatal("expected to find tool alpha")
	}
	if tool.Definition().Name != "alpha" {
		t.Errorf("got name %q, want alpha", tool.Definition().Name)
	}
}

func TestRegistryLookupMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.GetToolByName("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent tool")
	}
}

func TestRegistryGetToolsReturnsSnapshot(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("a", stubTool{name: "a"})
	r.RegisterTool("b", stubTool{name: "b"})

	snap := r.GetTools()
	if len(snap) != 2 {
		t.Fatalf("snapshot length = %d, want 2", len(snap))
	}

	// Mutating snapshot should not affect registry
	delete(snap, "a")
	if _, ok := r.GetToolByName("a"); !ok {
		t.Error("deleting from snapshot should not affect registry")
	}
}

func TestRegistryRemoveTool(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("x", stubTool{name: "x"})
	r.RemoveTool("x")

	if _, ok := r.GetToolByName("x"); ok {
		t.Error("expected tool to be removed")
	}
}

func TestRegistryRemoveNonexistent(t *testing.T) {
	r := NewRegistry()
	// Should not panic
	r.RemoveTool("does_not_exist")
}

func TestRegistryReplaceAll(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("old", stubTool{name: "old"})

	newTools := map[string]common.Tool{
		"new1": stubTool{name: "new1"},
		"new2": stubTool{name: "new2"},
	}
	old := r.ReplaceAll(newTools)

	// Old map returned
	if _, ok := old["old"]; !ok {
		t.Error("expected old map to contain 'old'")
	}

	// New tools accessible
	if _, ok := r.GetToolByName("new1"); !ok {
		t.Error("expected new1 to be accessible")
	}
	// Old tools gone
	if _, ok := r.GetToolByName("old"); ok {
		t.Error("expected old to be gone after ReplaceAll")
	}
}

func TestRegistryRestoreAll(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("orig", stubTool{name: "orig"})

	saved := r.ReplaceAll(map[string]common.Tool{
		"temp": stubTool{name: "temp"},
	})

	r.RestoreAll(saved)
	if _, ok := r.GetToolByName("orig"); !ok {
		t.Error("expected orig to be restored")
	}
	if _, ok := r.GetToolByName("temp"); ok {
		t.Error("expected temp to be gone after restore")
	}
}

func TestRegistryRegisterOverwrites(t *testing.T) {
	r := NewRegistry()
	r.RegisterTool("x", stubTool{name: "v1"})
	r.RegisterTool("x", stubTool{name: "v2"})

	tool, _ := r.GetToolByName("x")
	if tool.Definition().Name != "v2" {
		t.Errorf("expected overwrite, got name %q", tool.Definition().Name)
	}
}

func TestGetConfigExternalTool(t *testing.T) {
	tc := config.ToolConfig{Name: "ext"}
	tool := ExternalTool{BaseTool: BaseTool{Config: tc}}
	cfg, ok := GetConfig(tool)
	if !ok {
		t.Fatal("expected ok=true for ExternalTool")
	}
	if cfg.Name != "ext" {
		t.Errorf("Name = %q, want ext", cfg.Name)
	}
}

func TestGetConfigHTTPTool(t *testing.T) {
	tc := config.ToolConfig{Name: "http_t"}
	tool := HTTPTool{BaseTool: BaseTool{Config: tc}}
	cfg, ok := GetConfig(tool)
	if !ok {
		t.Fatal("expected ok=true for HTTPTool")
	}
	if cfg.Name != "http_t" {
		t.Errorf("Name = %q, want http_t", cfg.Name)
	}
}

func TestGetConfigTerminalTool(t *testing.T) {
	tc := config.ToolConfig{Name: "term"}
	tool := TerminalTool{BaseTool: BaseTool{Config: tc}}
	cfg, ok := GetConfig(tool)
	if !ok {
		t.Fatal("expected ok=true for TerminalTool")
	}
	if cfg.Name != "term" {
		t.Errorf("Name = %q, want term", cfg.Name)
	}
}

func TestGetConfigUnknownType(t *testing.T) {
	_, ok := GetConfig(stubTool{name: "unknown"})
	if ok {
		t.Error("expected ok=false for unknown tool type")
	}
}
