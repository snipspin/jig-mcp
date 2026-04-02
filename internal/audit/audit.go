// Package audit provides JSONL audit logging with log rotation and sensitive
// field redaction for jig-mcp tool executions.
package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/logging"
)

// defaultAuditFilePerms is used when JIG_LOG_FILE_PERMS is not set.
const defaultAuditFilePerms = 0640

// Log is the singleton audit log writer. It is safe for concurrent use.
var Log auditWriter

type auditWriter struct {
	mu       sync.Mutex
	file     *os.File
	root     *os.Root // root directory for audit logs
	logDir   string   // path of the configured log directory
	warnLast time.Time
}

// logMaxSizeBytes returns the max audit log size before rotation.
func logMaxSizeBytes() int64 {
	if v := os.Getenv("JIG_LOG_MAX_SIZE_MB"); v != "" {
		if mb, err := strconv.ParseInt(v, 10, 64); err == nil && mb > 0 {
			return mb * 1024 * 1024
		}
	}
	return 50 * 1024 * 1024 // default 50 MB
}

// logMaxFiles returns the maximum number of rotated audit log files to keep.
func logMaxFiles() int {
	if v := os.Getenv("JIG_LOG_MAX_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 3
}

// logFilePerms returns the configured file permissions for audit logs.
func logFilePerms() os.FileMode {
	if v := os.Getenv("JIG_LOG_FILE_PERMS"); v != "" {
		if p, err := strconv.ParseUint(v, 8, 32); err == nil {
			return os.FileMode(p)
		}
	}
	return defaultAuditFilePerms
}

// Init initializes the audit log system and validates the log directory and settings.
// It fails fast if the directory cannot be created or accessed, or if settings are invalid.
func Init() error {
	// Validate log size and max files settings
	if v := os.Getenv("JIG_LOG_MAX_SIZE_MB"); v != "" {
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return fmt.Errorf("invalid JIG_LOG_MAX_SIZE_MB: %v", err)
		}
	}
	if v := os.Getenv("JIG_LOG_MAX_FILES"); v != "" {
		if _, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("invalid JIG_LOG_MAX_FILES: %v", err)
		}
	}
	if v := os.Getenv("JIG_LOG_FILE_PERMS"); v != "" {
		if _, err := strconv.ParseUint(v, 8, 32); err != nil {
			return fmt.Errorf("invalid JIG_LOG_FILE_PERMS: %v", err)
		}
	}

	logDir := os.Getenv("JIG_LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	logDir = filepath.Clean(logDir)
	if strings.Contains(logDir, "..") {
		return fmt.Errorf("JIG_LOG_DIR contains '..': %s", logDir)
	}

	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory %q: %w", logDir, err)
	}

	// Verify it's actually a directory
	info, err := os.Stat(logDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("log directory %q is actually a file", logDir)
	}

	root, err := os.OpenRoot(logDir)
	if err != nil {
		return fmt.Errorf("failed to open root log directory %q: %w", logDir, err)
	}

	// Try to open/create the file to ensure write access.
	f, err := root.OpenFile("audit.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFilePerms())
	if err != nil {
		root.Close()
		return fmt.Errorf("failed to open audit.jsonl in %q: %w", logDir, err)
	}

	Log.mu.Lock()
	defer Log.mu.Unlock()
	Log.root = root
	Log.file = f
	Log.logDir = logDir

	return nil
}

// GetLogDir returns the validated log directory path.
func GetLogDir() string {
	Log.mu.Lock()
	defer Log.mu.Unlock()
	if Log.logDir != "" {
		return Log.logDir
	}
	logDir := os.Getenv("JIG_LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	return filepath.Clean(logDir)
}

// rotate closes the current file, shifts existing rotated files, and reopens
// a fresh log. Caller must hold w.mu.
func (w *auditWriter) rotate() {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			slog.Error("failed to close audit log file before rotation", "err", err)
		}
		w.file = nil
	}

	if w.root == nil {
		return
	}

	maxFiles := logMaxFiles()
	base := "audit.jsonl"

	// Remove the oldest rotated file if it exists.
	oldest := base + "." + strconv.Itoa(maxFiles)
	if err := w.root.Remove(oldest); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to remove old rotated log", "path", oldest, "err", err)
	}

	// Shift .N-1 → .N, .N-2 → .N-1, ... , .1 → .2
	for i := maxFiles - 1; i >= 1; i-- {
		src := base + "." + strconv.Itoa(i)
		dst := base + "." + strconv.Itoa(i+1)
		if err := w.root.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to rotate log file", "src", src, "dst", dst, "err", err)
		}
	}

	// Current file → .1
	if err := w.root.Rename(base, base+".1"); err != nil {
		slog.Warn("failed to rotate current log", "path", base, "err", err)
	}

	// Open a fresh file.
	f, err := w.root.OpenFile(base, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFilePerms())
	if err != nil {
		slog.Error("failed to open audit log after rotation", "path", base, "err", err)
		return
	}
	w.file = f
}

