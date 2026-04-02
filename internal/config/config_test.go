package config

import (
	"os"
	"testing"
)

func TestLoadConfigNoToolsDir(t *testing.T) {
	// Save current working directory
	origDir, _ := os.Getwd()

	// Create a temp dir with no tools/ subdirectory
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	tools := LoadManifests("") // LoadManifests is the function that reads tools/

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a tool directory with invalid YAML
	os.MkdirAll("tools/broken", 0755)
	os.WriteFile("tools/broken/manifest.yaml", []byte("{{invalid yaml"), 0644)

	tools := LoadManifests("")

	if len(tools) != 0 {
		t.Errorf("expected 0 tools from invalid YAML, got %d", len(tools))
	}
}

func TestLoadConfigRejectsShellMetachars(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	manifest := `
name: bad_tool
description: "should be rejected"
inputSchema:
  type: object
platforms:
  linux:
    command: "bash;rm"
    args: ["test"]
`
	os.MkdirAll("tools/bad", 0755)
	os.WriteFile("tools/bad/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")

	if len(tools) != 0 {
		t.Errorf("expected 0 tools (metachar rejected), got %d", len(tools))
	}
}

func TestProbeScriptsNoDir(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	existingNames := make(map[string]bool)
	tools := ProbeScripts(existingNames, "")

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestProbeScriptsValidScript(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a script that responds to --mcp-metadata
	os.MkdirAll("scripts", 0755)
	script := `#!/bin/bash
if [ "$1" = "--mcp-metadata" ]; then
    echo '{"name":"probe_test","description":"test tool","inputSchema":{"type":"object","properties":{}}}'
    exit 0
fi
echo '{"content":[{"type":"text","text":"hello"}]}'
`
	os.WriteFile("scripts/probe_test.sh", []byte(script), 0755)

	existingNames := make(map[string]bool)
	tools := ProbeScripts(existingNames, "")

	found := false
	for _, tool := range tools {
		if tool.Config.Name == "probe_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected probe_test to be registered")
	}
}

func TestLoadManifestsHTTPType(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	manifest := `
name: api_tool
description: "HTTP tool"
inputSchema:
  type: object
http:
  url: "http://example.com/api"
  method: GET
`
	os.MkdirAll("tools/api", 0755)
	os.WriteFile("tools/api/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Type != "http" {
		t.Errorf("Type = %q, want http", tools[0].Type)
	}
	if tools[0].Config.HTTP == nil {
		t.Error("expected HTTP config to be set")
	}
}

func TestLoadManifestsTerminalType(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	manifest := `
name: term_tool
description: "Terminal tool"
inputSchema:
  type: object
terminal:
  enabled: true
  allowlist:
    - echo
    - ls
`
	os.MkdirAll("tools/term", 0755)
	os.WriteFile("tools/term/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Type != "terminal" {
		t.Errorf("Type = %q, want terminal", tools[0].Type)
	}
}

func TestLoadManifestsTerminalDisabledSkipped(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	manifest := `
name: disabled_term
description: "Disabled terminal"
inputSchema:
  type: object
terminal:
  enabled: false
  allowlist:
    - echo
`
	os.MkdirAll("tools/disabled", 0755)
	os.WriteFile("tools/disabled/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (disabled terminal), got %d", len(tools))
	}
}

func TestLoadManifestsExternalType(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	manifest := `
name: ext_tool
description: "External tool"
inputSchema:
  type: object
platforms:
  linux:
    command: echo
    args: ["hello"]
`
	os.MkdirAll("tools/ext", 0755)
	os.WriteFile("tools/ext/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Type != "external" {
		t.Errorf("Type = %q, want external", tools[0].Type)
	}
}

func TestLoadConfigSkipsBadManifest(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Tool with no platforms, http, or terminal should be rejected
	manifest := `
name: incomplete
description: "no execution config"
inputSchema:
  type: object
`
	os.MkdirAll("tools/bad", 0755)
	os.WriteFile("tools/bad/manifest.yaml", []byte(manifest), 0644)

	tools := LoadManifests("")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (bad manifest), got %d", len(tools))
	}
}

func TestLoadManifestsMultipleTools(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	m1 := `
name: tool_a
description: "A"
inputSchema:
  type: object
platforms:
  linux:
    command: echo
`
	m2 := `
name: tool_b
description: "B"
inputSchema:
  type: object
http:
  url: "http://example.com"
`
	os.MkdirAll("tools/a", 0755)
	os.WriteFile("tools/a/manifest.yaml", []byte(m1), 0644)
	os.MkdirAll("tools/b", 0755)
	os.WriteFile("tools/b/manifest.yaml", []byte(m2), 0644)

	tools := LoadManifests("")
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestProbeScriptsSkipsDuplicate(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("scripts", 0755)
	script := `#!/bin/bash
if [ "$1" = "--mcp-metadata" ]; then
    echo '{"name":"dup_test","description":"dup","inputSchema":{"type":"object"}}'
    exit 0
fi
`
	os.WriteFile("scripts/dup.sh", []byte(script), 0755)

	// Mark as already existing
	existingNames := map[string]bool{"dup_test": true}
	tools := ProbeScripts(existingNames, "")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (duplicate skipped), got %d", len(tools))
	}
}

