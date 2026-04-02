package logging

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
		err   bool
	}{
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"invalid", slog.LevelInfo, true},
		{"  info  ", slog.LevelInfo, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("parseLogLevel(%q): err=%v, wantErr=%v", tt.input, err, tt.err)
			}
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInitLogging(t *testing.T) {
	// We can't easily check the global logger's level without re-creating handlers,
	// but we can call it with various env vars to ensure it doesn't panic
	// and covers the code paths.

	tests := []struct {
		level string
	}{
		{"debug"},
		{"info"},
		{"warn"},
		{"error"},
		{"invalid"},
		{""},
	}

	for _, tt := range tests {
		t.Run("level="+tt.level, func(t *testing.T) {
			t.Setenv("JIG_LOG_LEVEL", tt.level)
			InitLogging()
		})
	}
}

// --- Tests for Sanitize ---

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no special chars", "hello world", "hello world"},
		{"single newline", "hello\nworld", "helloworld"},
		{"single carriage return", "hello\rworld", "helloworld"},
		{"both newline and CR", "hello\n\rworld", "helloworld"},
		{"multiple newlines", "a\nb\nc", "abc"},
		{"leading newline", "\ntest", "test"},
		{"trailing newline", "test\n", "test"},
		{"empty string", "", ""},
		{"only newlines", "\n\n", ""},
		{"mixed with spaces", "log\ninjection\rattack", "loginjectionattack"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := Sanitize("key", tt.input)
			if attr.Value.String() != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, attr.Value.String(), tt.want)
			}
		})
	}
}

func TestSanitizePreservesKey(t *testing.T) {
	attr := Sanitize("mykey", "value")
	if attr.Key != "mykey" {
		t.Errorf("Sanitize key = %q, want %q", attr.Key, "mykey")
	}
}

// --- Tests for Untaint ---

func TestUntaintInt(t *testing.T) {
	input := 42
	got := Untaint(input)
	if got != 42 {
		t.Errorf("Untaint(42) = %d, want 42", got)
	}
}

func TestUntaintString(t *testing.T) {
	input := "test"
	got := Untaint(input)
	if got != "test" {
		t.Errorf("Untaint(%q) = %q, want %q", input, got, input)
	}
}

func TestUntaintDuration(t *testing.T) {
	input := 5 * 60 // 5 minutes in seconds
	got := Untaint(input)
	if got != 300 {
		t.Errorf("Untaint(300) = %d, want 300", got)
	}
}

func TestUntaintBool(t *testing.T) {
	input := true
	got := Untaint(input)
	if got != true {
		t.Errorf("Untaint(true) = %v, want true", got)
	}
}
