package raknet

import (
	"testing"
)

func TestParsePongPayload(t *testing.T) {
	// Standard Bedrock unconnected pong payload format:
	// MCPE;ServerName;Protocol;Version;Players;MaxPlayers;ServerID;SubName;GameMode;GameModeNumeric;Port4;Port6;
	payload := "MCPE;Survival Server;589;1.20.73;3;10;13253452345;Dedicated Realm;Survival;1;19132;19133;"

	res, err := ParsePongPayload(payload, 25)
	if err != nil {
		t.Fatalf("ParsePongPayload failed: %v", err)
	}

	if res.LatencyMs != 25 {
		t.Errorf("expected latency 25ms, got %d", res.LatencyMs)
	}
	if res.ServerName != "Survival Server" {
		t.Errorf("expected ServerName 'Survival Server', got '%s'", res.ServerName)
	}
	if res.Protocol != 589 {
		t.Errorf("expected protocol 589, got %d", res.Protocol)
	}
	if res.Version != "1.20.73" {
		t.Errorf("expected version 1.20.73, got '%s'", res.Version)
	}
	if res.OnlinePlayers != 3 {
		t.Errorf("expected OnlinePlayers 3, got %d", res.OnlinePlayers)
	}
	if res.MaxPlayers != 10 {
		t.Errorf("expected MaxPlayers 10, got %d", res.MaxPlayers)
	}
	if res.ServerGUID != "13253452345" {
		t.Errorf("expected ServerGUID 13253452345, got '%s'", res.ServerGUID)
	}
	if res.WorldName != "Dedicated Realm" {
		t.Errorf("expected WorldName 'Dedicated Realm', got '%s'", res.WorldName)
	}
	if res.GameMode != "Survival" {
		t.Errorf("expected GameMode 'Survival', got '%s'", res.GameMode)
	}
	if res.PortIPv4 != 19132 {
		t.Errorf("expected PortIPv4 19132, got %d", res.PortIPv4)
	}
	if res.PortIPv6 != 19133 {
		t.Errorf("expected PortIPv6 19133, got %d", res.PortIPv6)
	}
}

func TestParsePongPayloadInvalid(t *testing.T) {
	_, err := ParsePongPayload("invalid", 10)
	if err == nil {
		t.Fatalf("expected error for malformed payload")
	}
}
