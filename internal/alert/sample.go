package alert

import (
	"regexp"
	"strings"

	"caroline/internal/explorer"
)

var (
	redactAuthorization = regexp.MustCompile(`(?i)(\bauthorization\s*:\s*bearer\s+)[^\s]+`)
	redactBearer        = regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9._~+/=-]+`)
	redactKeyValue      = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|api[_-]?key|cookie|database[_-]?url|encryption[_-]?key|password|passwd|pwd|refresh[_-]?token|secret|session[_-]?id|token)\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;}]+)`)
	redactCredentials   = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://[^/\s:@]+:)[^@\s]+(@)`)
	redactEmail         = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
)

const redactedValue = "[REDACTED]"

// SanitizeNotification protects webhook boundaries even when a notifier is
// called with a notification assembled outside the alert engine.
func SanitizeNotification(notification Notification) Notification {
	notification.Sample = sampleForNotification(SampleModeFull, notification.Sample)
	return notification
}

func sampleForNotification(mode string, entry *explorer.Entry) *explorer.Entry {
	if entry == nil || mode == SampleModeOff {
		return nil
	}

	copyOfEntry := *entry
	copyOfEntry.Summary = redactSensitiveText(copyOfEntry.Summary)
	copyOfEntry.Labels = cloneLabels(copyOfEntry.Labels)
	copyOfEntry.Resource.Labels = cloneLabels(copyOfEntry.Resource.Labels)
	if mode == SampleModeFull {
		copyOfEntry.TextPayload = redactSensitiveText(copyOfEntry.TextPayload)
		copyOfEntry.JSONPayload = redactJSONPayload(copyOfEntry.JSONPayload)
	} else {
		copyOfEntry.TextPayload = ""
		copyOfEntry.JSONPayload = nil
	}
	return &copyOfEntry
}

func redactSensitiveText(value string) string {
	value = redactAuthorization.ReplaceAllString(value, `${1}`+redactedValue)
	value = redactBearer.ReplaceAllString(value, `${1}`+redactedValue)
	value = redactKeyValue.ReplaceAllString(value, `${1}`+redactedValue)
	value = redactCredentials.ReplaceAllString(value, `${1}`+redactedValue+`${2}`)
	return redactEmail.ReplaceAllString(value, redactedValue)
}

func redactJSONPayload(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	redacted, ok := redactJSONValue(value).(map[string]any)
	if !ok {
		return nil
	}
	return redacted
}

func redactJSONValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(current))
		for key, item := range current {
			if isSensitiveKey(key) {
				result[key] = redactedValue
				continue
			}
			result[key] = redactJSONValue(item)
		}
		return result
	case []any:
		result := make([]any, len(current))
		for index, item := range current {
			result[index] = redactJSONValue(item)
		}
		return result
	case string:
		return redactSensitiveText(current)
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch normalized {
	case "accesstoken", "apikey", "authorization", "cookie", "databaseurl", "encryptionkey", "password", "passwd", "pwd", "refreshtoken", "secret", "sessionid", "token":
		return true
	default:
		return strings.Contains(normalized, "password") ||
			strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "secret")
	}
}

func entryContainer(entry *explorer.Entry) string {
	if entry == nil {
		return ""
	}
	for _, labels := range []map[string]string{entry.Resource.Labels, entry.Labels} {
		for _, key := range []string{"container_name", "container", "service"} {
			if value := strings.TrimSpace(labels[key]); value != "" {
				return value
			}
		}
	}
	return ""
}
