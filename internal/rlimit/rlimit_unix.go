//go:build linux || darwin

package rlimit

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"
)

// rlimitMu serialises setrlimit/start/restore so child processes inherit the
// correct limits without racing. The lock is held only for the fork; actual
// execution runs concurrently after Start returns.
//
// RACE WINDOW NOTE (Go 1.25):
// This implementation uses a server-side setrlimit/start/restore pattern because
// syscall.SysProcAttr does not yet support the Rlimit field (proposed for future
// Go versions). Between setrlimit and cmd.Start(), the server process briefly runs
// under the child's restrictive limits for ~1-10 microseconds (fork latency).
//
// The mutex serializes concurrent tool starts and minimizes this window. This is
// acceptable for the target use case (homelab automation). For production multi-tenant
// isolation, the Windows Job Object approach (which suspends the child before
// applying limits) is preferred.
//
// MEMORY LIMIT IMPLEMENTATION:
// We use RLIMIT_DATA instead of RLIMIT_AS. RLIMIT_AS limits total virtual address
// space, which breaks modern runtimes (Go, Java, V8) that reserve large VA regions
// upfront. On Linux 4.7+, RLIMIT_DATA limits actual memory allocations (brk + anon
// mmap) without interfering with VA space reservations. On older kernels and macOS,
// RLIMIT_DATA only limits the data segment (brk), so memory enforcement may be
// incomplete — cgroups v2 is the proper solution for strict isolation.
var rlimitMu sync.Mutex

// StartWithLimits applies resource limits to cmd and starts it.
// Returns a cleanup function (no-op on unix) and any error from Start.
func StartWithLimits(cmd *exec.Cmd, limits ResourceLimits, timeoutSec int) (func(), error) {
	if limits.MaxMemoryMB < 0 {
		return func() {}, fmt.Errorf("MaxMemoryMB cannot be negative: %d", limits.MaxMemoryMB)
	}
	memBytes := uint64(limits.MaxMemoryMB) * 1024 * 1024
	cpuSec := uint64(float64(timeoutSec) * float64(limits.MaxCPUPercent) / 100.0)
	if cpuSec < 1 {
		cpuSec = 1
	}

	rlimitMu.Lock()
	defer rlimitMu.Unlock()

	// Save current limits.
	var oldData, oldCPU syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_DATA, &oldData); err != nil {
		slog.Warn("getrlimit RLIMIT_DATA failed", "err", err)
	}
	if err := syscall.Getrlimit(syscall.RLIMIT_CPU, &oldCPU); err != nil {
		slog.Warn("getrlimit RLIMIT_CPU failed", "err", err)
	}

	// Apply limits — child inherits them at fork.
	// This briefly exposes the server to the child's resource limits (see race window note above).
	_ = syscall.Setrlimit(syscall.RLIMIT_DATA, &syscall.Rlimit{Cur: memBytes, Max: memBytes})
	_ = syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Cur: cpuSec, Max: cpuSec})

	err := cmd.Start()

	// Restore immediately — the child already has its own copy.
	_ = syscall.Setrlimit(syscall.RLIMIT_DATA, &oldData)
	_ = syscall.Setrlimit(syscall.RLIMIT_CPU, &oldCPU)

	return func() {}, err
}
