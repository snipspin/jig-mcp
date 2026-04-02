//go:build windows

package rlimit

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

var procQueryInformationJobObject = modkernel32.NewProc("QueryInformationJobObject")

func buildSleepBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "sleep_tool")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "../../testdata/sleep_tool.go")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build sleep binary: %v\n%s", err, out)
	}
	return bin
}

func TestJobObjectAPISequence(t *testing.T) {
	binPath := buildSleepBinary(t)

	cmd := exec.Command(binPath, `{"duration":"30s"}`)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createSuspended,
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer cmd.Process.Kill()

	job, err := createJobObject()
	if err != nil {
		t.Fatalf("createJobObject: %v", err)
	}
	defer syscall.CloseHandle(job)

	wantMem := uintptr(128 * 1024 * 1024)
	if err := setJobResourceLimits(job, wantMem, 0); err != nil {
		t.Fatalf("setJobResourceLimits: %v", err)
	}

	// Verify limits were set correctly by querying them back.
	var info jobObjectExtendedLimitInformation
	var returnLen uint32
	r, _, qErr := procQueryInformationJobObject.Call(
		uintptr(job),
		uintptr(classExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
		uintptr(unsafe.Pointer(&returnLen)),
	)
	if r == 0 {
		t.Fatalf("QueryInformationJobObject: %v", qErr)
	}
	if info.BasicLimitInformation.LimitFlags&jobObjectLimitProcessMemory == 0 {
		t.Error("JOB_OBJECT_LIMIT_PROCESS_MEMORY flag not set")
	}
	if info.ProcessMemoryLimit != wantMem {
		t.Errorf("ProcessMemoryLimit: got %d, want %d", info.ProcessMemoryLimit, wantMem)
	}

	if err := assignToJob(job, cmd.Process.Pid); err != nil {
		t.Fatalf("assignToJob: %v", err)
	}

	if err := resumeProcess(cmd.Process.Pid); err != nil {
		t.Fatalf("resumeProcess: %v", err)
	}

	t.Log("all Job Object API calls succeeded, limits verified via QueryInformationJobObject")
	cmd.Process.Kill()
}

func TestJobObjectStructSizes(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("64-bit only")
	}
	checks := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"basicLimitInfo", unsafe.Sizeof(jobObjectBasicLimitInformation{}), 64},
		{"ioCounters", unsafe.Sizeof(ioCounters{}), 48},
		{"extendedLimitInfo", unsafe.Sizeof(jobObjectExtendedLimitInformation{}), 144},
		{"cpuRateControlInfo", unsafe.Sizeof(jobObjectCpuRateControlInformation{}), 8},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
