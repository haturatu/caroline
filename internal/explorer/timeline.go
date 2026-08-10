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
		daysValue := strings.TrimSuffix(value, "d")
		days, err := strconv.ParseInt(daysValue, 10, 64)
		if err == nil && days > 0 {
			if days > 30 {
				return 30 * 24 * time.Hour
			}
			return time.Duration(days) * 24 * time.Hour
		}
		if err != nil && isPositiveDecimal(daysValue) {
			return 30 * 24 * time.Hour
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

func isPositiveDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func NormalizeTimelineBuckets(value int) int {
	if value <= 0 {
		return DefaultTimelineBuckets
	}
	if value < MinTimelineBuckets {
		return MinTimelineBuckets
	}
	if value > MaxTimelineBuckets {
		return MaxTimelineBuckets
	}
	return value
}

func BuildTimeline(entries []Entry, from, to time.Time, requestedBucketCount int) []TimelineBucket {
	duration := to.Sub(from)
	if duration <= 0 {
		return nil
	}
	bucketCount := NormalizeTimelineBuckets(requestedBucketCount)
	if duration/time.Duration(bucketCount) <= 0 {
		bucketCount = 1
	}
	buckets := make([]TimelineBucket, bucketCount)
	span := duration / time.Duration(bucketCount)
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
		rank, known := severityRank(severity)
		if !known {
			severity = "INFO"
		} else if rank >= SeverityRank("ERROR") {
			severity = "ERROR"
		} else if rank >= SeverityRank("WARNING") {
			severity = "WARNING"
		} else if rank <= SeverityRank("DEBUG") {
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
		field.values[value]++
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
			fields = append(fields, FieldValue{Name: name, Count: field.count, Values: topFieldValues(field.values, 8)})
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

func topFieldValues(values map[string]int, limit int) map[string]int {
	type valueCount struct {
		value string
		count int
	}
	counts := make([]valueCount, 0, len(values))
	for value, count := range values {
		counts = append(counts, valueCount{value: value, count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].count == counts[j].count {
			return counts[i].value < counts[j].value
		}
		return counts[i].count > counts[j].count
	})
	if len(counts) > limit {
		counts = counts[:limit]
	}
	top := make(map[string]int, len(counts))
	for _, item := range counts {
		top[item.value] = item.count
	}
	return top
}
