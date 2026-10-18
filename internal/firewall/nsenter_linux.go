//go:build linux

package firewall

import (
	"os"
	"syscall"
)

func detectNetNS() bool {
	selfStat, err1 := os.Stat("/proc/self/ns/net")
	initStat, err2 := os.Stat("/proc/1/ns/net")
	if err1 != nil || err2 != nil {
		return false
	}

	selfSys, ok1 := selfStat.Sys().(*syscall.Stat_t)
	initSys, ok2 := initStat.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		return false
	}

	return selfSys.Ino != initSys.Ino
}

func isNonRootUser() bool {
	return os.Getuid() != 0
}
