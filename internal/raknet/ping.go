package raknet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// RakNet offline message ID magic bytes.
var RakNetMagic = []byte{
	0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe,
	0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78,
}

const (
	IDUnconnectedPing = 0x01
	IDUnconnectedPong = 0x1c
)

// PingResponse contains parsed metadata from RakNet Unconnected Pong.
type PingResponse struct {
	LatencyMs      int64  `json:"latency_ms"`
	ServerName     string `json:"server_name"`
	Protocol       int    `json:"protocol"`
	Version        string `json:"version"`
	OnlinePlayers  int    `json:"online_players"`
	MaxPlayers     int    `json:"max_players"`
	ServerGUID     string `json:"server_guid"`
	WorldName      string `json:"world_name"`
	GameMode       string `json:"game_mode"`
	PortIPv4       int    `json:"port_ipv4"`
	PortIPv6       int    `json:"port_ipv6"`
}

// PingServer sends an Unconnected Ping packet to a Bedrock server and parses the response.
func PingServer(host string, port int, timeout time.Duration) (*PingResponse, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Build Ping packet
	nowMs := time.Now().UnixMilli()
	buf := new(bytes.Buffer)
	buf.WriteByte(IDUnconnectedPing)
	_ = binary.Write(buf, binary.BigEndian, nowMs)
	buf.Write(RakNetMagic)
	_ = binary.Write(buf, binary.BigEndian, int64(2)) // Client GUID

	sendTime := time.Now()
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	// Read Pong
	recvBuf := make([]byte, 1500)
	n, err := conn.Read(recvBuf)
	if err != nil {
		return nil, err
	}
	latency := time.Since(sendTime).Milliseconds()

	if n < 35 || recvBuf[0] != IDUnconnectedPong {
		return nil, fmt.Errorf("invalid pong response")
	}

	// String length is at offset 33
	strLen := binary.BigEndian.Uint16(recvBuf[33:35])
	if int(35+strLen) > n {
		return nil, fmt.Errorf("truncated pong payload")
	}

	payload := string(recvBuf[35 : 35+strLen])
	return ParsePongPayload(payload, latency)
}

// ParsePongPayload parses the raw MCPE unconnected pong payload string.
func ParsePongPayload(payload string, latency int64) (*PingResponse, error) {
	parts := strings.Split(payload, ";")
	if len(parts) < 6 {
		return nil, fmt.Errorf("unexpected pong format: %s", payload)
	}

	// MCPE;ServerName;Protocol;Version;Players;MaxPlayers;ServerID;SubName;GameMode;GameModeNumeric;Port4;Port6;
	res := &PingResponse{
		LatencyMs:  latency,
		ServerName: parts[1],
		Version:    parts[3],
	}

	if p, err := strconv.Atoi(parts[2]); err == nil {
		res.Protocol = p
	}
	if players, err := strconv.Atoi(parts[4]); err == nil {
		res.OnlinePlayers = players
	}
	if max, err := strconv.Atoi(parts[5]); err == nil {
		res.MaxPlayers = max
	}
	if len(parts) > 6 {
		res.ServerGUID = parts[6]
	}
	if len(parts) > 7 {
		res.WorldName = parts[7]
	}
	if len(parts) > 8 {
		res.GameMode = parts[8]
	}
	if len(parts) > 10 {
		if p4, err := strconv.Atoi(parts[10]); err == nil {
			res.PortIPv4 = p4
		}
	}
	if len(parts) > 11 {
		if p6, err := strconv.Atoi(parts[11]); err == nil {
			res.PortIPv6 = p6
		}
	}

	return res, nil
}
