package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/snipspin/jig-mcp/internal/rlimit"
)

// defaultTerminalMaxOutput is the default max output size for terminal tool (100 KB).
const defaultTerminalMaxOutput = 100 * 1024

// TerminalTool wraps shell command execution with a configurable command allowlist
// for security. Only commands matching an allowed prefix are executed.
type TerminalTool struct {
	BaseTool
}

// Handle validates the command against the allowlist and executes it in a shell.
func (t TerminalTool) Handle(args map[string]any) any {
	cmdInput, _ := args["command"].(string)
	if cmdInput == "" {
		return errorResult("missing mandatory parameter: command")
	}

	// Safety check: is it in the allowlist?
	allowlist := t.Config.Terminal.Allowlist
	allowed := false
	for _, prefix := range allowlist {
		if cmdInput == prefix || strings.HasPrefix(cmdInput, prefix+" ") {
			allowed = true
			break
		}
	}
	if !allowed {
		return errorResult(fmt.Sprintf("command security check failed: %q doesn't match any allowed prefix", cmdInput))
	}

	// Prepare shell command
	var executable string
	var shellArgs []string
	if runtime.GOOS == "windows" {
		executable = "powershell"
		shellArgs = []string{"-Command", cmdInput}
	} else {
		executable = "bash"
		shellArgs = []string{"-c", cmdInput}
	}

	timeout := t.effectiveTimeout(DefaultGlobalTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, shellArgs...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	// Apply resource limits if configured
	limits := rlimit.EffectiveLimits(t.Config.MaxMemoryMB, t.Config.MaxCPUPercent)
	cleanup, startErr := rlimit.StartWithLimits(cmd, limits, int(timeout.Seconds()))
	if startErr != nil {
		return errorResult(fmt.Sprintf("failed to start terminal command: %v", startErr))
	}
	defer cleanup()

	err := cmd.Wait()
	out := combined.Bytes()

	// Truncate output if necessary
	maxSize := t.Config.Terminal.MaxOutputSize
	if maxSize <= 0 {
		maxSize = defaultTerminalMaxOutput
	}
	if len(out) > maxSize {
		out = append(out[:maxSize], []byte("\n[OUTPUT TRUNCATED]")...)
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errorResult(fmt.Sprintf("command timeout: %q killed after %s", cmdInput, timeout))
		}
		return errorResult(fmt.Sprintf("execution error: %v\nOutput: %s", err, string(out)))
	}

	return textResult(string(out))
}
