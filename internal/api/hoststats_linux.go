//go:build linux

package api

import (
	"os"
	"strconv"
	"strings"
)

// readHostMemory extracts real physical memory metrics on Linux from /proc/meminfo.
func readHostMemory() (totalBytes int64, usedBytes int64, percent float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}

	var memTotalKb, memAvailKb int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch fields[0] {
			case "MemTotal:":
				memTotalKb, _ = strconv.ParseInt(fields[1], 10, 64)
			case "MemAvailable:":
				memAvailKb, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	if memTotalKb > 0 {
		totalBytes = memTotalKb * 1024
		availBytes := memAvailKb * 1024
		usedBytes = totalBytes - availBytes
		if usedBytes < 0 {
			usedBytes = 0
		}
		percent = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}
	return
}

// readHostLoadAvg parses 1m, 5m, 15m system load averages on Linux from /proc/loadavg.
func readHostLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return
}

// readRawCPUStat reads aggregate CPU ticks from /proc/stat on Linux.
func readRawCPUStat() (total, idle uint64, ok bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, 0, false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			continue
		}
		total += val
		if i == 4 || i == 5 { // idle + iowait
			idle += val
		}
	}
	return total, idle, true
}
