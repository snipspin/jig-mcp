//go:build linux || darwin

package rlimit

import (
	"os/exec"
	"testing"
)

func TestStartWithLimits(t *testing.T) {
	limits := ResourceLimits{
		MaxMemoryMB:   256,
		MaxCPUPercent: 50,
	}

	cmd := exec.Command("echo", "hello")
	cleanup, err := StartWithLimits(cmd, limits, 30)
	if err != nil {
		t.Fatalf("StartWithLimits failed: %v", err)
	}
	defer cleanup()

	if err := cmd.Wait(); err != nil {
		t.Errorf("command failed: %v", err)
	}
}

func TestStartWithLimits_VeryLowCPU(t *testing.T) {
	limits := ResourceLimits{
		MaxMemoryMB:   64,
		MaxCPUPercent: 1, // Should be clamped to 1 second minimum
	}

	cmd := exec.Command("echo", "test")
	cleanup, err := StartWithLimits(cmd, limits, 1)
	if err != nil {
		t.Fatalf("StartWithLimits failed: %v", err)
	}
	defer cleanup()

	if err := cmd.Wait(); err != nil {
		t.Errorf("command failed: %v", err)
	}
}

func TestStartWithLimits_NegativeMemory(t *testing.T) {
	limits := ResourceLimits{
		MaxMemoryMB:   -1,
		MaxCPUPercent: 50,
	}

	cmd := exec.Command("echo", "test")
	_, err := StartWithLimits(cmd, limits, 30)
	if err == nil {
		t.Fatal("expected error for negative MaxMemoryMB, got nil")
	}
	expectedMsg := "MaxMemoryMB cannot be negative: -1"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}
