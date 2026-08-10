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

	SampleModeOff     = "off"
	SampleModeSummary = "summary"
	SampleModeFull    = "full"

	maxRuleWindow     = 7 * 24 * time.Hour
	maxRuleCooldown   = 30 * 24 * time.Hour
	maxRuleLabels     = 32
	maxLabelKeySize   = 64
	maxLabelValueSize = 256
)

type Rule struct {
	ID              string
	Name            string
	Query           string
	Severity        string
	Labels          map[string]string
	RunbookURL      string
	SampleMode      string
	Threshold       int
	WindowSeconds   int
	CooldownSeconds int
	Enabled         bool
	WebhookURL      string
}

type RuleSpec struct {
	Name            string            `json:"name"`
	Query           string            `json:"query"`
	Severity        string            `json:"severity"`
	Labels          map[string]string `json:"labels"`
	RunbookURL      string            `json:"runbookUrl"`
	SampleMode      string            `json:"sampleMode"`
	Threshold       int               `json:"threshold"`
	WindowSeconds   int               `json:"windowSeconds"`
	CooldownSeconds int               `json:"cooldownSeconds"`
	Enabled         *bool             `json:"enabled"`
	WebhookURL      string            `json:"webhookUrl"`
}

type RuleState struct {
	Status                 string      `json:"status"`
	Matches                []time.Time `json:"matches"`
	LastFiredAt            *time.Time  `json:"lastFiredAt,omitempty"`
	FiringSince            *time.Time  `json:"firingSince,omitempty"`
	PeakMatches            int         `json:"peakMatches,omitempty"`
	Container              string      `json:"container,omitempty"`
	FiringNotificationSent bool        `json:"firingNotificationSent"`
	UpdatedAt              time.Time   `json:"updatedAt"`
}

type RuleView struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Query             string            `json:"query"`
	Severity          string            `json:"severity,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	RunbookURL        string            `json:"runbookUrl,omitempty"`
	SampleMode        string            `json:"sampleMode"`
	Threshold         int               `json:"threshold"`
	WindowSeconds     int               `json:"windowSeconds"`
	CooldownSeconds   int               `json:"cooldownSeconds"`
	Enabled           bool              `json:"enabled"`
	WebhookConfigured bool              `json:"webhookConfigured"`
	Status            string            `json:"status"`
	MatchCount        int               `json:"matchCount"`
	LastFiredAt       *time.Time        `json:"lastFiredAt,omitempty"`
	FiringSince       *time.Time        `json:"firingSince,omitempty"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

func normalizeSpec(spec RuleSpec) (Rule, error) {
	severity := strings.ToLower(strings.TrimSpace(spec.Severity))
	if len(severity) > maxLabelValueSize {
		return Rule{}, fmt.Errorf("severity must be at most %d characters", maxLabelValueSize)
	}
	labels, err := normalizeLabels(spec.Labels)
	if err != nil {
		return Rule{}, err
	}
	sampleMode := strings.ToLower(strings.TrimSpace(spec.SampleMode))
	if sampleMode == "" {
		sampleMode = SampleModeSummary
	}
	if sampleMode != SampleModeOff && sampleMode != SampleModeSummary && sampleMode != SampleModeFull {
		return Rule{}, fmt.Errorf("sampleMode must be one of %q, %q, or %q", SampleModeOff, SampleModeSummary, SampleModeFull)
	}
	rule := Rule{
		Name:            strings.TrimSpace(spec.Name),
		Query:           strings.TrimSpace(spec.Query),
		Severity:        severity,
		Labels:          labels,
		RunbookURL:      strings.TrimSpace(spec.RunbookURL),
		SampleMode:      sampleMode,
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
		if !isHTTPURL(rule.WebhookURL) {
			return Rule{}, fmt.Errorf("webhookUrl must be an http or https URL")
		}
	}
	if rule.RunbookURL != "" && !isHTTPURL(rule.RunbookURL) {
		return Rule{}, fmt.Errorf("runbookUrl must be an http or https URL")
	}
	return rule, nil
}

func isHTTPURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func normalizeLabels(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	labels := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("label keys must not be empty")
		}
		if len(key) > maxLabelKeySize {
			return nil, fmt.Errorf("label key %q is too long", key)
		}
		if len(value) > maxLabelValueSize {
			return nil, fmt.Errorf("label %q value is too long", key)
		}
		labels[key] = value
	}
	if len(labels) > maxRuleLabels {
		return nil, fmt.Errorf("labels must contain at most %d entries", maxRuleLabels)
	}
	return labels, nil
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
		Severity:          r.Severity,
		Labels:            cloneLabels(r.Labels),
		RunbookURL:        r.RunbookURL,
		SampleMode:        r.SampleMode,
		Threshold:         r.Threshold,
		WindowSeconds:     r.WindowSeconds,
		CooldownSeconds:   r.CooldownSeconds,
		Enabled:           r.Enabled,
		WebhookConfigured: r.WebhookURL != "",
		Status:            state.Status,
		MatchCount:        len(trimmed),
		LastFiredAt:       state.LastFiredAt,
		FiringSince:       state.FiringSince,
		UpdatedAt:         state.UpdatedAt,
	}
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	clone := make(map[string]string, len(labels))
	for key, value := range labels {
		clone[key] = value
	}
	return clone
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
