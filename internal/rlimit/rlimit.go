// Package rlimit provides platform-specific resource limit enforcement (CPU and
// memory) for child processes spawned by jig-mcp tools.
package rlimit

// ResourceLimits holds the effective resource constraints for a tool process.
type ResourceLimits struct {
	MaxMemoryMB   int
	MaxCPUPercent int
}

const (
	DefaultMaxMemoryMB   = 512
	DefaultMaxCPUPercent = 90
)

// EffectiveLimits returns resource limits for a tool, applying defaults for
// any value that is zero or negative.
func EffectiveLimits(maxMemoryMB, maxCPUPercent int) ResourceLimits {
	mem := maxMemoryMB
	if mem <= 0 {
		mem = DefaultMaxMemoryMB
	}
	cpu := maxCPUPercent
	if cpu <= 0 {
		cpu = DefaultMaxCPUPercent
	}
	return ResourceLimits{MaxMemoryMB: mem, MaxCPUPercent: cpu}
}
