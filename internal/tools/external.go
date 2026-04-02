package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"log/slog"

	"github.com/snipspin/jig-mcp/internal/rlimit"
)

// ExternalTool executes scripts and binaries as MCP tools, with support for
// Docker sandbox isolation and platform-specific commands.
type ExternalTool struct {
	BaseTool
}

// Handle executes the external tool with the given arguments and returns an MCP result.
func (t ExternalTool) Handle(args map[string]any) any {
	plat, ok := t.Config.Platforms[runtime.GOOS]
	if !ok {
		return errorResult(fmt.Sprintf("unsupported platform: %s", runtime.GOOS))
	}

	timeout := t.effectiveTimeout(DefaultGlobalTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Prepare arguments as JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		slog.Error("failed to marshal tool arguments", "error", err)
		return errorResult(fmt.Sprintf("failed to marshal arguments: %v", err))
	}

	executable := plat.Command
	argsToPass := make([]string, len(plat.Args), len(plat.Args)+1)
	copy(argsToPass, plat.Args)
	argsToPass = append(argsToPass, string(argsJSON))

	if t.Config.Sandbox != nil {
		switch strings.ToLower(t.Config.Sandbox.Type) {
		case "docker":
			if _, err := exec.LookPath("docker"); err != nil {
				return errorResult("docker sandbox requested but docker command not found in PATH")
			}
			if t.Config.Sandbox.Image == "" {
				return errorResult("docker sandbox requested but no image specified")
			}
			// Wrap: docker run --rm <image> <executable> <args...>
			newArgs := []string{"run", "--rm", t.Config.Sandbox.Image, executable}
			newArgs = append(newArgs, argsToPass...)
			argsToPass = newArgs
			executable = "docker"
		case "wasm":
			return errorResult("wasm sandbox mode is not yet supported")
		}
	}

	// Build command with context for timeout/cancellation.
	cmd := exec.CommandContext(ctx, executable, argsToPass...)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	// Start with resource limits applied.
	limits := rlimit.EffectiveLimits(t.Config.MaxMemoryMB, t.Config.MaxCPUPercent)
	cleanup, startErr := rlimit.StartWithLimits(cmd, limits, int(timeout.Seconds()))
	if startErr != nil {
		return errorResult(fmt.Sprintf("failed to start tool: %v", startErr))
	}
	defer cleanup()

	err = cmd.Wait()
	out := combined.Bytes()

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errorResult(fmt.Sprintf("tool %q killed: execution exceeded timeout of %s", t.Config.Name, timeout))
		}
		return errorResult(fmt.Sprintf("execution error: %v\nOutput: %s", err, string(out)))
	}

	// Attempt to parse output as JSON (expecting an MCP-compliant result or content item)
	var toolResp any
	if err := json.Unmarshal(out, &toolResp); err != nil {
		// If not valid JSON, treat the whole output as a text response
		return textResult(string(out))
	}

	// inspect the response map
	respMap, ok := toolResp.(map[string]any)
	if !ok {
		// Not a map (e.g. array or primitive), return it as-is if it's already structured,
		// but typically MCP CallToolResult is an object.
		return toolResp
	}

	// Case 1: Script returned a single content item instead of a full CallToolResult.
	// We detect this if there's a "type" but no "content" key.
	if typeVal, hasType := respMap["type"].(string); hasType {
		if _, hasContent := respMap["content"]; !hasContent {
			// If it's something MCP-like (text, image, resource), wrap it.
			if typeVal == "text" || typeVal == "image" || typeVal == "resource" {
				toolResp = map[string]any{
					"content": []any{respMap},
				}
				respMap = toolResp.(map[string]any)
			}
		}
	}

	// Case 2: Ensure "content" is present (mandatory for MCP)
	content, ok := respMap["content"].([]any)
	if !ok {
		// If it's valid JSON but doesn't have "content", and wasn't a single item,
		// maybe it's just a raw object the user wants as text?
		// To be safe, if it's not a standard MCP response, wrap it in text.
		data, _ := json.MarshalIndent(respMap, "", "  ")
		return textResult(string(data))
	}

	// Case 3: Iterate content items to validate base64 for images
	for _, item := range content {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if typeVal, _ := itemMap["type"].(string); typeVal == "image" {
			dataStr, _ := itemMap["data"].(string)
			if dataStr == "" {
				return errorResult("error: image content missing 'data' field")
			}
			// Validate base64
			if _, err := base64.StdEncoding.DecodeString(dataStr); err != nil {
				return errorResult(fmt.Sprintf("error: invalid base64 in image content block: %v", err))
			}
		}
	}

	return toolResp
}
