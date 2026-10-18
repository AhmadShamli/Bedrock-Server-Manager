//go:build !linux && !windows

package api

import "runtime"

func readHostMemory() (totalBytes int64, usedBytes int64, percent float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys), int64(m.Alloc), float64(m.Alloc) / float64(m.Sys) * 100.0
}

func readHostLoadAvg() (l1, l5, l15 float64) {
	return 0, 0, 0
}

func readRawCPUStat() (total, idle uint64, ok bool) {
	return 0, 0, false
}
