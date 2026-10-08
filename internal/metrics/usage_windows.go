//go:build windows

package metrics

import (
	"syscall"
	"unsafe"
)

var (
	getProcessTimes      = syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessTimes")
	getProcessMemoryInfo = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
)

type memoryCounters struct {
	Size                    uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uintptr
	WorkingSetSize          uintptr
	QuotaPeakPagedPoolUsage uintptr
	QuotaPagedPoolUsage     uintptr
	QuotaPeakNonPagedUsage  uintptr
	QuotaNonPagedUsage      uintptr
	PagefileUsage           uintptr
	PeakPagefileUsage       uintptr
}

func processUsage() (float64, uint64, bool) {
	handle := ^uintptr(0) // GetCurrentProcess pseudo-handle.
	var creation, exit, kernel, user syscall.Filetime
	ok, _, _ := getProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return 0, 0, false
	}
	ticks := func(t syscall.Filetime) uint64 { return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime) }
	memory := memoryCounters{}
	memory.Size = uint32(unsafe.Sizeof(memory))
	ok, _, _ = getProcessMemoryInfo.Call(handle, uintptr(unsafe.Pointer(&memory)), uintptr(memory.Size))
	if ok == 0 {
		return 0, 0, false
	}
	return float64(ticks(kernel)+ticks(user)) / 1e7, uint64(memory.PeakWorkingSetSize), true
}
