package logstream

import (
	"context"
	"sort"
	"sync"
	"time"

	"caroline/internal/docker"
	"caroline/internal/explorer"
)

const (
	defaultSubscriptionBuffer = 1024
	maxSeenEntries            = 4096
	streamRetryDelay          = time.Second
	containerListAttempts     = 3
	containerListRetryDelay   = 100 * time.Millisecond
)

type Source interface {
	ListRunning(context.Context) ([]docker.Container, error)
	FollowLogs(context.Context, string, time.Time, func(docker.Frame) error) error
}

type StreamError struct {
	Container docker.Container
	Err       error
}

type Subscription struct {
	Entries            <-chan explorer.Entry
	Errors             <-chan StreamError
	SelectedContainers int
	StreamedContainers int

	manager *Manager
	broker  *Broker
	owner   *subscriber
	once    sync.Once
}

func (s *Subscription) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		if s.manager != nil {
			s.manager.closeSubscription(s.owner)
		}
		if s.broker != nil {
			s.broker.closeSubscription(s.owner)
		}
	})
}

type Manager struct {
	source   Source
	broker   *Broker
	nodeID   string
	nodeName string

	rootContext context.Context
	rootCancel  context.CancelFunc

	mu          sync.Mutex
	streams     map[string]*stream
	subscribers map[*subscriber]struct{}
}

type subscriber struct {
	manager       *Manager
	selected      map[string]bool
	selectedNodes map[string]bool
	since         time.Time
	done          chan struct{}
	entries       chan explorer.Entry
	errors        chan StreamError
	once          sync.Once
}

type stream struct {
	manager    *Manager
	container  docker.Container
	since      time.Time
	context    context.Context
	cancel     context.CancelFunc
	started    bool
	subs       map[*subscriber]struct{}
	seen       map[string]struct{}
	seenOrder  []string
	lastSeenMu sync.Mutex
	lastSeen   time.Time
	mu         sync.Mutex
}

func NewManager(source Source) *Manager {
	return NewManagerForNode(source, "", "")
}

func NewManagerForNode(source Source, nodeID, nodeName string) *Manager {
	context, cancel := context.WithCancel(context.Background())
	return &Manager{
		source:      source,
		nodeID:      nodeID,
		nodeName:    nodeName,
		rootContext: context,
		rootCancel:  cancel,
		streams:     make(map[string]*stream),
		subscribers: make(map[*subscriber]struct{}),
	}
}

func NewBrokerManager(broker *Broker) *Manager {
	context, cancel := context.WithCancel(context.Background())
	return &Manager{
		broker:      broker,
		rootContext: context,
		rootCancel:  cancel,
	}
}

func (m *Manager) Close() {
	if m.broker != nil {
		m.rootCancel()
		m.broker.Close()
		return
	}
	m.rootCancel()
	m.mu.Lock()
	owners := make([]*subscriber, 0, len(m.subscribers))
	for owner := range m.subscribers {
		owners = append(owners, owner)
	}
	m.mu.Unlock()
	for _, owner := range owners {
		m.closeSubscription(owner)
	}
	m.mu.Lock()
	streams := make([]*stream, 0, len(m.streams))
	for _, current := range m.streams {
		streams = append(streams, current)
	}
	m.streams = make(map[string]*stream)
	m.mu.Unlock()
	for _, current := range streams {
		current.cancel()
	}
}

func (m *Manager) Subscribe(
	ctx context.Context,
	selected map[string]bool,
	since time.Time,
	maxContainers int,
) (*Subscription, error) {
	return m.SubscribeWithNodes(ctx, selected, nil, since, maxContainers)
}

func (m *Manager) SubscribeWithNodes(
	ctx context.Context,
	selected map[string]bool,
	selectedNodes map[string]bool,
	since time.Time,
	maxContainers int,
) (*Subscription, error) {
	if m.broker != nil {
		return m.broker.Subscribe(ctx, selected, selectedNodes, since), nil
	}
	containers, err := m.listRunning(ctx)
	if err != nil {
		return nil, err
	}

	selectedContainers := make([]docker.Container, 0, len(containers))
	for _, container := range containers {
		if len(selected) == 0 || explorer.MatchesContainerSelection(container, selected) {
			selectedContainers = append(selectedContainers, container)
		}
	}
	sort.Slice(selectedContainers, func(i, j int) bool {
		return explorer.ContainerName(selectedContainers[i]) < explorer.ContainerName(selectedContainers[j])
	})
	streamedContainers := selectedContainers
	if maxContainers > 0 && len(streamedContainers) > maxContainers {
		streamedContainers = streamedContainers[:maxContainers]
	}

	owner := &subscriber{
		manager:       m,
		selected:      cloneSelection(selected),
		selectedNodes: cloneSelection(selectedNodes),
		since:         since,
		done:          make(chan struct{}),
		entries:       make(chan explorer.Entry, defaultSubscriptionBuffer),
		errors:        make(chan StreamError, 32),
	}
	newStreams := make([]*stream, 0, len(streamedContainers))
	m.mu.Lock()
	m.subscribers[owner] = struct{}{}
	for _, container := range streamedContainers {
		current, created := m.getOrCreateStreamLocked(container, since)
		shouldStart := created
		current.mu.Lock()
		current.subs[owner] = struct{}{}
		if !current.started {
			shouldStart = true
		}
		current.mu.Unlock()
		if shouldStart {
			newStreams = append(newStreams, current)
		}
	}
	m.mu.Unlock()
	for _, current := range newStreams {
		m.startStream(current)
	}

	go func() {
		select {
		case <-ctx.Done():
			m.closeSubscription(owner)
		case <-owner.done:
		}
	}()

	return &Subscription{
		Entries:            owner.entries,
		Errors:             owner.errors,
		SelectedContainers: len(selectedContainers),
		StreamedContainers: len(streamedContainers),
		manager:            m,
		owner:              owner,
	}, nil
}

