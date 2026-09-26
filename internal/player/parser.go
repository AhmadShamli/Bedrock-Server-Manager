package player

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// OnlinePlayer represents an active player on a Bedrock server.
type OnlinePlayer struct {
	ServerID string    `json:"server_id"`
	Gamertag string    `json:"gamertag"`
	XUID     string    `json:"xuid"`
	JoinedAt time.Time `json:"joined_at"`
}

// ChatMessage represents a captured in-game chat message.
type ChatMessage struct {
	ServerID  string    `json:"server_id"`
	Gamertag  string    `json:"gamertag"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	// Player connected: Steve, xuid: 2535412345678901
	connectRegex = regexp.MustCompile(`Player connected:\s*([^,]+),\s*xuid:\s*(\w+)`)
	// Player disconnected: Steve, xuid: 2535412345678901
	disconnectRegex = regexp.MustCompile(`Player disconnected:\s*([^,]+),\s*xuid:\s*(\w+)`)
	// In-game chat patterns:
	// <Steve> Hello world!
	// [CHAT] Steve: Hello world!
	chatRegex1 = regexp.MustCompile(`<([^>]+)>\s*(.*)`)
	chatRegex2 = regexp.MustCompile(`\[CHAT\]\s*([^:]+):\s*(.*)`)
)

// Manager tracks real-time player states and chat feeds across servers.
type Manager struct {
	mu            sync.RWMutex
	onlinePlayers map[string]map[string]*OnlinePlayer // serverID -> gamertag -> player
	chatFeeds     map[string][]ChatMessage            // serverID -> list of chat messages
	onPlayerJoin  func(serverID, gamertag, xuid string)
	onPlayerLeave func(serverID, gamertag, xuid string)
}

// NewManager creates a Player Manager.
func NewManager(onJoin, onLeave func(serverID, gamertag, xuid string)) *Manager {
	return &Manager{
		onlinePlayers: make(map[string]map[string]*OnlinePlayer),
		chatFeeds:     make(map[string][]ChatMessage),
		onPlayerJoin:  onJoin,
		onPlayerLeave: onLeave,
	}
}

// ProcessLine inspects stdout logs for player connection and chat events.
func (m *Manager) ProcessLine(serverID, line string) {
	now := time.Now().UTC()

	// 1. Player Connected
	if matches := connectRegex.FindStringSubmatch(line); len(matches) == 3 {
		gt := strings.TrimSpace(matches[1])
		xuid := strings.TrimSpace(matches[2])

		m.mu.Lock()
		if _, exists := m.onlinePlayers[serverID]; !exists {
			m.onlinePlayers[serverID] = make(map[string]*OnlinePlayer)
		}
		m.onlinePlayers[serverID][gt] = &OnlinePlayer{
			ServerID: serverID,
			Gamertag: gt,
			XUID:     xuid,
			JoinedAt: now,
		}
		m.mu.Unlock()

		if m.onPlayerJoin != nil {
			m.onPlayerJoin(serverID, gt, xuid)
		}
		return
	}

	// 2. Player Disconnected
	if matches := disconnectRegex.FindStringSubmatch(line); len(matches) == 3 {
		gt := strings.TrimSpace(matches[1])
		xuid := strings.TrimSpace(matches[2])

		m.mu.Lock()
		if players, exists := m.onlinePlayers[serverID]; exists {
			delete(players, gt)
		}
		m.mu.Unlock()

		if m.onPlayerLeave != nil {
			m.onPlayerLeave(serverID, gt, xuid)
		}
		return
	}

	// 3. Chat Detection
	var chatGT, chatMsg string
	if matches := chatRegex1.FindStringSubmatch(line); len(matches) == 3 {
		chatGT = strings.TrimSpace(matches[1])
		chatMsg = strings.TrimSpace(matches[2])
	} else if matches := chatRegex2.FindStringSubmatch(line); len(matches) == 3 {
		chatGT = strings.TrimSpace(matches[1])
		chatMsg = strings.TrimSpace(matches[2])
	}

	if chatGT != "" && chatMsg != "" {
		m.mu.Lock()
		feed := m.chatFeeds[serverID]
		msg := ChatMessage{
			ServerID:  serverID,
			Gamertag:  chatGT,
			Message:   chatMsg,
			Timestamp: now,
		}
		if len(feed) >= 200 {
			feed = feed[1:]
		}
		m.chatFeeds[serverID] = append(feed, msg)
		m.mu.Unlock()
	}
}

// GetOnlinePlayers returns active players for a given server.
func (m *Manager) GetOnlinePlayers(serverID string) []OnlinePlayer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playersMap, exists := m.onlinePlayers[serverID]
	if !exists {
		return []OnlinePlayer{}
	}

	list := make([]OnlinePlayer, 0)
	for _, p := range playersMap {
		list = append(list, *p)
	}
	return list
}

// GetChatFeed returns recent in-game chat messages.
func (m *Manager) GetChatFeed(serverID string, limit int) []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feed, exists := m.chatFeeds[serverID]
	if !exists || len(feed) == 0 {
		return []ChatMessage{}
	}

	if limit <= 0 || limit > len(feed) {
		limit = len(feed)
	}
	start := len(feed) - limit
	res := feed[start:]
	if res == nil {
		return []ChatMessage{}
	}
	return res
}

// ClearServer clears player state when a server stops.
func (m *Manager) ClearServer(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.onlinePlayers, serverID)
}
