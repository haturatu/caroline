package node

import (
	"context"
	"time"
)

type Status string

const (
	StatusRegistering Status = "registering"
	StatusOnline      Status = "online"
	StatusOffline     Status = "offline"
	StatusRevoked     Status = "revoked"
)

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Fingerprint is inventory metadata for identifying a host. Authentication
	// is provided by the Agent public key and signed requests, not by this value.
	Fingerprint     string    `json:"fingerprint"`
	PublicKey       []byte    `json:"publicKey,omitempty"`
	Hostname        string    `json:"hostname"`
	OS              string    `json:"os"`
	Architecture    string    `json:"architecture"`
	AgentVersion    string    `json:"agentVersion,omitempty"`
	ProtocolVersion int       `json:"protocolVersion"`
	ConnectedAt     time.Time `json:"connectedAt,omitempty"`
	LastSeenAt      time.Time `json:"lastSeenAt,omitempty"`
	Status          Status    `json:"status"`
}

type EnrollmentToken struct {
	ID        string     `json:"id"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

type Store interface {
	SaveNode(context.Context, Node) error
	GetNode(context.Context, string) (Node, error)
	ListNodes(context.Context) ([]Node, error)
	RevokeNode(context.Context, string, time.Time) error
	TouchNode(context.Context, string, time.Time) error
	PutEnrollmentToken(context.Context, string, string, time.Time, time.Time) error
	ConsumeEnrollmentToken(context.Context, string, time.Time) (bool, error)
	RememberNonce(context.Context, string, string, time.Time) (bool, error)
}
