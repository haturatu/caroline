package agentproto

import (
	"time"

	"caroline/internal/explorer"
)

type RegisterRequest struct {
	ProtocolVersion int      `json:"protocolVersion"`
	AgentVersion    string   `json:"agentVersion"`
	EnrollmentToken string   `json:"enrollmentToken"`
	AgentID         string   `json:"agentId"`
	PublicKey       []byte   `json:"publicKey"`
	Fingerprint     string   `json:"fingerprint"`
	Hostname        string   `json:"hostname"`
	OS              string   `json:"os"`
	Architecture    string   `json:"architecture"`
	Nonce           string   `json:"nonce"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

type RegisterResponse struct {
	ProtocolVersion int       `json:"protocolVersion"`
	AgentID         string    `json:"agentId"`
	SessionID       string    `json:"sessionId"`
	HubKeyID        string    `json:"hubKeyId"`
	HubPublicKey    []byte    `json:"hubPublicKey"`
	Nonce           string    `json:"nonce"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Signature       []byte    `json:"signature"`
	Capabilities    []string  `json:"capabilities,omitempty"`
}

type SessionRequest struct {
	ProtocolVersion int    `json:"protocolVersion"`
	AgentID         string `json:"agentId"`
	SessionID       string `json:"sessionId"`
	Nonce           string `json:"nonce"`
}

type SessionResponse struct {
	ProtocolVersion int       `json:"protocolVersion"`
	SessionID       string    `json:"sessionId"`
	HubKeyID        string    `json:"hubKeyId"`
	HubPublicKey    []byte    `json:"hubPublicKey"`
	Nonce           string    `json:"nonce"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Signature       []byte    `json:"signature"`
	Capabilities    []string  `json:"capabilities,omitempty"`
}

type LogBatch struct {
	ProtocolVersion int                      `json:"protocolVersion"`
	AgentID         string                   `json:"agentId"`
	BootID          string                   `json:"bootId"`
	Sequence        uint64                   `json:"sequence"`
	Entries         []explorer.Entry         `json:"entries"`
	Containers      []explorer.ContainerInfo `json:"containers,omitempty"`
}

type Heartbeat struct {
	ProtocolVersion int       `json:"protocolVersion"`
	AgentID         string    `json:"agentId"`
	BootID          string    `json:"bootId"`
	AgentTime       time.Time `json:"agentTime"`
	UptimeSeconds   int64     `json:"uptimeSeconds"`
	Containers      int       `json:"containers"`
	QueueDepth      int       `json:"queueDepth"`
	SpoolBytes      int64     `json:"spoolBytes"`
	DroppedEntries  uint64    `json:"droppedEntries"`
}

type ControlEvent struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	ExpiresAt *time.Time     `json:"expiresAt,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}
