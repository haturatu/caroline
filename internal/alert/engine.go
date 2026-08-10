package alert

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"caroline/internal/explorer"
	"caroline/internal/logstream"
)

var ErrRuleNotFound = errors.New("alert rule was not found")

type Notification struct {
	Event           string          `json:"event"`
	Rule            string          `json:"rule"`
	Value           int             `json:"value"`
	Threshold       int             `json:"threshold"`
	WindowSeconds   int             `json:"windowSeconds"`
	CooldownSeconds int             `json:"cooldownSeconds"`
	Timestamp       time.Time       `json:"timestamp"`
	Sample          *explorer.Entry `json:"sample,omitempty"`
}

type Notifier interface {
	Notify(context.Context, Rule, Notification) error
}

type Engine struct {
	streams  *logstream.Manager
	notifier Notifier

	mu     sync.RWMutex
	rules  map[string]Rule
	states map[string]RuleState
}

func NewEngine(streams *logstream.Manager, notifier Notifier) *Engine {
	return &Engine{
		streams:  streams,
		notifier: notifier,
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
	rule.ID = id
	e.rules[id] = rule
	e.states[id] = RuleState{Status: StatusOK, UpdatedAt: now}
	view := rule.view(e.states[id], now)
	e.mu.Unlock()
	return view, nil
}

func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("%w: %q", ErrRuleNotFound, id)
	}
	delete(e.rules, id)
	delete(e.states, id)
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
	e.mu.Lock()
	for id, rule := range e.rules {
		if !rule.Enabled || !matchesRule(entry, rule) {
			continue
		}
		state := e.states[id]
		state.Matches = append(trimMatches(state.Matches, now.Add(-rule.window())), now)
		state.UpdatedAt = now
		if len(state.Matches) >= rule.Threshold {
			if state.Status != StatusFiring {
				state.Status = StatusFiring
				if state.LastFiredAt == nil || now.Sub(*state.LastFiredAt) >= rule.cooldown() {
					firedAt := now
					state.LastFiredAt = &firedAt
					notifications = append(notifications, struct {
						rule Rule
						item Notification
					}{rule: rule, item: notificationFor(rule, "alert.firing", len(state.Matches), now, &entry)})
				}
			}
		}
		e.states[id] = state
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
	e.mu.Lock()
	for id, rule := range e.rules {
		state := e.states[id]
		state.Matches = trimMatches(state.Matches, now.Add(-rule.window()))
		if state.Status == StatusFiring && len(state.Matches) < rule.Threshold {
			state.Status = StatusOK
			state.UpdatedAt = now
			notifications = append(notifications, struct {
				rule Rule
				item Notification
			}{rule: rule, item: notificationFor(rule, "alert.resolved", len(state.Matches), now, nil)})
		}
		e.states[id] = state
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

func notificationFor(rule Rule, event string, value int, now time.Time, sample *explorer.Entry) Notification {
	return Notification{
		Event:           event,
		Rule:            rule.Name,
		Value:           value,
		Threshold:       rule.Threshold,
		WindowSeconds:   rule.WindowSeconds,
		CooldownSeconds: rule.CooldownSeconds,
		Timestamp:       now,
		Sample:          sample,
	}
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
