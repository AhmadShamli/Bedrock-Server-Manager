package allocator

import (
	"net"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestPortAllocatorFindAvailablePortPair(t *testing.T) {
	pa := NewPortAllocator()

	existing := []models.Server{
		{ID: "s1", Port: 19132, PortV6: 19133},
	}

	p4, p6, err := pa.FindAvailablePortPair(19132, existing)
	if err != nil {
		t.Fatalf("FindAvailablePortPair failed: %v", err)
	}

	if p4 <= 19133 {
		t.Errorf("expected allocated port > 19133, got %d", p4)
	}
	if p6 != p4+1 {
		t.Errorf("expected IPv6 port to be %d, got %d", p4+1, p6)
	}
}

func TestPortAllocatorValidatePortAssignment(t *testing.T) {
	pa := NewPortAllocator()

	existing := []models.Server{
		{ID: "s1", Name: "Survival 1", Port: 19132, PortV6: 19133},
	}

	// Conflict with existing
	if err := pa.ValidatePortAssignment(19132, 19135, "s2", existing); err == nil {
		t.Errorf("expected error on port collision with existing server")
	}

	// Same server exclude check
	// Binding 19132 should work for same server ID if port is free on host
	// If 19132 is free on host, ValidatePortAssignment should not fail on database collision for itself
	_ = pa.ValidatePortAssignment(19132, 19133, "s1", existing)

	// In-use port on host
	pc, err := net.ListenPacket("udp", ":29132")
	if err == nil {
		defer pc.Close()
		if err := pa.ValidatePortAssignment(29132, 29133, "new-srv", existing); err == nil {
			t.Errorf("expected error when port is already bound on host")
		}
	}
}
