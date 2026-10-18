//go:build !windows

package main

import (
	"context"
	"fmt"
)

func isWindowsService() bool {
	return false
}

func runWindowsService(ctx context.Context, cancel context.CancelFunc) error {
	return nil
}

func handleServiceCommand(args []string) (bool, error) {
	if len(args) > 0 && args[0] == "service" {
		return true, fmt.Errorf("the 'service' command is only supported on Windows hosts")
	}
	return false, nil
}
