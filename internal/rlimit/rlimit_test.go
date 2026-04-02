package rlimit

import "testing"

func TestEffectiveLimitsDefaults(t *testing.T) {
	l := EffectiveLimits(0, 0)
	if l.MaxMemoryMB != DefaultMaxMemoryMB {
		t.Errorf("MaxMemoryMB = %d, want %d", l.MaxMemoryMB, DefaultMaxMemoryMB)
	}
	if l.MaxCPUPercent != DefaultMaxCPUPercent {
		t.Errorf("MaxCPUPercent = %d, want %d", l.MaxCPUPercent, DefaultMaxCPUPercent)
	}
}

func TestEffectiveLimitsNegativeUsesDefaults(t *testing.T) {
	l := EffectiveLimits(-1, -50)
	if l.MaxMemoryMB != DefaultMaxMemoryMB {
		t.Errorf("MaxMemoryMB = %d, want default %d", l.MaxMemoryMB, DefaultMaxMemoryMB)
	}
	if l.MaxCPUPercent != DefaultMaxCPUPercent {
		t.Errorf("MaxCPUPercent = %d, want default %d", l.MaxCPUPercent, DefaultMaxCPUPercent)
	}
}

func TestEffectiveLimitsPositivePreserved(t *testing.T) {
	l := EffectiveLimits(256, 50)
	if l.MaxMemoryMB != 256 {
		t.Errorf("MaxMemoryMB = %d, want 256", l.MaxMemoryMB)
	}
	if l.MaxCPUPercent != 50 {
		t.Errorf("MaxCPUPercent = %d, want 50", l.MaxCPUPercent)
	}
}

func TestEffectiveLimitsPartialOverride(t *testing.T) {
	l := EffectiveLimits(128, 0) // memory set, CPU default
	if l.MaxMemoryMB != 128 {
		t.Errorf("MaxMemoryMB = %d, want 128", l.MaxMemoryMB)
	}
	if l.MaxCPUPercent != DefaultMaxCPUPercent {
		t.Errorf("MaxCPUPercent = %d, want default %d", l.MaxCPUPercent, DefaultMaxCPUPercent)
	}
}

func TestEffectiveLimitsLargeValues(t *testing.T) {
	l := EffectiveLimits(16384, 200)
	if l.MaxMemoryMB != 16384 {
		t.Errorf("MaxMemoryMB = %d, want 16384", l.MaxMemoryMB)
	}
	if l.MaxCPUPercent != 200 {
		t.Errorf("MaxCPUPercent = %d, want 200", l.MaxCPUPercent)
	}
}
