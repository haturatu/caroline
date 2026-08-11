package logstream

import (
	"context"
	"sync"
	"time"

	"caroline/internal/explorer"
)

// Broker is the Hub-side in-memory event bus. It has no Docker dependency;
// agents and local compatibility collectors publish normalized Entries into it.
type Broker struct {
	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	seen        map[string]struct{}
	seenOrder   []string
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[*subscriber]struct{}),
		seen:        make(map[string]struct{}),
	}
}

func (b *Broker) Subscribe(ctx context.Context, selected map[string]bool, selectedNodes map[string]bool, since time.Time) *Subscription {
	owner := &subscriber{
		selected:      cloneSelection(selected),
		selectedNodes: cloneSelection(selectedNodes),
		since:         since,
		done:          make(chan struct{}),
		entries:       make(chan explorer.Entry, defaultSubscriptionBuffer),
		errors:        make(chan StreamError, 32),
	}
	b.mu.Lock()
	b.subscribers[owner] = struct{}{}
	b.mu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			b.closeSubscription(owner)
		case <-owner.done:
		}
	}()
	return &Subscription{Entries: owner.entries, Errors: owner.errors, broker: b, owner: owner}
}

func (b *Broker) Publish(entry explorer.Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if entry.InsertID != "" {
		if _, exists := b.seen[entry.InsertID]; exists {
			return
		}
		b.seen[entry.InsertID] = struct{}{}
		b.seenOrder = append(b.seenOrder, entry.InsertID)
		if len(b.seenOrder) > maxSeenEntries {
			delete(b.seen, b.seenOrder[0])
			b.seenOrder = b.seenOrder[1:]
		}
	}
	for owner := range b.subscribers {
		if !owner.matches(entry) {
			continue
		}
		select {
		case owner.entries <- entry:
		case <-owner.done:
		default:
		}
	}
}

func (b *Broker) Close() {
	b.mu.Lock()
	owners := make([]*subscriber, 0, len(b.subscribers))
	for owner := range b.subscribers {
		owners = append(owners, owner)
	}
	b.mu.Unlock()
	for _, owner := range owners {
		b.closeSubscription(owner)
	}
}

func (b *Broker) closeSubscription(owner *subscriber) {
	b.mu.Lock()
	delete(b.subscribers, owner)
	b.mu.Unlock()
	owner.close()
}

func (s *subscriber) matches(entry explorer.Entry) bool {
	if !s.since.IsZero() && entry.Timestamp.Before(s.since) {
		return false
	}
	labels := entry.Resource.Labels
	if len(s.selected) > 0 && !s.selected[labels["container_id"]] {
		return false
	}
	if len(s.selectedNodes) > 0 && !s.selectedNodes[labels["node_id"]] {
		return false
	}
	return true
}
