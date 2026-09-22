package player

import (
	"testing"
)

func TestPlayerParser(t *testing.T) {
	var joinedGT, leftGT string
	pm := NewManager(
		func(srv, gt, xuid string) { joinedGT = gt },
		func(srv, gt, xuid string) { leftGT = gt },
	)

	// 1. Connection line
	pm.ProcessLine("srv-1", "Player connected: Steve, xuid: 2535412345678901")
	if joinedGT != "Steve" {
		t.Errorf("expected Steve to join, got %s", joinedGT)
	}

	online := pm.GetOnlinePlayers("srv-1")
	if len(online) != 1 || online[0].Gamertag != "Steve" || online[0].XUID != "2535412345678901" {
		t.Fatalf("unexpected online players: %+v", online)
	}

	// 2. Chat message
	pm.ProcessLine("srv-1", "<Steve> Watch out for creepers!")
	chat := pm.GetChatFeed("srv-1", 10)
	if len(chat) != 1 || chat[0].Gamertag != "Steve" || chat[0].Message != "Watch out for creepers!" {
		t.Fatalf("unexpected chat feed: %+v", chat)
	}

	// 3. Alternate chat format
	pm.ProcessLine("srv-1", "[CHAT] Alex: Found diamond ore!")
	chat = pm.GetChatFeed("srv-1", 10)
	if len(chat) != 2 || chat[1].Gamertag != "Alex" || chat[1].Message != "Found diamond ore!" {
		t.Fatalf("unexpected chat feed: %+v", chat)
	}

	// 4. Disconnect line
	pm.ProcessLine("srv-1", "Player disconnected: Steve, xuid: 2535412345678901")
	if leftGT != "Steve" {
		t.Errorf("expected Steve to leave, got %s", leftGT)
	}

	onlineAfter := pm.GetOnlinePlayers("srv-1")
	if len(onlineAfter) != 0 {
		t.Errorf("expected 0 online players after disconnect, got %d", len(onlineAfter))
	}
}