// write serializes a single audit line to the log file. It lazily opens the
// file when the target path changes (or on first call). If the file exceeds
// the configured size limit, it is rotated before writing.
func (w *auditWriter) write(line []byte) {
	logDir := os.Getenv("JIG_LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	logDir = filepath.Clean(logDir)
	if strings.Contains(logDir, "..") {
		slog.Error("JIG_LOG_DIR cannot contain '..'", logging.Sanitize("dir", logDir))
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// (Re)open if the target path changed (e.g. tests override JIG_LOG_DIR)
	// or if the file hasn't been opened yet.
	if w.file == nil || w.logDir != logDir {
		if w.file != nil {
			if err := w.file.Close(); err != nil {
				slog.Error("failed to close audit log file on dir change", "err", err)
			}
		}
		if w.root != nil {
			if err := w.root.Close(); err != nil {
				slog.Error("failed to close root log directory", "err", err)
			}
		}

		if err := os.MkdirAll(logDir, 0750); err != nil {
			slog.Error("failed to create log directory", logging.Sanitize("dir", logDir), slog.Any("err", err))
			return
		}
		root, err := os.OpenRoot(logDir)
		if err != nil {
			slog.Error("failed to open root log directory", logging.Sanitize("dir", logDir), slog.Any("err", err))
			return
		}

		f, err := root.OpenFile("audit.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFilePerms())
		if err != nil {
			slog.Error("failed to open audit log", "path", "audit.jsonl", "err", err)
			if closeErr := root.Close(); closeErr != nil {
				slog.Error("failed to close root log directory after open failure", "err", closeErr)
			}
			return
		}
		w.root = root
		w.file = f
		w.logDir = logDir
	}

	// Rotate if the file exceeds the size limit.
	if info, err := w.file.Stat(); err == nil && info.Size() >= logMaxSizeBytes() {
		w.rotate()
	}

	if _, err := w.file.Write(line); err != nil {
		w.warnOnce(err)
	}
}

// warnOnce emits a write-error warning at most once per minute. Caller must
// hold w.mu.
func (w *auditWriter) warnOnce(err error) {
	if time.Since(w.warnLast) < time.Minute {
		return
	}
	w.warnLast = time.Now()
	slog.Error("audit log write failed (suppressing further warnings for 1m)", "err", err)
}

// Close flushes and closes the audit log file. Called during graceful
// shutdown.
func Close() {
	Log.mu.Lock()
	defer Log.mu.Unlock()
	if Log.file != nil {
		if err := Log.file.Close(); err != nil {
			slog.Error("failed to close audit log file on shutdown", "err", err)
		}
		Log.file = nil
	}
	if Log.root != nil {
		if err := Log.root.Close(); err != nil {
			slog.Error("failed to close root log directory on shutdown", "err", err)
		}
		Log.root = nil
	}
	Log.logDir = ""
}

// redactArguments redacts sensitive fields from the arguments map.
func redactArguments(tool common.Tool, args map[string]any) map[string]any {
	redactedArgs := make(map[string]any)
	for k, v := range args {
		redactedArgs[k] = v
	}

	def := tool.Definition()
	properties, ok := def.InputSchema["properties"].(map[string]any)
	if !ok {
		return redactedArgs
	}

	for k := range redactedArgs {
		prop, ok := properties[k].(map[string]any)
		if !ok {
			continue
		}
		if sensitive, ok := prop["sensitive"].(bool); ok && sensitive {
			redactedArgs[k] = "[REDACTED]"
		}
	}
	return redactedArgs
}

// Record writes an audit log entry for a tool execution.
func Record(toolName string, args map[string]any, duration time.Duration, result any, tool common.Tool, callerName string) {
	success := true
	errorMessage := ""
	if resMap, ok := result.(map[string]any); ok {
		if isErr, ok := resMap["isError"].(bool); ok && isErr {
			success = false
			// Try to extract error message from content
			if content, ok := resMap["content"].([]any); ok && len(content) > 0 {
				if item, ok := content[0].(map[string]any); ok {
					if text, ok := item["text"].(string); ok {
						errorMessage = text
					}
				}
			} else if content, ok := resMap["content"].([]map[string]any); ok && len(content) > 0 {
				if text, ok := content[0]["text"].(string); ok {
					errorMessage = text
				}
			}
		}
	}

	redactedArgs := redactArguments(tool, args)

	entry := map[string]any{
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"tool":        toolName,
		"caller":      callerName,
		"arguments":   redactedArgs,
		"duration_ms": duration.Milliseconds(),
		"success":     success,
	}
	if errorMessage != "" {
		entry["error"] = errorMessage
	} else if !success {
		entry["error"] = "unknown error"
	}

	line, err := json.Marshal(entry)
	if err != nil {
		slog.Error("failed to marshal audit log entry", "error", err, "tool", toolName)
		return
	}
	Log.write(append(line, '\n'))
}
