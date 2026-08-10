package alert

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"caroline/internal/explorer"
	"caroline/internal/logstream"
)

var (
	ErrRuleNotFound = errors.New("alert rule was not found")
	ErrPersistence  = errors.New("alert persistence failed")
)

const (
	alertStoreVersion         = 3
	legacyAlertStoreVersion   = 1
	previousAlertStoreVersion = 2
)

type Notification struct {
	Event           string            `json:"event"`
	RuleID          string            `json:"ruleId"`
	Rule            string            `json:"rule"`
	Severity        string            `json:"severity,omitempty"`
	Query           string            `json:"query"`
	Value           int               `json:"value"`
	PeakValue       int               `json:"peakValue,omitempty"`
	Threshold       int               `json:"threshold"`
	WindowSeconds   int               `json:"windowSeconds"`
	CooldownSeconds int               `json:"cooldownSeconds"`
	Container       string            `json:"container,omitempty"`
	StartedAt       *time.Time        `json:"startedAt,omitempty"`
	Timestamp       time.Time         `json:"timestamp"`
	ExplorerURL     string            `json:"explorerUrl,omitempty"`
	RunbookURL      string            `json:"runbookUrl,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Sample          *explorer.Entry   `json:"sample,omitempty"`
}

type Notifier interface {
	Notify(context.Context, Rule, Notification) error
}

type Engine struct {
	streams  *logstream.Manager
	notifier Notifier
	store    string

	mu     sync.RWMutex
	rules  map[string]Rule
	states map[string]RuleState
}

func NewEngine(streams *logstream.Manager, notifier Notifier) *Engine {
	return newEngine(streams, notifier, "")
}

func NewEngineWithPersistence(streams *logstream.Manager, notifier Notifier, path string) (*Engine, error) {
	engine := newEngine(streams, notifier, strings.TrimSpace(path))
	if engine.store == "" {
		return engine, nil
	}
	if err := engine.load(); err != nil {
		return nil, fmt.Errorf("load alert store: %w", err)
	}
	return engine, nil
}

func newEngine(streams *logstream.Manager, notifier Notifier, store string) *Engine {
	return &Engine{
		streams:  streams,
		notifier: notifier,
		store:    store,
		rules:    make(map[string]Rule),
		states:   make(map[string]RuleState),
	}
}

func (e *Engine) Create(spec RuleSpec) (RuleView, error) {
	rule, err := normalizeSpec(spec)
	if err != nil {
		return RuleView{}, err
	}
	rule.ID = newRuleID()
	now := time.Now().UTC()
	e.mu.Lock()
	e.rules[rule.ID] = rule
	e.states[rule.ID] = RuleState{Status: StatusOK, UpdatedAt: now}
	view := rule.view(e.states[rule.ID], now)
	if err := e.saveLocked(); err != nil {
		delete(e.rules, rule.ID)
		delete(e.states, rule.ID)
		e.mu.Unlock()
		return RuleView{}, fmt.Errorf("%w: %v", ErrPersistence, err)
	}
	e.mu.Unlock()
	return view, nil
}

func (e *Engine) Update(id string, spec RuleSpec) (RuleView, error) {
	rule, err := normalizeSpec(spec)
	if err != nil {
		return RuleView{}, err
	}
	now := time.Now().UTC()
	e.mu.Lock()
	if _, ok := e.rules[id]; !ok {
		e.mu.Unlock()
		return RuleView{}, fmt.Errorf("%w: %q", ErrRuleNotFound, id)
	}
	previousRule := e.rules[id]
	previousState := e.states[id]
	rule.ID = id
	e.rules[id] = rule
	e.states[id] = RuleState{Status: StatusOK, UpdatedAt: now}
	view := rule.view(e.states[id], now)
	if err := e.saveLocked(); err != nil {
		e.rules[id] = previousRule
		e.states[id] = previousState
		e.mu.Unlock()
		return RuleView{}, fmt.Errorf("%w: %v", ErrPersistence, err)
	}
	e.mu.Unlock()
	return view, nil
}

func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("%w: %q", ErrRuleNotFound, id)
	}
	previousRule := e.rules[id]
	previousState := e.states[id]
	delete(e.rules, id)
	delete(e.states, id)
	if err := e.saveLocked(); err != nil {
		e.rules[id] = previousRule
		e.states[id] = previousState
		return fmt.Errorf("%w: %v", ErrPersistence, err)
	}
	return nil
}

func (e *Engine) List() []RuleView {
	now := time.Now().UTC()
	e.mu.Lock()
	defer e.mu.Unlock()
	views := make([]RuleView, 0, len(e.rules))
	for id, rule := range e.rules {
		state := e.states[id]
		state.Matches = trimMatches(state.Matches, now.Add(-rule.window()))
		e.states[id] = state
		views = append(views, rule.view(state, now))
	}
	for left := 0; left < len(views); left++ {
		for right := left + 1; right < len(views); right++ {
			if views[right].Name < views[left].Name {
				views[left], views[right] = views[right], views[left]
			}
		}
	}
	return views
}

func (e *Engine) Run(ctx context.Context) error {
	for {
		subscription, err := e.streams.Subscribe(ctx, nil, time.Now().UTC(), 0)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !waitForRetry(ctx, 10*time.Second) {
				return nil
			}
			continue
		}
		err = e.consume(ctx, subscription)
		subscription.Close()
		if err == nil || ctx.Err() != nil {
			return nil
		}
		if !waitForRetry(ctx, time.Second) {
			return nil
		}
	}
}

func (e *Engine) consume(ctx context.Context, subscription *logstream.Subscription) error {
	refreshTicker := time.NewTicker(30 * time.Second)
	resolveTicker := time.NewTicker(time.Second)
	defer refreshTicker.Stop()
	defer resolveTicker.Stop()
	for {
		select {
		case entry, open := <-subscription.Entries:
			if !open {
				return fmt.Errorf("alert log subscription closed")
			}
			e.processEntry(ctx, entry)
		case _, open := <-subscription.Errors:
			if !open {
				return fmt.Errorf("alert log error subscription closed")
			}
		case <-refreshTicker.C:
			if err := e.streams.Refresh(ctx); err != nil && ctx.Err() == nil {
				log.Printf("alert stream refresh failed: %v", err)
			}
		case <-resolveTicker.C:
			e.resolve(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

func (e *Engine) processEntry(ctx context.Context, entry explorer.Entry) {
	now := time.Now().UTC()
	notifications := make([]struct {
		rule Rule
		item Notification
	}, 0)
	changed := false
	e.mu.Lock()
	for id, rule := range e.rules {
		if !rule.Enabled || !matchesRule(entry, rule) {
			continue
		}
		state := e.states[id]
		state.Matches = append(trimMatches(state.Matches, now.Add(-rule.window())), now)
		state.UpdatedAt = now
		changed = true
		if len(state.Matches) >= rule.Threshold {
			if state.Status == StatusFiring && len(state.Matches) > state.PeakMatches {
				state.PeakMatches = len(state.Matches)
			}
			if state.Status != StatusFiring {
				state.Status = StatusFiring
				state.FiringNotificationSent = false
				firingSince := now
				state.FiringSince = &firingSince
				state.PeakMatches = len(state.Matches)
				state.Container = entryContainer(&entry)
				if state.LastFiredAt == nil || now.Sub(*state.LastFiredAt) >= rule.cooldown() {
					firedAt := now
					state.LastFiredAt = &firedAt
					state.FiringNotificationSent = true
					notifications = append(notifications, struct {
						rule Rule
						item Notification
					}{rule: rule, item: notificationFor(rule, "alert.firing", len(state.Matches), state.PeakMatches, now, state.FiringSince, state.Container, &entry)})
				}
			}
		}
		e.states[id] = state
	}
	if changed {
		if err := e.saveLocked(); err != nil {
			log.Printf("alert persistence failed: %v", err)
		}
	}
	e.mu.Unlock()
	e.notify(ctx, notifications)
}

func (e *Engine) resolve(ctx context.Context) {
	now := time.Now().UTC()
	notifications := make([]struct {
		rule Rule
		item Notification
	}, 0)
	changed := false
	e.mu.Lock()
	for id, rule := range e.rules {
		state := e.states[id]
		previousMatches := len(state.Matches)
		state.Matches = trimMatches(state.Matches, now.Add(-rule.window()))
		if len(state.Matches) != previousMatches {
			changed = true
		}
		if state.Status == StatusFiring && len(state.Matches) < rule.Threshold {
			resolved := notificationFor(rule, "alert.resolved", len(state.Matches), state.PeakMatches, now, state.FiringSince, state.Container, nil)
			state.Status = StatusOK
			state.UpdatedAt = now
			changed = true
			if state.FiringNotificationSent {
				notifications = append(notifications, struct {
					rule Rule
					item Notification
				}{rule: rule, item: resolved})
			}
			state.FiringSince = nil
			state.PeakMatches = 0
			state.Container = ""
			state.FiringNotificationSent = false
		}
		e.states[id] = state
	}
	if changed {
		if err := e.saveLocked(); err != nil {
			log.Printf("alert persistence failed: %v", err)
		}
	}
	e.mu.Unlock()
	e.notify(ctx, notifications)
}

func (e *Engine) notify(ctx context.Context, notifications []struct {
	rule Rule
	item Notification
}) {
	if e.notifier == nil {
		return
	}
	for _, notification := range notifications {
		if err := e.notifier.Notify(ctx, notification.rule, notification.item); err != nil {
			log.Printf("alert notification failed for %q: %v", notification.rule.Name, err)
		}
	}
}

func notificationFor(rule Rule, event string, value, peakValue int, now time.Time, startedAt *time.Time, container string, sample *explorer.Entry) Notification {
	if container == "" {
		container = entryContainer(sample)
	}
	return Notification{
		Event:           event,
		RuleID:          rule.ID,
		Rule:            rule.Name,
		Severity:        rule.Severity,
		Query:           rule.Query,
		Value:           value,
		PeakValue:       peakValue,
		Threshold:       rule.Threshold,
		WindowSeconds:   rule.WindowSeconds,
		CooldownSeconds: rule.CooldownSeconds,
		Container:       container,
		StartedAt:       cloneTime(startedAt),
		Timestamp:       now,
		RunbookURL:      rule.RunbookURL,
		Labels:          cloneLabels(rule.Labels),
		Sample:          sampleForNotification(rule.SampleMode, sample),
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func waitForRetry(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func newRuleID() string {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err == nil {
		return "rule-" + hex.EncodeToString(randomBytes)
	}
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}

type alertStore struct {
	Version int                  `json:"version"`
	Rules   []storedRule         `json:"rules"`
	States  map[string]RuleState `json:"states"`
}

type storedRule struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Query           string            `json:"query"`
	Severity        string            `json:"severity,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	RunbookURL      string            `json:"runbookUrl,omitempty"`
	SampleMode      string            `json:"sampleMode,omitempty"`
	Threshold       int               `json:"threshold"`
	WindowSeconds   int               `json:"windowSeconds"`
	CooldownSeconds int               `json:"cooldownSeconds"`
	Enabled         bool              `json:"enabled"`
	WebhookURL      string            `json:"webhookUrl"`
}

