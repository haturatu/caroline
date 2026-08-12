package agentproto

import (
	"time"

	"caroline/internal/explorer"
)

const MaxContainerMetadata = 500

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
	ProtocolVersion int    `json:"protocolVersion"`
	AgentID         string `json:"agentId"`
	// ChallengeID identifies the Hub identity challenge. It is not an access
	// credential; normal requests use Ed25519 signatures.
	ChallengeID  string    `json:"challengeId"`
	HubKeyID     string    `json:"hubKeyId"`
	HubPublicKey []byte    `json:"hubPublicKey"`
	Nonce        string    `json:"nonce"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Signature    []byte    `json:"signature"`
	Capabilities []string  `json:"capabilities,omitempty"`
}

type ChallengeRequest struct {
	ProtocolVersion int    `json:"protocolVersion"`
	AgentID         string `json:"agentId"`
	Nonce           string `json:"nonce"`
}

type ChallengeResponse struct {
	ProtocolVersion int       `json:"protocolVersion"`
	ChallengeID     string    `json:"challengeId"`
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
	// ContainerMetadata is the complete snapshot of containers currently
	// collected by this Agent, rather than a delta since the last heartbeat.
	ContainerMetadata []explorer.ContainerInfo `json:"containerMetadata"`
	QueueDepth        int                      `json:"queueDepth"`
	SpoolBytes        int64                    `json:"spoolBytes"`
	DroppedEntries    uint64                   `json:"droppedEntries"`
}

type ControlEvent struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	ExpiresAt *time.Time     `json:"expiresAt,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}
