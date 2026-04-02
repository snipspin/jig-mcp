package tools

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/snipspin/jig-mcp/internal/config"
)

func TestDockerSandboxWrapping(t *testing.T) {
	// We want to verify that the command is correctly transformed to "docker run --rm ..."
	// without actually running docker (since it might not be installed).
	// However, ExternalTool.Handle *actually* runs the command.
	// So we might need to mock or just check if it returns "docker sandbox requested..." error.

	tc := config.ToolConfig{
		Name:        "docker_test",
		Description: "Test sandbox",
		InputSchema: map[string]any{"type": "object"},
		Platforms: map[string]config.PlatformConfig{
			"windows": {Command: "echo", Args: []string{"hello"}},
			"linux":   {Command: "echo", Args: []string{"hello"}},
			"darwin":  {Command: "echo", Args: []string{"hello"}},
		},
		Sandbox: &config.SandboxConfig{
			Type:  "docker",
			Image: "alpine:latest",
		},
	}

	tool := ExternalTool{BaseTool: BaseTool{Config: tc}}

	// This will try to run 'docker'.
	// If docker is not installed, it should return a graceful error.
	resp := tool.Handle(map[string]any{})

	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}

	if respMap["isError"] != true {
		// If it succeeded, it means docker is installed and it actually ran.
		// We still want to verify the output if possible.
		return
	}

	content, _ := respMap["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatalf("missing content in error response")
	}
	text, _ := content[0]["text"].(string)

	// If docker is missing, it should say so.
	if _, err := exec.LookPath("docker"); err != nil {
		if !strings.Contains(text, "docker sandbox requested but docker command not found in PATH") {
			t.Errorf("expected missing docker error, got: %s", text)
		}
	} else {
		// If docker IS installed but failed for some other reason (e.g. image not found)
		t.Logf("Docker is installed, error was: %s", text)
	}
}

func TestWasmSandboxNotImplemented(t *testing.T) {
	tc := config.ToolConfig{
		Name: "wasm_test",
		Platforms: map[string]config.PlatformConfig{
			runtime.GOOS: {Command: "echo"},
		},
		Sandbox: &config.SandboxConfig{
			Type: "wasm",
		},
	}
	tool := ExternalTool{BaseTool: BaseTool{Config: tc}}
	resp := tool.Handle(map[string]any{})

	respMap := resp.(map[string]any)
	if respMap["isError"] != true {
		t.Fatal("expected error for unimplemented wasm sandbox")
	}
	content := respMap["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if !strings.Contains(text, "wasm sandbox mode is not yet supported") {
		t.Errorf("expected not implemented message, got: %s", text)
	}
}
