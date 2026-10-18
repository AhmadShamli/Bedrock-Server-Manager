//go:build windows

package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func readHostMemory() (totalBytes int64, usedBytes int64, percent float64) {
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))

	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if r == 0 {
		return 0, 0, 0
	}

	totalBytes = int64(mem.TotalPhys)
	availBytes := int64(mem.AvailPhys)
	usedBytes = totalBytes - availBytes
	if usedBytes < 0 {
		usedBytes = 0
	}
	if totalBytes > 0 {
		percent = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}
	return
}

func readHostLoadAvg() (l1, l5, l15 float64) {
	return 0, 0, 0
}

func readRawCPUStat() (total, idle uint64, ok bool) {
	var idleTime, kernelTime, userTime windows.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if r == 0 {
		return 0, 0, false
	}

	filetimeToUint64 := func(ft windows.Filetime) uint64 {
		return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
	}

	idleTicks := filetimeToUint64(idleTime)
	kernelTicks := filetimeToUint64(kernelTime)
	userTicks := filetimeToUint64(userTime)

	// In Windows GetSystemTimes, kernelTime includes idleTime.
	totalTicks := kernelTicks + userTicks
	return totalTicks, idleTicks, true
}
