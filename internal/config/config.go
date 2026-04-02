// Package config handles YAML manifest loading, script discovery, and tool
// configuration validation for jig-mcp.
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/snipspin/jig-mcp/internal/logging"
	"gopkg.in/yaml.v3"
)

// scriptProbeTimeout is the timeout for probing scripts for MCP metadata.
const scriptProbeTimeout = 5 * time.Second

// PlatformConfig describes the command and arguments for a specific OS.
type PlatformConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// SandboxConfig specifies optional sandbox isolation (e.g. Docker) for a tool.
type SandboxConfig struct {
	Type  string `yaml:"type"`
	Image string `yaml:"image,omitempty"`
}

// ToolConfig holds the full configuration for a single tool, as parsed from
// a manifest.yaml file or auto-discovered from a script.
type ToolConfig struct {
	Name          string                    `yaml:"name"`
	Description   string                    `yaml:"description"`
	InputSchema   map[string]any            `yaml:"inputSchema"`
	Platforms     map[string]PlatformConfig `yaml:"platforms,omitempty"`
	HTTP          *HTTPConfig               `yaml:"http,omitempty"`
	Timeout       string                    `yaml:"timeout,omitempty"`
	MaxMemoryMB   int                       `yaml:"maxMemoryMB,omitempty"`
	MaxCPUPercent int                       `yaml:"maxCPUPercent,omitempty"`
	Sandbox       *SandboxConfig            `yaml:"sandbox,omitempty"`
	Terminal      *TerminalConfig           `yaml:"terminal,omitempty"`
}

// TerminalConfig holds settings for TerminalTool, including the command allowlist.
type TerminalConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Allowlist     []string `yaml:"allowlist"`
	MaxOutputSize int      `yaml:"maxOutputSize,omitempty"`
}

// HTTPConfig holds settings for HTTPTool, including URL prefixes for SSRF prevention.
type HTTPConfig struct {
	URL                string            `yaml:"url,omitempty"`
	Method             string            `yaml:"method,omitempty"`
	Headers            map[string]string `yaml:"headers,omitempty"`
	QueryParams        map[string]string `yaml:"queryParams,omitempty"`
	AllowedURLPrefixes []string          `yaml:"allowedURLPrefixes,omitempty"`
}

// shellMetachars are characters that enable shell injection when present in
// command names or static arguments. We reject these at manifest load time.
var shellMetachars = []string{";", "|", "&", ">", "<", "`", "$(", "${", "\n", "\r"}

// validatePlatformConfig checks a single platform entry for shell metacharacters
// in the command and static args. Returns an error describing the first violation.
func validatePlatformConfig(platform string, pc PlatformConfig) error {
	for _, mc := range shellMetachars {
		if strings.Contains(pc.Command, mc) {
			return fmt.Errorf("platform %s: command %q contains shell metacharacter %q", platform, pc.Command, mc)
		}
	}
	// Reject commands with embedded spaces that look like "cmd arg" compound strings.
	// exec.Command treats the first argument as a literal program path, so a command
	// like "rm -rf /" would fail anyway, but rejecting it early gives a clear error.
	if strings.ContainsAny(pc.Command, " \t") {
		return fmt.Errorf("platform %s: command %q contains whitespace — use args for parameters", platform, pc.Command)
	}
	for i, arg := range pc.Args {
		for _, mc := range shellMetachars {
			if strings.Contains(arg, mc) {
				return fmt.Errorf("platform %s: args[%d] %q contains shell metacharacter %q", platform, i, arg, mc)
			}
		}
	}
	return nil
}

// ValidateToolConfig checks all platform entries in a ToolConfig. Returns an
// error on the first violation found. Tools that fail validation are not loaded.
func ValidateToolConfig(tc ToolConfig) error {
	if tc.Platforms == nil && tc.HTTP == nil && tc.Terminal == nil {
		return fmt.Errorf("tool %q: must specify either platforms, http, or terminal", tc.Name)
	}
	for platform, pc := range tc.Platforms {
		if err := validatePlatformConfig(platform, pc); err != nil {
			return fmt.Errorf("tool %q: %w", tc.Name, err)
		}
	}
	return nil
}

