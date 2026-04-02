package tools

import (
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/internal/config"
)

func TestDefinitionReturnsConfigFields(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{
		Name:        "my_tool",
		Description: "does stuff",
		InputSchema: map[string]any{"type": "object"},
	}}
	def := bt.Definition()
	if def.Name != "my_tool" {
		t.Errorf("Name = %q, want %q", def.Name, "my_tool")
	}
	if def.Description != "does stuff" {
		t.Errorf("Description = %q, want %q", def.Description, "does stuff")
	}
	if def.InputSchema["type"] != "object" {
		t.Errorf("InputSchema[type] = %v, want %q", def.InputSchema["type"], "object")
	}
}

func TestEffectiveTimeoutValidDuration(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{Timeout: "5s"}}
	got := bt.effectiveTimeout(30 * time.Second)
	if got != 5*time.Second {
		t.Errorf("effectiveTimeout = %v, want 5s", got)
	}
}

func TestEffectiveTimeoutInvalidFallsBack(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{Timeout: "not-a-duration"}}
	got := bt.effectiveTimeout(30 * time.Second)
	if got != 30*time.Second {
		t.Errorf("effectiveTimeout = %v, want 30s", got)
	}
}

func TestEffectiveTimeoutEmptyUsesFallback(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{}}
	got := bt.effectiveTimeout(15 * time.Second)
	if got != 15*time.Second {
		t.Errorf("effectiveTimeout = %v, want 15s", got)
	}
}

func TestEffectiveTimeoutZeroDurationFallsBack(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{Timeout: "0s"}}
	got := bt.effectiveTimeout(30 * time.Second)
	if got != 30*time.Second {
		t.Errorf("effectiveTimeout = %v, want 30s (0s is not > 0)", got)
	}
}

func TestEffectiveTimeoutNegativeFallsBack(t *testing.T) {
	bt := BaseTool{Config: config.ToolConfig{Timeout: "-5s"}}
	got := bt.effectiveTimeout(30 * time.Second)
	if got != 30*time.Second {
		t.Errorf("effectiveTimeout = %v, want 30s", got)
	}
}

func TestErrorResultFormat(t *testing.T) {
	res := errorResult("something broke")
	if res["isError"] != true {
		t.Error("expected isError=true")
	}
	content, ok := res["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatal("expected content with 1 item")
	}
	if content[0]["type"] != "text" {
		t.Errorf("type = %v, want text", content[0]["type"])
	}
	if content[0]["text"] != "something broke" {
		t.Errorf("text = %v, want %q", content[0]["text"], "something broke")
	}
}

func TestTextResultFormat(t *testing.T) {
	res := textResult("hello world")
	if _, hasErr := res["isError"]; hasErr {
		t.Error("textResult should not have isError key")
	}
	content, ok := res["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatal("expected content with 1 item")
	}
	if content[0]["type"] != "text" {
		t.Errorf("type = %v, want text", content[0]["type"])
	}
	if content[0]["text"] != "hello world" {
		t.Errorf("text = %v, want %q", content[0]["text"], "hello world")
	}
}
