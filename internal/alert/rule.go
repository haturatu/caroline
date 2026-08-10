package alert

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"caroline/internal/explorer"
)

const (
	StatusOK     = "OK"
	StatusFiring = "FIRING"

	maxRuleWindow   = 7 * 24 * time.Hour
	maxRuleCooldown = 30 * 24 * time.Hour
)

type Rule struct {
	ID              string
	Name            string
	Query           string
	Threshold       int
	WindowSeconds   int
	CooldownSeconds int
	Enabled         bool
	WebhookURL      string
}

type RuleSpec struct {
	Name            string `json:"name"`
	Query           string `json:"query"`
	Threshold       int    `json:"threshold"`
	WindowSeconds   int    `json:"windowSeconds"`
	CooldownSeconds int    `json:"cooldownSeconds"`
	Enabled         *bool  `json:"enabled"`
	WebhookURL      string `json:"webhookUrl"`
}

type RuleState struct {
	Status      string
	Matches     []time.Time
	LastFiredAt *time.Time
	UpdatedAt   time.Time
}

type RuleView struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Query             string     `json:"query"`
	Threshold         int        `json:"threshold"`
	WindowSeconds     int        `json:"windowSeconds"`
	CooldownSeconds   int        `json:"cooldownSeconds"`
	Enabled           bool       `json:"enabled"`
	WebhookConfigured bool       `json:"webhookConfigured"`
	Status            string     `json:"status"`
	MatchCount        int        `json:"matchCount"`
	LastFiredAt       *time.Time `json:"lastFiredAt,omitempty"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func normalizeSpec(spec RuleSpec) (Rule, error) {
	rule := Rule{
		Name:            strings.TrimSpace(spec.Name),
		Query:           strings.TrimSpace(spec.Query),
		Threshold:       spec.Threshold,
		WindowSeconds:   spec.WindowSeconds,
		CooldownSeconds: spec.CooldownSeconds,
		Enabled:         spec.Enabled == nil || *spec.Enabled,
		WebhookURL:      strings.TrimSpace(spec.WebhookURL),
	}
	if rule.Name == "" {
		return Rule{}, fmt.Errorf("alert name is required")
	}
	if rule.Threshold < 1 || rule.Threshold > 1_000_000 {
		return Rule{}, fmt.Errorf("threshold must be between 1 and 1000000")
	}
	if rule.WindowSeconds < 1 || time.Duration(rule.WindowSeconds)*time.Second > maxRuleWindow {
		return Rule{}, fmt.Errorf("windowSeconds must be between 1 and %d", int(maxRuleWindow/time.Second))
	}
	if rule.CooldownSeconds < 0 || time.Duration(rule.CooldownSeconds)*time.Second > maxRuleCooldown {
		return Rule{}, fmt.Errorf("cooldownSeconds must be between 0 and %d", int(maxRuleCooldown/time.Second))
	}
	if rule.WebhookURL != "" {
		parsed, err := url.ParseRequestURI(rule.WebhookURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Rule{}, fmt.Errorf("webhookUrl must be an http or https URL")
		}
	}
	return rule, nil
}

func (r Rule) window() time.Duration {
	return time.Duration(r.WindowSeconds) * time.Second
}

func (r Rule) cooldown() time.Duration {
	return time.Duration(r.CooldownSeconds) * time.Second
}

func (r Rule) view(state RuleState, now time.Time) RuleView {
	trimmed := trimMatches(state.Matches, now.Add(-r.window()))
	return RuleView{
		ID:                r.ID,
		Name:              r.Name,
		Query:             r.Query,
		Threshold:         r.Threshold,
		WindowSeconds:     r.WindowSeconds,
		CooldownSeconds:   r.CooldownSeconds,
		Enabled:           r.Enabled,
		WebhookConfigured: r.WebhookURL != "",
		Status:            state.Status,
		MatchCount:        len(trimmed),
		LastFiredAt:       state.LastFiredAt,
		UpdatedAt:         state.UpdatedAt,
	}
}

func trimMatches(matches []time.Time, cutoff time.Time) []time.Time {
	first := 0
	for first < len(matches) && matches[first].Before(cutoff) {
		first++
	}
	if first == 0 {
		return matches
	}
	return matches[first:]
}

func matchesRule(entry explorer.Entry, rule Rule) bool {
	return explorer.MatchesFilters(entry, rule.Query, "", "")
}