// mcpMetadata is the JSON structure returned by scripts supporting --mcp-metadata.
type mcpMetadata struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// LoadedTool represents a tool ready for registration.
type LoadedTool struct {
	Config ToolConfig
	Type   string // "external", "http", "terminal"
}

// LoadManifests reads tools/*/manifest.yaml and returns valid tool configs.
// If configDir is empty, it looks for tools/ relative to the current working directory.
func LoadManifests(configDir string) []LoadedTool {
	var tools []LoadedTool

	toolsDir := "tools"
	if configDir != "" {
		toolsDir = filepath.Join(configDir, "tools")
	}

	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return tools
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Clean(filepath.Join(toolsDir, entry.Name(), "manifest.yaml"))
		func() {
			f, err := os.Open(manifestPath)
			if err != nil {
				slog.Warn("skipping tool: cannot open manifest", logging.Sanitize("path", manifestPath), slog.Any("err", err))
				return
			}
			defer f.Close()

			var tc ToolConfig
			if err := yaml.NewDecoder(f).Decode(&tc); err != nil {
				slog.Warn("error decoding manifest", logging.Sanitize("path", manifestPath), slog.Any("err", err))
				return
			}

			if err := ValidateToolConfig(tc); err != nil {
				return
			}

			var toolType string
			if tc.HTTP != nil {
				toolType = "http"
			} else if tc.Terminal != nil {
				if !tc.Terminal.Enabled {
					return
				}
				toolType = "terminal"
			} else {
				toolType = "external"
			}
			tools = append(tools, LoadedTool{Config: tc, Type: toolType})
		}()
	}

	return tools
}

// ProbeScripts scans the scripts/ directory and probes each script with --mcp-metadata.
// Scripts that return valid metadata JSON are auto-registered as tools.
// Returns tool configs for scripts that support --mcp-metadata.
// If configDir is empty, it looks for scripts/ relative to the current working directory.
func ProbeScripts(existingNames map[string]bool, configDir string) []LoadedTool {
	var tools []LoadedTool

	scriptsDir := "scripts"
	if configDir != "" {
		scriptsDir = filepath.Join(configDir, "scripts")
	}

	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return tools
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		var cmd *exec.Cmd
		scriptPath := filepath.Join(scriptsDir, name)

		switch ext {
		case ".ps1":
			cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "--mcp-metadata")
		case ".sh":
			cmd = exec.Command("bash", scriptPath, "--mcp-metadata")
		default:
			continue
		}

		// Use a short timeout so a misbehaving script doesn't block startup.
		done := make(chan struct{})
		var out []byte
		var cmdErr error
		go func() {
			out, cmdErr = cmd.CombinedOutput()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(scriptProbeTimeout):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			continue
		}

		if cmdErr != nil {
			continue
		}

		var meta mcpMetadata
		if err := json.Unmarshal(out, &meta); err != nil {
			continue
		}
		if meta.Name == "" || meta.Description == "" || meta.InputSchema == nil {
			continue
		}

		// Don't overwrite tools already loaded from manifests.
		if _, exists := existingNames[meta.Name]; exists {
			continue
		}

		// Build a ToolConfig with platform entries using this script.
		tc := ToolConfig{
			Name:        meta.Name,
			Description: meta.Description,
			InputSchema: meta.InputSchema,
			Platforms:   make(map[string]PlatformConfig),
		}

		switch ext {
		case ".ps1":
			tc.Platforms["windows"] = PlatformConfig{
				Command: "powershell",
				Args:    []string{"-ExecutionPolicy", "Bypass", "-File", scriptPath},
			}
		case ".sh":
			tc.Platforms["linux"] = PlatformConfig{
				Command: "bash",
				Args:    []string{scriptPath},
			}
			tc.Platforms["darwin"] = PlatformConfig{
				Command: "bash",
				Args:    []string{scriptPath},
			}
		}

		tools = append(tools, LoadedTool{Config: tc, Type: "external"})
	}

	return tools
}
