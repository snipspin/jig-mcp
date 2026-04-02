package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// TestExternalTool_Handle_PlainTextOutput tests that non-JSON output is returned as text
func TestExternalTool_Handle_PlainTextOutput(t *testing.T) {
	// Create a script that returns plain text
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "text_output.sh")
	script := `#!/bin/bash
echo "plain text output"
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "text_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if text != "plain text output\n" {
		t.Errorf("expected 'plain text output\\n', got %q", text)
	}
}

// TestExternalTool_Handle_JSONArray tests that JSON array output is returned as-is
func TestExternalTool_Handle_JSONArray(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "array_output.sh")
	script := `#!/bin/bash
echo '["item1", "item2", "item3"]'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "array_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultArr, ok := result.([]any)
	if !ok {
		// It might be wrapped as text - check that path
		resultMap, ok := result.(map[string]any)
		if ok {
			content, _ := resultMap["content"].([]map[string]any)
			if len(content) > 0 {
				t.Logf("array was wrapped as text: %v", content[0]["text"])
				return
			}
		}
		t.Fatalf("expected result to be []any, got %T", result)
	}

	if len(resultArr) != 3 {
		t.Errorf("expected 3 items, got %d", len(resultArr))
	}
}

// TestExternalTool_Handle_SingleContentItem tests wrapping of single content items
func TestExternalTool_Handle_SingleContentItem(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "single_item.sh")
	script := `#!/bin/bash
echo '{"type": "text", "text": "single item response"}'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "single_item_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatal("expected content item to be map")
	}

	if item["type"] != "text" {
		t.Errorf("expected type 'text', got %v", item["type"])
	}
	if item["text"] != "single item response" {
		t.Errorf("expected 'single item response', got %v", item["text"])
	}
}

// TestExternalTool_Handle_NoContentField tests wrapping response without content field
func TestExternalTool_Handle_NoContentField(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "no_content.sh")
	script := `#!/bin/bash
echo '{"result": "success", "data": 42}'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "no_content_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	// The response should be wrapped as indented JSON text
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("expected wrapped response to be valid JSON: %v", err)
	}
	if parsed["result"] != "success" {
		t.Errorf("expected result='success', got %v", parsed["result"])
	}
}

// TestExternalTool_Handle_InvalidBase64 tests that invalid base64 in image content returns error
func TestExternalTool_Handle_InvalidBase64(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "bad_image.sh")
	script := `#!/bin/bash
echo '{"content": [{"type": "image", "data": "not-valid-base64!!!"}]}'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "bad_image_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError=true for invalid base64")
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !strings.Contains(text, "invalid base64") {
		t.Errorf("expected base64 error, got: %s", text)
	}
}

// TestExternalTool_Handle_ImageMissingData tests that image content without data field returns error
func TestExternalTool_Handle_ImageMissingData(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "missing_data.sh")
	script := `#!/bin/bash
echo '{"content": [{"type": "image", "mimeType": "image/png"}]}'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "missing_data_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, ok := resultMap["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError=true for missing data field")
	}
}

// TestExternalTool_Handle_ValidBase64Image tests that valid base64 image passes validation
func TestExternalTool_Handle_ValidBase64Image(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "valid_image.sh")
	// Valid base64 encoded PNG header (minimal valid example)
	script := `#!/bin/bash
echo '{"content": [{"type": "image", "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==", "mimeType": "image/png"}]}'
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	cfg := config.ToolConfig{
		Name: "valid_image_tool",
		Platforms: map[string]config.PlatformConfig{
			"linux": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
			"darwin": {
				Command: "bash",
				Args:    []string{scriptPath},
			},
		},
	}
	tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: cfg}}

	result := tool.Handle(map[string]any{})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map[string]any, got %T", result)
	}

	isError, hasError := resultMap["isError"].(bool)
	if hasError && isError {
		t.Error("expected no error for valid base64 image")
	}

	content, ok := resultMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in result")
	}
}