func (e *Engine) load() error {
	file, err := os.Open(e.store)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %q: %w", e.store, err)
	}
	defer file.Close()

	var store alertStore
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&store); err != nil {
		return fmt.Errorf("decode %q: %w", e.store, err)
	}
	if store.Version != legacyAlertStoreVersion && store.Version != previousAlertStoreVersion && store.Version != alertStoreVersion {
		return fmt.Errorf("unsupported alert store version %d", store.Version)
	}

	now := time.Now().UTC()
	for _, saved := range store.Rules {
		if saved.ID == "" {
			return errors.New("alert store contains a rule without an id")
		}
		enabled := saved.Enabled
		rule, err := normalizeSpec(RuleSpec{
			Name:            saved.Name,
			Query:           saved.Query,
			Severity:        saved.Severity,
			Labels:          saved.Labels,
			RunbookURL:      saved.RunbookURL,
			SampleMode:      saved.SampleMode,
			Threshold:       saved.Threshold,
			WindowSeconds:   saved.WindowSeconds,
			CooldownSeconds: saved.CooldownSeconds,
			Enabled:         &enabled,
			WebhookURL:      saved.WebhookURL,
		})
		if err != nil {
			return fmt.Errorf("validate rule %q: %w", saved.ID, err)
		}
		if _, exists := e.rules[saved.ID]; exists {
			return fmt.Errorf("alert store contains duplicate rule id %q", saved.ID)
		}
		rule.ID = saved.ID
		state := store.States[saved.ID]
		if state.Status == "" {
			state.Status = StatusOK
		}
		if store.Version == legacyAlertStoreVersion && state.Status == StatusFiring {
			// Version 1 did not persist whether the firing notification was
			// sent. Treat an active legacy rule as notified during migration.
			state.FiringNotificationSent = true
		}
		if state.Status != StatusOK && state.Status != StatusFiring {
			return fmt.Errorf("alert store contains invalid status %q for rule %q", state.Status, saved.ID)
		}
		state.Matches = trimMatches(state.Matches, now.Add(-rule.window()))
		if state.Status == StatusFiring && len(state.Matches) < rule.Threshold {
			state.Status = StatusOK
			state.FiringSince = nil
			state.PeakMatches = 0
			state.Container = ""
			state.FiringNotificationSent = false
		}
		if state.Status == StatusFiring {
			if state.FiringSince == nil {
				candidate := state.LastFiredAt
				if candidate == nil && !state.UpdatedAt.IsZero() {
					candidate = &state.UpdatedAt
				}
				state.FiringSince = cloneTime(candidate)
			}
			if state.PeakMatches < len(state.Matches) {
				state.PeakMatches = len(state.Matches)
			}
		}
		if state.UpdatedAt.IsZero() {
			state.UpdatedAt = now
		}
		e.rules[saved.ID] = rule
		e.states[saved.ID] = state
	}
	return nil
}