func TestProbeScriptsInvalidJSON(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("scripts", 0755)
	script := `#!/bin/bash
echo 'not valid json'
`
	os.WriteFile("scripts/bad.sh", []byte(script), 0755)

	tools := ProbeScripts(make(map[string]bool), "")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (invalid JSON), got %d", len(tools))
	}
}

func TestProbeScriptsNonZeroExit(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("scripts", 0755)
	script := `#!/bin/bash
exit 1
`
	os.WriteFile("scripts/fail.sh", []byte(script), 0755)

	tools := ProbeScripts(make(map[string]bool), "")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (non-zero exit), got %d", len(tools))
	}
}

func TestProbeScriptsSkipsDirectories(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("scripts/subdir", 0755)

	tools := ProbeScripts(make(map[string]bool), "")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (directories skipped), got %d", len(tools))
	}
}

func TestProbeScriptsMissingFields(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("scripts", 0755)
	// Missing description
	script := `#!/bin/bash
if [ "$1" = "--mcp-metadata" ]; then
    echo '{"name":"incomplete","inputSchema":{"type":"object"}}'
    exit 0
fi
`
	os.WriteFile("scripts/incomplete.sh", []byte(script), 0755)

	tools := ProbeScripts(make(map[string]bool), "")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (missing description), got %d", len(tools))
	}
}

func TestValidateToolConfigNoPlatformsOrHTTPOrTerminal(t *testing.T) {
	tc := ToolConfig{
		Name:        "empty",
		Description: "no execution",
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for tool with no platforms/http/terminal")
	}
}

func TestValidateRejectsMetacharsInCommand(t *testing.T) {
	tc := ToolConfig{
		Name: "bad",
		Platforms: map[string]PlatformConfig{
			"linux": {Command: "echo;rm"},
		},
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for metachar in command")
	}
}

func TestValidateRejectsMetacharsInArgs(t *testing.T) {
	tc := ToolConfig{
		Name: "bad",
		Platforms: map[string]PlatformConfig{
			"linux": {Command: "echo", Args: []string{"hello|world"}},
		},
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for metachar in args")
	}
}

func TestValidateRejectsCommandWithSpaces(t *testing.T) {
	tc := ToolConfig{
		Name: "bad",
		Platforms: map[string]PlatformConfig{
			"linux": {Command: "rm -rf"},
		},
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for spaces in command")
	}
}

func TestValidateRejectsTabInCommand(t *testing.T) {
	tc := ToolConfig{
		Name: "bad",
		Platforms: map[string]PlatformConfig{
			"linux": {Command: "rm\t-rf"},
		},
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for tab in command")
	}
}

func TestValidateAcceptsCleanConfig(t *testing.T) {
	tc := ToolConfig{
		Name: "good",
		Platforms: map[string]PlatformConfig{
			"linux":  {Command: "echo", Args: []string{"hello"}},
			"darwin": {Command: "echo", Args: []string{"hello"}},
		},
	}
	if err := ValidateToolConfig(tc); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAcceptsBinaryPath(t *testing.T) {
	tc := ToolConfig{
		Name: "binary",
		Platforms: map[string]PlatformConfig{
			"linux": {Command: "/usr/local/bin/mytool"},
		},
	}
	if err := ValidateToolConfig(tc); err != nil {
		t.Errorf("unexpected error for binary path: %v", err)
	}
}

func TestValidateChecksAllPlatforms(t *testing.T) {
	tc := ToolConfig{
		Name: "multi",
		Platforms: map[string]PlatformConfig{
			"linux":   {Command: "echo"},
			"windows": {Command: "echo;bad"},
		},
	}
	if err := ValidateToolConfig(tc); err == nil {
		t.Error("expected error for metachar in windows platform")
	}
}

func TestValidateHTTPOnlyToolPasses(t *testing.T) {
	tc := ToolConfig{
		Name: "http_only",
		HTTP: &HTTPConfig{URL: "http://example.com"},
	}
	if err := ValidateToolConfig(tc); err != nil {
		t.Errorf("unexpected error for HTTP-only tool: %v", err)
	}
}

func TestValidateTerminalOnlyToolPasses(t *testing.T) {
	tc := ToolConfig{
		Name:     "term_only",
		Terminal: &TerminalConfig{Enabled: true, Allowlist: []string{"echo"}},
	}
	if err := ValidateToolConfig(tc); err != nil {
		t.Errorf("unexpected error for terminal-only tool: %v", err)
	}
}