func (m *Manager) Refresh(ctx context.Context) error {
	if m.broker != nil {
		return nil
	}
	containers, err := m.listRunning(ctx)
	if err != nil {
		return err
	}

	newStreams := make([]*stream, 0, len(containers))
	m.mu.Lock()
	for _, container := range containers {
		current, created := m.getOrCreateStreamLocked(container, time.Now().UTC())
		if created {
			current.mu.Lock()
			for owner := range m.subscribers {
				if len(owner.selected) == 0 || explorer.MatchesContainerSelection(container, owner.selected) {
					current.subs[owner] = struct{}{}
				}
			}
			current.mu.Unlock()
			if currentHasSubscribers(current) {
				newStreams = append(newStreams, current)
			}
		}
	}
	m.mu.Unlock()
	for _, current := range newStreams {
		m.startStream(current)
	}
	return nil
}

func (m *Manager) listRunning(ctx context.Context) ([]docker.Container, error) {
	var lastErr error
	for attempt := 0; attempt < containerListAttempts; attempt++ {
		listContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		containers, err := m.source.ListRunning(listContext)
		cancel()
		if err == nil {
			return containers, nil
		}
		lastErr = err
		if attempt+1 == containerListAttempts {
			break
		}
		timer := time.NewTimer(containerListRetryDelay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (m *Manager) getOrCreateStreamLocked(container docker.Container, since time.Time) (*stream, bool) {
	if current, ok := m.streams[container.ID]; ok {
		return current, false
	}
	streamContext, cancel := context.WithCancel(m.rootContext)
	current := &stream{
		manager:   m,
		container: container,
		since:     since,
		context:   streamContext,
		cancel:    cancel,
		subs:      make(map[*subscriber]struct{}),
		seen:      make(map[string]struct{}),
		lastSeen:  since,
	}
	m.streams[container.ID] = current
	return current, true
}

func (m *Manager) startStream(current *stream) {
	current.mu.Lock()
	if current.started || len(current.subs) == 0 {
		current.mu.Unlock()
		return
	}
	current.started = true
	current.mu.Unlock()
	go m.watch(current)
}

func (m *Manager) watch(current *stream) {
	defer func() {
		m.mu.Lock()
		if m.streams[current.container.ID] == current {
			delete(m.streams, current.container.ID)
		}
		m.mu.Unlock()
	}()

	for {
		if !current.hasSubscribers() {
			return
		}
		current.lastSeenMu.Lock()
		since := current.lastSeen
		current.lastSeenMu.Unlock()
		err := m.source.FollowLogs(current.context, current.container.ID, since, func(frame docker.Frame) error {
			for _, line := range explorer.ParseLogFrame(frame, current.container) {
				current.lastSeenMu.Lock()
				if line.Timestamp.After(current.lastSeen) {
					current.lastSeen = line.Timestamp
				}
				current.lastSeenMu.Unlock()
				current.publish(explorer.ToEntryForNode(line, current.container, m.nodeID, m.nodeName))
			}
			return nil
		})
		if current.context.Err() != nil {
			return
		}
		if err != nil {
			current.publishError(StreamError{Container: current.container, Err: err})
		}
		timer := time.NewTimer(streamRetryDelay)
		select {
		case <-current.context.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *stream) hasSubscribers() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs) > 0
}

func (s *stream) publish(entry explorer.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.seen[entry.InsertID]; exists {
		return
	}
	s.seen[entry.InsertID] = struct{}{}
	s.seenOrder = append(s.seenOrder, entry.InsertID)
	if len(s.seenOrder) > maxSeenEntries {
		delete(s.seen, s.seenOrder[0])
		s.seenOrder = s.seenOrder[1:]
	}
	for owner := range s.subs {
		if !owner.since.IsZero() && entry.Timestamp.Before(owner.since) {
			continue
		}
		select {
		case owner.entries <- entry:
		case <-owner.done:
		default:
		}
	}
}

func (s *stream) publishError(streamError StreamError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for owner := range s.subs {
		select {
		case owner.errors <- streamError:
		case <-owner.done:
		default:
		}
	}
}

func (m *Manager) closeSubscription(owner *subscriber) {
	m.mu.Lock()
	delete(m.subscribers, owner)
	streams := make([]*stream, 0, len(m.streams))
	for _, current := range m.streams {
		current.mu.Lock()
		delete(current.subs, owner)
		empty := len(current.subs) == 0
		current.mu.Unlock()
		if empty {
			delete(m.streams, current.container.ID)
			streams = append(streams, current)
		}
	}
	m.mu.Unlock()
	for _, current := range streams {
		current.cancel()
	}
	owner.close()
}

func (s *subscriber) close() {
	s.once.Do(func() {
		close(s.done)
		close(s.entries)
		close(s.errors)
	})
}

func currentHasSubscribers(current *stream) bool {
	current.mu.Lock()
	defer current.mu.Unlock()
	return len(current.subs) > 0
}

func cloneSelection(selected map[string]bool) map[string]bool {
	if len(selected) == 0 {
		return nil
	}
	copyOfSelection := make(map[string]bool, len(selected))
	for key, value := range selected {
		copyOfSelection[key] = value
	}
	return copyOfSelection
}