func (e *Engine) saveLocked() error {
	if e.store == "" {
		return nil
	}

	ids := make([]string, 0, len(e.rules))
	for id := range e.rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	store := alertStore{
		Version: alertStoreVersion,
		Rules:   make([]storedRule, 0, len(ids)),
		States:  make(map[string]RuleState, len(ids)),
	}
	for _, id := range ids {
		rule := e.rules[id]
		store.Rules = append(store.Rules, storedRule{
			ID:              rule.ID,
			Name:            rule.Name,
			Query:           rule.Query,
			Severity:        rule.Severity,
			Labels:          cloneLabels(rule.Labels),
			RunbookURL:      rule.RunbookURL,
			SampleMode:      rule.SampleMode,
			Threshold:       rule.Threshold,
			WindowSeconds:   rule.WindowSeconds,
			CooldownSeconds: rule.CooldownSeconds,
			Enabled:         rule.Enabled,
			WebhookURL:      rule.WebhookURL,
		})
		state := e.states[id]
		state.Matches = append([]time.Time(nil), state.Matches...)
		store.States[id] = state
	}

	directory := filepath.Dir(e.store)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("create alert store directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(e.store)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary alert store: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set alert store permissions: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(store); err != nil {
		return fmt.Errorf("encode alert store: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync alert store: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close alert store: %w", err)
	}
	if err := os.Rename(temporaryName, e.store); err != nil {
		return fmt.Errorf("replace alert store: %w", err)
	}
	return nil
}
