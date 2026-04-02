package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snipspin/jig-mcp/common"
)

func TestLogMaxSizeBytesEnvOverride(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_SIZE_MB", "100")
	got := logMaxSizeBytes()
	want := int64(100 * 1024 * 1024)
	if got != want {
		t.Errorf("logMaxSizeBytes() = %d, want %d", got, want)
	}
}

func TestLogMaxSizeBytesDefault(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_SIZE_MB", "")
	got := logMaxSizeBytes()
	want := int64(50 * 1024 * 1024)
	if got != want {
		t.Errorf("logMaxSizeBytes() = %d, want %d", got, want)
	}
}

func TestLogMaxSizeBytesInvalidFallsBack(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_SIZE_MB", "not_a_number")
	got := logMaxSizeBytes()
	want := int64(50 * 1024 * 1024)
	if got != want {
		t.Errorf("logMaxSizeBytes() = %d, want default %d", got, want)
	}
}

func TestLogMaxSizeBytesZeroFallsBack(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_SIZE_MB", "0")
	got := logMaxSizeBytes()
	want := int64(50 * 1024 * 1024)
	if got != want {
		t.Errorf("logMaxSizeBytes() = %d, want default %d (0 is not > 0)", got, want)
	}
}

func TestLogMaxFilesEnvOverride(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_FILES", "5")
	got := logMaxFiles()
	if got != 5 {
		t.Errorf("logMaxFiles() = %d, want 5", got)
	}
}

func TestLogMaxFilesDefault(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_FILES", "")
	got := logMaxFiles()
	if got != 3 {
		t.Errorf("logMaxFiles() = %d, want 3", got)
	}
}

func TestLogMaxFilesInvalidFallsBack(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_FILES", "abc")
	got := logMaxFiles()
	if got != 3 {
		t.Errorf("logMaxFiles() = %d, want 3", got)
	}
}

type mockTool struct {
	def common.ToolDef
	res map[string]any
}

func (m mockTool) Definition() common.ToolDef     { return m.def }
func (m mockTool) Handle(args map[string]any) any { return m.res }

func TestRedactArguments(t *testing.T) {
	tool := mockTool{
		def: common.ToolDef{
			Name: "test",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"password": map[string]any{
						"type":      "string",
						"sensitive": true,
					},
					"username": map[string]any{
						"type": "string",
					},
				},
			},
		},
	}

	args := map[string]any{
		"password": "secret123",
		"username": "admin",
	}

	redacted := redactArguments(tool, args)
	if redacted["password"] != "[REDACTED]" {
		t.Errorf("expected password to be redacted, got %v", redacted["password"])
	}
	if redacted["username"] != "admin" {
		t.Errorf("expected username to be preserved, got %v", redacted["username"])
	}
	// Original args should not be modified
	if args["password"] != "secret123" {
		t.Error("original args were modified")
	}
}

func TestRedactArgumentsNoProperties(t *testing.T) {
	tool := mockTool{
		def: common.ToolDef{
			Name:        "test",
			InputSchema: map[string]any{"type": "object"},
		},
	}
	args := map[string]any{"key": "value"}
	redacted := redactArguments(tool, args)
	if redacted["key"] != "value" {
		t.Errorf("expected key preserved when no properties defined, got %v", redacted["key"])
	}
}

func TestRedactArgumentsSensitiveFalseNotRedacted(t *testing.T) {
	tool := mockTool{
		def: common.ToolDef{
			Name: "test",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"token": map[string]any{"sensitive": false},
				},
			},
		},
	}
	args := map[string]any{"token": "visible"}
	redacted := redactArguments(tool, args)
	if redacted["token"] != "visible" {
		t.Errorf("sensitive=false should not redact, got %v", redacted["token"])
	}
}

func TestRedactArgumentsExtraArgNotInSchema(t *testing.T) {
	tool := mockTool{
		def: common.ToolDef{
			Name: "test",
			InputSchema: map[string]any{
				"properties": map[string]any{},
			},
		},
	}
	args := map[string]any{"unknown_field": "data"}
	redacted := redactArguments(tool, args)
	if redacted["unknown_field"] != "data" {
		t.Errorf("fields not in schema should be preserved, got %v", redacted["unknown_field"])
	}
}

func TestRotateShiftsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// Create a log file with some content
	os.WriteFile(logPath, []byte("entry1\n"), 0640)

	t.Setenv("JIG_LOG_MAX_FILES", "3")

	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("failed to open root: %v", err)
	}
	defer root.Close()
	w := &auditWriter{logDir: tmpDir, root: root}
	f, _ := root.OpenFile("audit.jsonl", os.O_RDWR, 0640)
	w.file = f

	w.rotate()

	// Original should be rotated to .1
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected %s.1 to exist: %v", logPath, err)
	}
	// Original should be a fresh (empty) file
	data, _ := os.ReadFile(logPath + ".1")
	if string(data) != "entry1\n" {
		t.Errorf("rotated file content = %q, want %q", data, "entry1\n")
	}

	// Fresh file should be open
	if w.file == nil {
		t.Error("expected file to be reopened after rotation")
	}
	w.file.Close()
}

func TestRotateMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")
	t.Setenv("JIG_LOG_MAX_FILES", "2")

	// Simulate 3 rotations to test oldest deletion
	for i := 1; i <= 3; i++ {
		os.WriteFile(logPath, []byte("entry\n"), 0640)
		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatalf("failed to open root: %v", err)
		}
		w := &auditWriter{logDir: tmpDir, root: root}
		f, _ := root.OpenFile("audit.jsonl", os.O_RDWR, 0640)
		w.file = f
		w.rotate()
		w.file.Close()
		root.Close()
	}

	// .1 and .2 should exist, .3 should not (maxFiles=2)
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Error("expected .1 to exist")
	}
	if _, err := os.Stat(logPath + ".2"); err != nil {
		t.Error("expected .2 to exist")
	}
	if _, err := os.Stat(logPath + ".3"); !os.IsNotExist(err) {
		t.Error("expected .3 to NOT exist (exceeds maxFiles)")
	}
}

func TestWarnOnceSuppresses(t *testing.T) {
	w := &auditWriter{}

	// First call should update warnLast
	w.warnOnce(os.ErrClosed)
	if w.warnLast.IsZero() {
		t.Error("expected warnLast to be set")
	}

	first := w.warnLast

	// Second call within a minute should not update
	w.warnOnce(os.ErrClosed)
	if !w.warnLast.Equal(first) {
		t.Error("expected warnLast to remain unchanged within suppression window")
	}
}

func TestWarnOnceResetsAfterWindow(t *testing.T) {
	w := &auditWriter{}
	// Set warnLast to more than a minute ago
	w.warnLast = time.Now().Add(-2 * time.Minute)
	old := w.warnLast

	w.warnOnce(os.ErrClosed)
	if w.warnLast.Equal(old) {
		t.Error("expected warnLast to be updated after suppression window expired")
	}
}

func TestCloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)

	// Write something to open the file
	Log.write([]byte("test\n"))

	Close()
	// Second close should not panic
	Close()

	if Log.file != nil {
		t.Error("expected file to be nil after close")
	}
	if Log.logDir != "" {
		t.Error("expected logDir to be empty after close")
	}
}

func TestWriteLazyOpensFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// File should not exist yet
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("expected log file to not exist before first write")
	}

	Log.write([]byte("first line\n"))

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist after write: %v", err)
	}
	if string(data) != "first line\n" {
		t.Errorf("content = %q, want %q", data, "first line\n")
	}
}

func TestWriteAppendsToFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	Log.write([]byte("line1\n"))
	Log.write([]byte("line2\n"))

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, _ := os.ReadFile(logPath)
	if string(data) != "line1\nline2\n" {
		t.Errorf("content = %q, want %q", data, "line1\nline2\n")
	}
}

func TestWriteReopensOnLogDirChange(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	t.Setenv("JIG_LOG_DIR", tmpDir1)
	Log.write([]byte("first dir\n"))

	// Change log dir
	t.Setenv("JIG_LOG_DIR", tmpDir2)
	Log.write([]byte("second dir\n"))
	defer Close()

	// Verify file in second dir exists
	logPath2 := filepath.Join(tmpDir2, "audit.jsonl")
	data, err := os.ReadFile(logPath2)
	if err != nil {
		t.Fatalf("expected log file in second dir: %v", err)
	}
	if string(data) != "second dir\n" {
		t.Errorf("content = %q, want %q", data, "second dir\n")
	}
}

func TestRecordWithContentAsSliceOfMaps(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	// Test error extraction from []map[string]any content
	tool := mockTool{
		def: common.ToolDef{Name: "test_tool", InputSchema: map[string]any{}},
		res: map[string]any{
			"isError": true,
			"content": []map[string]any{{"type": "text", "text": "error from map"}},
		},
	}

	Record("test_tool", map[string]any{}, 100, tool.res, tool, "caller")
	Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected audit log to exist: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if entry["error"] != "error from map" {
		t.Errorf("expected 'error from map', got %v", entry["error"])
	}
}

func TestRecordWithUnknownError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	// Test error extraction when isError=true but no content
	tool := mockTool{
		def: common.ToolDef{Name: "unknown_error_tool", InputSchema: map[string]any{}},
		res: map[string]any{
			"isError": true,
		},
	}

	Record("unknown_error_tool", map[string]any{}, 100, tool.res, tool, "caller")
	Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected audit log to exist: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if entry["error"] != "unknown error" {
		t.Errorf("expected 'unknown error', got %v", entry["error"])
	}
}

func TestRecordWithInvalidArguments(t *testing.T) {
	// Test that Record handles invalid args gracefully
	tool := mockTool{
		def: common.ToolDef{
			Name: "test",
			InputSchema: map[string]any{
				"properties": "not_a_map", // Invalid schema
			},
		},
		res: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
	}

	// Should not panic
	Record("test", map[string]any{"key": "value"}, 100, tool.res, tool, "caller")
}

// --- Tests for logFilePerms ---

