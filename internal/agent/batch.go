package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"caroline/internal/explorer"
)

var ErrQueueFull = errors.New("agent batch queue is full")

type BatchQueue struct {
	mu            sync.Mutex
	entries       []explorer.Entry
	containers    map[string]explorer.ContainerInfo
	maxEntries    int
	maxBytes      int
	capacity      int
	approxBytes   int
	flushInterval time.Duration
	sequence      uint64
	agentID       string
	bootID        string
	wake          chan struct{}
}

func NewBatchQueue(agentID, bootID string, maxEntries, maxBytes, capacity int, flushInterval time.Duration, initialSequence uint64) *BatchQueue {
	return &BatchQueue{agentID: agentID, bootID: bootID, maxEntries: maxEntries, maxBytes: maxBytes, capacity: capacity, flushInterval: flushInterval, containers: make(map[string]explorer.ContainerInfo), wake: make(chan struct{}, 1), sequence: initialSequence}
}

func (q *BatchQueue) Add(entry explorer.Entry) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) >= q.capacity {
		return ErrQueueFull
	}
	size, _ := json.Marshal(entry)
	q.entries = append(q.entries, entry)
	q.approxBytes += len(size)
	if len(q.entries) >= q.maxEntries || q.approxBytes >= q.maxBytes {
		select {
		case q.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

func (q *BatchQueue) SetContainers(containers []explorer.ContainerInfo) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, container := range containers {
		q.containers[container.NodeID+"/"+container.ID] = container
	}
}

func (q *BatchQueue) Ready() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries) >= q.maxEntries || q.approxBytes >= q.maxBytes
}

func (q *BatchQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

func (q *BatchQueue) Sequence() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sequence
}

func (q *BatchQueue) Flush() (explorer.EntryBatch, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 && len(q.containers) == 0 {
		return explorer.EntryBatch{}, false
	}
	q.sequence++
	containers := make([]explorer.ContainerInfo, 0, len(q.containers))
	for _, container := range q.containers {
		containers = append(containers, container)
	}
	batch := explorer.EntryBatch{AgentID: q.agentID, BootID: q.bootID, Sequence: q.sequence, Entries: append([]explorer.Entry(nil), q.entries...), Containers: containers}
	q.entries = q.entries[:0]
	q.containers = make(map[string]explorer.ContainerInfo)
	q.approxBytes = 0
	return batch, true
}

func (q *BatchQueue) Run(ctx context.Context, output chan<- explorer.EntryBatch) {
	ticker := time.NewTicker(q.flushInterval)
	defer ticker.Stop()
	defer close(output)
	for {
		select {
		case <-ctx.Done():
			if batch, ok := q.Flush(); ok {
				select {
				case output <- batch:
				case <-ctx.Done():
				}
			}
			return
		case <-ticker.C:
		case <-q.wake:
		}
		if batch, ok := q.Flush(); ok {
			select {
			case output <- batch:
			case <-ctx.Done():
				return
			}
		}
	}
}
