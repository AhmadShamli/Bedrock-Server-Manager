package allocator

import (
	"fmt"
	"net"
	"sync"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// PortAllocator handles UDP port discovery and collision prevention.
type PortAllocator struct {
	mu sync.Mutex
}

// NewPortAllocator creates a PortAllocator.
func NewPortAllocator() *PortAllocator {
	return &PortAllocator{}
}

// IsUDPPortAvailable checks if a UDP port is open and can be bound on the host.
func (pa *PortAllocator) IsUDPPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	_ = pc.Close()
	return true
}

// FindAvailablePortPair scans for an unused (IPv4, IPv6) port pair starting from startPort.
// Minecraft Bedrock uses consecutive ports: port for IPv4 and port+1 for IPv6.
func (pa *PortAllocator) FindAvailablePortPair(startPort int, existingServers []models.Server) (int, int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	allocatedPorts := make(map[int]bool)
	for _, s := range existingServers {
		allocatedPorts[s.Port] = true
		allocatedPorts[s.PortV6] = true
	}

	current := startPort
	if current < 1024 {
		current = 19132
	}

	for current <= 65534 {
		portV4 := current
		portV6 := current + 1

		// Check if registered in database
		if !allocatedPorts[portV4] && !allocatedPorts[portV6] {
			// Check if bindable on host
			if pa.IsUDPPortAvailable(portV4) && pa.IsUDPPortAvailable(portV6) {
				return portV4, portV6, nil
			}
		}

		current += 2
	}

	return 0, 0, fmt.Errorf("no available UDP port pairs found in range [%d, 65534]", startPort)
}

// ValidatePortAssignment verifies that a requested port is not used by other servers or active listeners.
func (pa *PortAllocator) ValidatePortAssignment(port, portV6 int, excludeServerID string, existingServers []models.Server) error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	if port <= 0 || port > 65535 || portV6 <= 0 || portV6 > 65535 {
		return fmt.Errorf("port numbers must be between 1 and 65535")
	}
	if port == portV6 {
		return fmt.Errorf("IPv4 and IPv6 ports cannot be the same")
	}

	for _, s := range existingServers {
		if s.ID == excludeServerID {
			continue
		}
		if s.Port == port || s.PortV6 == port {
			return fmt.Errorf("port %d is already assigned to server '%s' (%s)", port, s.Name, s.ID)
		}
		if s.Port == portV6 || s.PortV6 == portV6 {
			return fmt.Errorf("port %d is already assigned to server '%s' (%s)", portV6, s.Name, s.ID)
		}
	}

	if !pa.IsUDPPortAvailable(port) {
		return fmt.Errorf("port %d is already in use by another process on this host", port)
	}
	if !pa.IsUDPPortAvailable(portV6) {
		return fmt.Errorf("port %d is already in use by another process on this host", portV6)
	}

	return nil
}