func TestLogFilePermsEnvOverride(t *testing.T) {
	t.Setenv("JIG_LOG_FILE_PERMS", "0600")
	got := logFilePerms()
	want := os.FileMode(0600)
	if got != want {
		t.Errorf("logFilePerms() = %o, want %o", got, want)
	}
}

func TestLogFilePermsDefault(t *testing.T) {
	t.Setenv("JIG_LOG_FILE_PERMS", "")
	got := logFilePerms()
	want := os.FileMode(0640)
	if got != want {
		t.Errorf("logFilePerms() = %o, want %o", got, want)
	}
}

func TestLogFilePermsInvalidFallsBack(t *testing.T) {
	t.Setenv("JIG_LOG_FILE_PERMS", "invalid")
	got := logFilePerms()
	want := os.FileMode(0640)
	if got != want {
		t.Errorf("logFilePerms() = %o, want default %o", got, want)
	}
}

// --- Tests for GetLogDir ---

func TestGetLogDirEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	// Initialize to set logDir
	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	got := GetLogDir()
	if got != tmpDir {
		t.Errorf("GetLogDir() = %q, want %q", got, tmpDir)
	}
}

func TestGetLogDirDefault(t *testing.T) {
	t.Setenv("JIG_LOG_DIR", "")
	got := GetLogDir()
	want := "logs"
	if got != want {
		t.Errorf("GetLogDir() = %q, want %q", got, want)
	}
}

// --- Tests for Init error cases ---

func TestInitRejectsDotDotInLogDir(t *testing.T) {
	t.Setenv("JIG_LOG_DIR", "../logs")
	err := Init()
	if err == nil {
		t.Error("Init should fail when JIG_LOG_DIR contains '..'")
	}
	if !strings.Contains(err.Error(), "..") {
		t.Errorf("error should mention '..', got: %v", err)
	}
}

func TestInitFailsWhenLogDirIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "logfile")
	// Create a file instead of directory
	if err := os.WriteFile(logFile, []byte("test"), 0640); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	t.Setenv("JIG_LOG_DIR", logFile)
	err := Init()
	if err == nil {
		t.Error("Init should fail when JIG_LOG_DIR is a file")
	}
	// Error should mention it's not a directory
	if !strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "actually a file") {
		t.Errorf("error should mention directory issue, got: %v", err)
	}
}

func TestInitValidatesLogMaxSizeMB(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_SIZE_MB", "not_a_number")
	err := Init()
	if err == nil {
		t.Error("Init should fail with invalid JIG_LOG_MAX_SIZE_MB")
	}
	if !strings.Contains(err.Error(), "JIG_LOG_MAX_SIZE_MB") {
		t.Errorf("error should mention JIG_LOG_MAX_SIZE_MB, got: %v", err)
	}
}

func TestInitValidatesLogMaxFiles(t *testing.T) {
	t.Setenv("JIG_LOG_MAX_FILES", "not_a_number")
	err := Init()
	if err == nil {
		t.Error("Init should fail with invalid JIG_LOG_MAX_FILES")
	}
	if !strings.Contains(err.Error(), "JIG_LOG_MAX_FILES") {
		t.Errorf("error should mention JIG_LOG_MAX_FILES, got: %v", err)
	}
}

func TestInitValidatesLogFilePerms(t *testing.T) {
	t.Setenv("JIG_LOG_FILE_PERMS", "not_octal")
	err := Init()
	if err == nil {
		t.Error("Init should fail with invalid JIG_LOG_FILE_PERMS")
	}
	if !strings.Contains(err.Error(), "JIG_LOG_FILE_PERMS") {
		t.Errorf("error should mention JIG_LOG_FILE_PERMS, got: %v", err)
	}
}

// --- Tests for Record edge cases ---

func TestRecordWithNonMapResult(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	tool := mockTool{
		def: common.ToolDef{Name: "test", InputSchema: map[string]any{}},
		res:  map[string]any{"content": "string_not_map"}, // Content is string, not []map
	}

	// Should not panic - error extraction should handle this
	Record("test", map[string]any{}, 100, tool.res, tool, "caller")
}

func TestRecordWithNilContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	tool := mockTool{
		def: common.ToolDef{Name: "nil_content_tool", InputSchema: map[string]any{}},
		res: map[string]any{"isError": true}, // No content field
	}

	Record("nil_content_tool", map[string]any{}, 100, tool.res, tool, "caller")
	Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected audit log to exist: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if entry["error"] != "unknown error" {
		t.Errorf("expected 'unknown error' for nil content, got %v", entry["error"])
	}
}

func TestRecordWithEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JIG_LOG_DIR", tmpDir)
	defer Close()

	tool := mockTool{
		def: common.ToolDef{Name: "empty_content_tool", InputSchema: map[string]any{}},
		res: map[string]any{
			"isError": true,
			"content": []any{}, // Empty content
		},
	}

	Record("empty_content_tool", map[string]any{}, 100, tool.res, tool, "caller")
	Close()

	logPath := filepath.Join(tmpDir, "audit.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected audit log to exist: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if entry["error"] != "unknown error" {
		t.Errorf("expected 'unknown error' for empty content, got %v", entry["error"])
	}
}
