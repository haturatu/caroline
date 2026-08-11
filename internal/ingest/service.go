package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"caroline/internal/agentproto"
	"caroline/internal/explorer"
	"caroline/internal/logstream"
	"caroline/internal/node"
)

var (
	ErrInvalidBatch  = errors.New("invalid agent log batch")
	ErrAgentMismatch = errors.New("agent identity does not match the authenticated node")
)

type Service struct {
	store  explorer.LogStore
	nodes  *node.Service
	broker *logstream.Broker
}

func NewService(store explorer.LogStore, nodes *node.Service, broker *logstream.Broker) (*Service, error) {
	if store == nil || nodes == nil {
		return nil, errors.New("ingest store and node service are required")
	}
	return &Service{store: store, nodes: nodes, broker: broker}, nil
}

func (s *Service) Ingest(ctx context.Context, authenticated node.Node, batch agentproto.LogBatch) (int, bool, error) {
	if batch.ProtocolVersion != agentproto.ProtocolVersion || batch.AgentID == "" || batch.BootID == "" {
		return 0, false, ErrInvalidBatch
	}
	if authenticated.ID != batch.AgentID {
		return 0, false, ErrAgentMismatch
	}
	if len(batch.Entries) > agentproto.MaxBatchEntries {
		return 0, false, fmt.Errorf("%w: too many entries", ErrInvalidBatch)
	}
	for index := range batch.Entries {
		entry := &batch.Entries[index]
		if strings.TrimSpace(entry.InsertID) == "" {
			return 0, false, fmt.Errorf("%w: entry %d has no insertId", ErrInvalidBatch, index)
		}
		labels := entry.Resource.Labels
		if labels == nil {
			labels = make(map[string]string)
			entry.Resource.Labels = labels
		}
		if existing := labels["node_id"]; existing != "" && existing != authenticated.ID {
			return 0, false, ErrAgentMismatch
		}
		labels["node_id"] = authenticated.ID
		labels["node_name"] = authenticated.Name
	}
	for index := range batch.Containers {
		container := &batch.Containers[index]
		if container.NodeID != "" && container.NodeID != authenticated.ID {
			return 0, false, ErrAgentMismatch
		}
		container.NodeID = authenticated.ID
		container.NodeName = authenticated.Name
	}
	accepted, err := s.store.WriteBatch(ctx, explorer.EntryBatch{
		AgentID: batch.AgentID, BootID: batch.BootID, Sequence: batch.Sequence,
		Entries: batch.Entries, Containers: batch.Containers,
	})
	if err != nil {
		return 0, false, err
	}
	if accepted && s.broker != nil {
		for _, entry := range batch.Entries {
			s.broker.Publish(entry)
		}
	}
	return len(batch.Entries), accepted, nil
}

func (s *Service) Heartbeat(ctx context.Context, authenticated node.Node) error {
	return s.nodes.Touch(ctx, authenticated.ID)
}
