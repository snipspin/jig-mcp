//go:build windows

package rlimit

import (
	"fmt"
	"log/slog"
	"os/exec"
	"syscall"
	"unsafe"
)

// Windows Job Object API via kernel32.
var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = modkernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = modkernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = modkernel32.NewProc("AssignProcessToJobObject")

	modntdll            = syscall.NewLazyDLL("ntdll.dll")
	procNtResumeProcess = modntdll.NewProc("NtResumeProcess")
)

// Job Object information classes.
const (
	classExtendedLimitInformation  = 9
	classCpuRateControlInformation = 15
)

// Limit flags for JOBOBJECT_BASIC_LIMIT_INFORMATION.LimitFlags.
const (
	jobObjectLimitJobTime        = 0x00000004
	jobObjectLimitProcessMemory  = 0x00000100
	jobObjectLimitKillOnJobClose = 0x00002000
)

// CPU rate control flags.
const (
	jobObjectCpuRateControlEnable  = 0x1
	jobObjectCpuRateControlHardCap = 0x4
)

// Process creation and access flags.
const (
	createSuspended      = 0x00000004
	processSetQuota      = 0x0100
	processTerminate     = 0x0001
	processSuspendResume = 0x0800
)

// ---- Windows structs (64-bit layout, fields naturally aligned) ----

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	_                       uint32 // padding
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	_                       uint32 // padding
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type jobObjectCpuRateControlInformation struct {
	ControlFlags uint32
	Value        uint32
}

// ---- helpers ----

func createJobObject() (syscall.Handle, error) {
	r, _, err := procCreateJobObjectW.Call(0, 0)
	if r == 0 {
		return syscall.InvalidHandle, fmt.Errorf("CreateJobObject: %w", err)
	}
	return syscall.Handle(r), nil
}

func setJobResourceLimits(job syscall.Handle, memoryBytes uintptr, cpuTimeLimit100ns int64) error {
	info := jobObjectExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitProcessMemory | jobObjectLimitKillOnJobClose
	info.ProcessMemoryLimit = memoryBytes

	if cpuTimeLimit100ns > 0 {
		info.BasicLimitInformation.LimitFlags |= jobObjectLimitJobTime
		info.BasicLimitInformation.PerJobUserTimeLimit = cpuTimeLimit100ns
	}

	r, _, err := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(classExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if r == 0 {
		return fmt.Errorf("SetInformationJobObject (resource limits): %w", err)
	}
	return nil
}

func setJobCpuRate(job syscall.Handle, cpuPercent int) error {
	// CpuRate is in hundredths of a percent (1-10000).
	rate := uint32(cpuPercent * 100)
	if rate < 1 {
		rate = 1
	}
	if rate > 10000 {
		rate = 10000
	}
	info := jobObjectCpuRateControlInformation{
		ControlFlags: jobObjectCpuRateControlEnable | jobObjectCpuRateControlHardCap,
		Value:        rate,
	}
	r, _, err := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(classCpuRateControlInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if r == 0 {
		return fmt.Errorf("SetInformationJobObject (CPU rate): %w", err)
	}
	return nil
}

func assignToJob(job syscall.Handle, pid int) error {
	procHandle, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer syscall.CloseHandle(procHandle)

	r, _, callErr := procAssignProcessToJobObject.Call(uintptr(job), uintptr(procHandle))
	if r == 0 {
		return fmt.Errorf("AssignProcessToJobObject: %w", callErr)
	}
	return nil
}

func resumeProcess(pid int) error {
	handle, err := syscall.OpenProcess(processSuspendResume, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess for resume(%d): %w", pid, err)
	}
	defer syscall.CloseHandle(handle)

	r, _, callErr := procNtResumeProcess.Call(uintptr(handle))
	if r != 0 { // NTSTATUS: 0 = STATUS_SUCCESS
		return fmt.Errorf("NtResumeProcess: NTSTATUS 0x%x: %w", r, callErr)
	}
	return nil
}

// StartWithLimits starts cmd suspended, applies resource limits via a Windows
// Job Object, then resumes the process. This eliminates the race between
// process start and limit application. The returned cleanup function closes the
// job handle (which also kills surviving child processes via KILL_ON_JOB_CLOSE).
func StartWithLimits(cmd *exec.Cmd, limits ResourceLimits, timeoutSec int) (func(), error) {
	// Start the process suspended so limits are applied before it runs.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createSuspended

	if err := cmd.Start(); err != nil {
		return func() {}, err
	}

	pid := cmd.Process.Pid

	// Create and configure the Job Object.
	job, err := createJobObject()
	if err != nil {
		slog.Warn("could not create job object", "err", err)
		if rErr := resumeProcess(pid); rErr != nil {
			slog.Warn("could not resume process", "err", rErr)
		}
		return func() {}, nil
	}

	cleanup := func() { syscall.CloseHandle(job) }

	memBytes := uintptr(limits.MaxMemoryMB) * 1024 * 1024
	// Calculate CPU user time limit: timeoutSec * (MaxCPUPercent / 100).
	// Result is converted to 100-nanosecond ticks.
	cpuSec := float64(timeoutSec) * float64(limits.MaxCPUPercent) / 100.0
	if cpuSec < 1.0 {
		cpuSec = 1.0
	}
	cpuTime100ns := int64(cpuSec * 10000000)

	if err := setJobResourceLimits(job, memBytes, cpuTime100ns); err != nil {
		slog.Warn("could not set resource limits", "err", err)
	}

	if err := setJobCpuRate(job, limits.MaxCPUPercent); err != nil {
		// CPU rate control may not be available on all Windows editions.
		slog.Warn("could not set CPU rate", "err", err)
	}

	// Assign process to job, then resume.
	if err := assignToJob(job, pid); err != nil {
		slog.Warn("could not assign process to job", "err", err)
	}

	if err := resumeProcess(pid); err != nil {
		slog.Warn("could not resume process", "err", err)
	}

	return cleanup, nil
}
