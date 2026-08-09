package explorer

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

func ParseDuration(value string) time.Duration {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "pt") {
		value = strings.TrimPrefix(value, "pt")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err == nil {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 5 * time.Minute
	}
	if parsed > 30*24*time.Hour {
		return 30 * 24 * time.Hour
	}
	return parsed
}

func BuildTimeline(entries []Entry, from, to time.Time) []TimelineBucket {
	const bucketCount = 24
	duration := to.Sub(from)
	if duration <= 0 {
		return nil
	}
	buckets := make([]TimelineBucket, bucketCount)
	span := duration / bucketCount
	for index := range buckets {
		start := from.Add(time.Duration(index) * span)
		end := start.Add(span)
		if index == bucketCount-1 {
			end = to
		}
		buckets[index] = TimelineBucket{Start: start, End: end, Severities: map[string]int{"DEBUG": 0, "INFO": 0, "WARNING": 0, "ERROR": 0}}
	}
	for _, entry := range entries {
		index := int(entry.Timestamp.Sub(from) / span)
		if index < 0 {
			index = 0
		}
		if index >= bucketCount {
			index = bucketCount - 1
		}
		buckets[index].Total++
		severity := entry.Severity
		if SeverityRank(severity) >= SeverityRank("ERROR") {
			severity = "ERROR"
		} else if SeverityRank(severity) >= SeverityRank("WARNING") {
			severity = "WARNING"
		} else if SeverityRank(severity) <= SeverityRank("DEBUG") {
			severity = "DEBUG"
		} else {
			severity = "INFO"
		}
		buckets[index].Severities[severity]++
	}
	return buckets
}

func BuildFieldGroups(entries []Entry) []FieldGroup {
	type counter struct {
		count  int
		values map[string]int
	}
	groups := map[string]map[string]*counter{
		"System Metadata": {},
		"Frequent Fields": {},
	}
	add := func(group, name, value string) {
		if value == "" {
			return
		}
		field, ok := groups[group][name]
		if !ok {
			field = &counter{values: map[string]int{}}
			groups[group][name] = field
		}
		field.count++
		if len(field.values) < 8 {
			field.values[value]++
		}
	}
	for _, entry := range entries {
		add("System Metadata", "severity", entry.Severity)
		add("System Metadata", "resource.type", entry.Resource.Type)
		add("System Metadata", "resource.labels.container_name", entry.Resource.Labels["container_name"])
		add("System Metadata", "logName", entry.LogName)
		for key, value := range entry.JSONPayload {
			if scalar, ok := value.(string); ok {
				add("Frequent Fields", "jsonPayload."+key, scalar)
			}
		}
	}
	result := make([]FieldGroup, 0, 2)
	for _, groupName := range []string{"Pinned", "System Metadata", "Frequent Fields"} {
		fields := make([]FieldValue, 0)
		for name, field := range groups[groupName] {
			fields = append(fields, FieldValue{Name: name, Count: field.count, Values: field.values})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Count > fields[j].Count })
		if len(fields) > 10 {
			fields = fields[:10]
		}
		if len(fields) > 0 {
			result = append(result, FieldGroup{Name: groupName, Fields: fields})
		}
	}
	return result
}
