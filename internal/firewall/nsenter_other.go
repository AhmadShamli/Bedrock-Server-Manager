//go:build !linux

package firewall

func detectNetNS() bool {
	return false
}

func isNonRootUser() bool {
	return false
}
