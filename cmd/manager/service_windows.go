//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	ServiceName        = "BedrockServerManager"
	ServiceDisplayName = "Bedrock Server Manager"
	ServiceDescription = "Bedrock Server Manager (BSM) service managing Minecraft Bedrock Dedicated Servers"
)

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	return err == nil && isSvc
}

type bsmServiceHandler struct {
	cancel context.CancelFunc
}

func (m *bsmServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for req := range r {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			log.Println("[Service] SCM stop/shutdown signal received. Initiating graceful shutdown...")
			changes <- svc.Status{State: svc.StopPending}
			if m.cancel != nil {
				m.cancel()
			}
			return false, 0
		}
	}
	return false, 0
}

func runWindowsService(ctx context.Context, cancel context.CancelFunc) error {
	return svc.Run(ServiceName, &bsmServiceHandler{cancel: cancel})
}

func handleServiceCommand(args []string) (bool, error) {
	if len(args) == 0 || args[0] != "service" {
		return false, nil
	}

	if len(args) < 2 {
		return true, fmt.Errorf("usage: bsm service [install|uninstall|start|stop|status] [options]")
	}

	action := strings.ToLower(args[1])

	m, err := mgr.Connect()
	if err != nil {
		return true, fmt.Errorf("failed to connect to Windows Service Control Manager (ensure you run as Administrator): %w", err)
	}
	defer m.Disconnect()

	switch action {
	case "install":
		return true, installService(m, args[2:])
	case "uninstall":
		return true, uninstallService(m, args[2:])
	case "start":
		return true, startService(m)
	case "stop":
		return true, stopService(m)
	case "status":
		return true, statusService(m)
	default:
		return true, fmt.Errorf("unknown service action '%s'. Valid actions: install, uninstall, start, stop, status", action)
	}
}

func installService(m *mgr.Mgr, extraArgs []string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	startupType := "delayed"
	customDataDir := ""

	for i := 0; i < len(extraArgs); i++ {
		arg := extraArgs[i]
		switch {
		case arg == "--startup" && i+1 < len(extraArgs):
			startupType = strings.ToLower(extraArgs[i+1])
			i++
		case arg == "--data-dir" && i+1 < len(extraArgs):
			customDataDir = extraArgs[i+1]
			i++
		case strings.HasPrefix(arg, "--data-dir="):
			customDataDir = strings.TrimPrefix(arg, "--data-dir=")
		case strings.HasPrefix(arg, "--startup="):
			startupType = strings.ToLower(strings.TrimPrefix(arg, "--startup="))
		}
	}

	// Service launch arguments: bsm.exe serve [--data-dir=...]
	var serviceArgs []string
	serviceArgs = append(serviceArgs, "serve")
	if customDataDir != "" {
		serviceArgs = append(serviceArgs, fmt.Sprintf("--data-dir=%s", customDataDir))
	}

	startType := mgr.StartAutomatic
	delayedAuto := false
	if startupType == "delayed" {
		delayedAuto = true
	} else if startupType == "manual" {
		startType = mgr.StartManual
	}

	cfg := mgr.Config{
		ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:        uint32(startType),
		DisplayName:      ServiceDisplayName,
		Description:      ServiceDescription,
		DelayedAutoStart: delayedAuto,
	}

	s, err := m.CreateService(ServiceName, exePath, cfg, serviceArgs...)
	if err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}
	defer s.Close()

	// Configure SCM auto-recovery: restart on failure
	_ = exec.Command("sc.exe", "failure", ServiceName, "reset=", "86400", "actions=", "restart/60000/restart/60000//").Run()

	fmt.Printf("Service '%s' successfully installed.\n", ServiceName)
	fmt.Printf("Binary Path: %s\n", exePath)
	fmt.Printf("Startup Type: %s (delayed=%v)\n", startupType, delayedAuto)
	if customDataDir != "" {
		fmt.Printf("Data Directory: %s\n", customDataDir)
	} else {
		fmt.Printf("Default Data Directory: %%ProgramData%%\\BedrockServerManager\n")
	}
	fmt.Printf("To start the service, run: bsm service start\n")
	return nil
}

func uninstallService(m *mgr.Mgr, extraArgs []string) error {
	purgeData := false
	for _, arg := range extraArgs {
		if arg == "--purge-data" {
			purgeData = true
		}
	}

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service '%s' is not installed: %w", ServiceName, err)
	}
	defer s.Close()

	// Stop service if running
	st, err := s.Query()
	if err == nil && st.State == svc.Running {
		fmt.Printf("Stopping service '%s'...\n", ServiceName)
		_, _ = s.Control(svc.Stop)
		time.Sleep(2 * time.Second)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	// Flush managed port gate firewall rules (Decision 38)
	_ = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=BSM_PORT_GATE_*").Run()

	fmt.Printf("Service '%s' successfully uninstalled.\n", ServiceName)
	if !purgeData {
		fmt.Println("All world data, servers, and configurations have been preserved. (Use --purge-data if complete removal is desired)")
	} else {
		fmt.Println("Purge data flag detected. Any manual folders should now be cleaned up.")
	}
	return nil
}

func startService(m *mgr.Mgr) error {
	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service '%s' is not installed: %w", ServiceName, err)
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Printf("Service '%s' started successfully.\n", ServiceName)
	return nil
}

func stopService(m *mgr.Mgr) error {
	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service '%s' is not installed: %w", ServiceName, err)
	}
	defer s.Close()

	st, err := s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	fmt.Printf("Service '%s' stop signaled (State: %d).\n", ServiceName, st.State)
	return nil
}

func statusService(m *mgr.Mgr) error {
	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service '%s' is not installed: %w", ServiceName, err)
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("failed to query service status: %w", err)
	}

	stateStr := "Unknown"
	switch st.State {
	case svc.Stopped:
		stateStr = "Stopped"
	case svc.StartPending:
		stateStr = "Start Pending"
	case svc.StopPending:
		stateStr = "Stop Pending"
	case svc.Running:
		stateStr = "Running"
	case svc.ContinuePending:
		stateStr = "Continue Pending"
	case svc.PausePending:
		stateStr = "Pause Pending"
	case svc.Paused:
		stateStr = "Paused"
	}

	fmt.Printf("Service: %s\nStatus:  %s\n", ServiceName, stateStr)
	return nil
}
