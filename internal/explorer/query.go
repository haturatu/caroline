package explorer

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var explorerClause = regexp.MustCompile("^\\s*([A-Za-z0-9_.-]+)\\s*(>=|<=|!=|=|:|>|<)\\s*(?:\"([^\"]*)\"|`([^`]*)`|([^\\s]+))\\s*$")

func splitQueryOperator(input, operator string) []string {
	parts := make([]string, 0, 2)
	start := 0
	inDoubleQuote := false
	inBacktick := false
	for index := 0; index <= len(input)-len(operator); index++ {
		switch input[index] {
		case '"':
			if !inBacktick && (index == 0 || input[index-1] != '\\') {
				inDoubleQuote = !inDoubleQuote
			}
		case '`':
			if !inDoubleQuote {
				inBacktick = !inBacktick
			}
		}
		if inDoubleQuote || inBacktick || !strings.EqualFold(input[index:index+len(operator)], operator) {
			continue
		}
		leftBoundary := index == 0 || input[index-1] == ' ' || input[index-1] == '\n' || input[index-1] == '\t'
		rightIndex := index + len(operator)
		rightBoundary := rightIndex == len(input) || input[rightIndex] == ' ' || input[rightIndex] == '\n' || input[rightIndex] == '\t'
		if leftBoundary && rightBoundary {
			parts = append(parts, strings.TrimSpace(input[start:index]))
			start = rightIndex
			index = rightIndex - 1
		}
	}
	parts = append(parts, strings.TrimSpace(input[start:]))
	return parts
}

func queryValue(entry Entry, field string) []string {
	field = strings.TrimSpace(field)
	switch field {
	case "severity":
		return []string{entry.Severity}
	case "textPayload", "message":
		return []string{entry.TextPayload}
	case "stream", "labels.stream":
		return []string{entry.Stream}
	case "logName":
		return []string{entry.LogName}
	case "resource.type":
		return []string{entry.Resource.Type}
	case "resource.labels.container_name", "container", "container.name":
		return []string{entry.Resource.Labels["container_name"]}
	case "resource.labels.container_id", "container.id":
		return []string{entry.Resource.Labels["container_id"]}
	case "resource.labels.image", "container.image":
		return []string{entry.Resource.Labels["image"]}
	case "timestamp":
		return []string{entry.Timestamp.Format(time.RFC3339Nano)}
	}
	if strings.HasPrefix(field, "labels.") {
		return []string{entry.Labels[strings.TrimPrefix(field, "labels.")]}
	}
	if strings.HasPrefix(field, "jsonPayload.") {
		var current any = entry.JSONPayload
		for _, part := range strings.Split(strings.TrimPrefix(field, "jsonPayload."), ".") {
			object, ok := current.(map[string]any)
			if !ok {
				return nil
			}
			current = object[part]
		}
		if current == nil {
			return nil
		}
		return []string{fmt.Sprint(current)}
	}
	return nil
}

func SeverityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case "DEBUG":
		return 100
	case "INFO":
		return 200
	case "NOTICE":
		return 300
	case "WARNING":
		return 400
	case "ERROR":
		return 500
	case "CRITICAL":
		return 600
	case "ALERT":
		return 700
	case "EMERGENCY":
		return 800
	default:
		return 0
	}
}

func compareExplorerValue(actual, operator, expected string, field string) bool {
	if field == "severity" {
		left, right := SeverityRank(actual), SeverityRank(expected)
		switch operator {
		case ">=":
			return left >= right
		case "<=":
			return left <= right
		case ">":
			return left > right
		case "<":
			return left < right
		case "=":
			return strings.EqualFold(actual, expected)
		case "!=":
			return !strings.EqualFold(actual, expected)
		}
	}
	if operator == ":" {
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
	}
	if field == "timestamp" {
		left, leftErr := time.Parse(time.RFC3339Nano, actual)
		right, rightErr := time.Parse(time.RFC3339Nano, expected)
		if leftErr == nil && rightErr == nil {
			switch operator {
			case ">=":
				return !left.Before(right)
			case "<=":
				return !left.After(right)
			case ">":
				return left.After(right)
			case "<":
				return left.Before(right)
			}
		}
	}
	switch operator {
	case "=":
		return strings.EqualFold(actual, expected)
	case "!=":
		return !strings.EqualFold(actual, expected)
	case ">":
		return actual > expected
	case "<":
		return actual < expected
	case ">=":
		return actual >= expected
	case "<=":
		return actual <= expected
	default:
		return false
	}
}

func containsQueryTokens(value, query string) bool {
	value = strings.ToLower(value)
	query = strings.TrimSpace(query)
	if len(query) >= 2 && ((query[0] == '"' && query[len(query)-1] == '"') || (query[0] == '`' && query[len(query)-1] == '`')) {
		return strings.Contains(value, strings.ToLower(query[1:len(query)-1]))
	}
	for _, token := range strings.Fields(strings.Trim(query, "\"`")) {
		if !strings.Contains(value, strings.ToLower(token)) {
			return false
		}
	}
	return true
}

func matchesExplorerClause(entry Entry, clause string) bool {
	clause = strings.TrimSpace(clause)
	upper := strings.ToUpper(clause)
	if strings.HasPrefix(upper, "SEARCH(") && strings.HasSuffix(clause, ")") {
		query := strings.TrimSpace(clause[len("SEARCH(") : len(clause)-1])
		if len(query) >= 2 && query[0] != '"' && query[0] != '`' {
			parts := strings.SplitN(query, ",", 2)
			query = strings.TrimSpace(parts[len(parts)-1])
		}
		return containsQueryTokens(explorerSearchText(entry), query)
	}
	matches := explorerClause.FindStringSubmatch(clause)
	if len(matches) == 0 {
		return containsQueryTokens(explorerSearchText(entry), clause)
	}
	expected := matches[3]
	if expected == "" {
		expected = matches[4]
	}
	if expected == "" {
		expected = matches[5]
	}
	for _, actual := range queryValue(entry, matches[1]) {
		if actual != "" && compareExplorerValue(actual, matches[2], expected, matches[1]) {
			return true
		}
	}
	return false
}

func NormalizeQuery(query string) string {
	var normalized strings.Builder
	normalized.Grow(len(query) + 8)
	inDoubleQuote := false
	inBacktick := false
	for index := 0; index < len(query); index++ {
		character := query[index]
		if character == '"' && !inBacktick && (index == 0 || query[index-1] != '\\') {
			inDoubleQuote = !inDoubleQuote
		}
		if character == '`' && !inDoubleQuote {
			inBacktick = !inBacktick
		}
		if !inDoubleQuote && !inBacktick && (character == '\n' || character == '\r') {
			normalized.WriteString(" AND ")
			if character == '\r' && index+1 < len(query) && query[index+1] == '\n' {
				index++
			}
			continue
		}
		normalized.WriteByte(character)
	}
	return strings.TrimSpace(normalized.String())
}

func MatchesQuery(entry Entry, query string) bool {
	query = NormalizeQuery(query)
	if query == "" {
		return true
	}
	for _, orPart := range splitQueryOperator(query, "OR") {
		matched := true
		for _, andPart := range splitQueryOperator(orPart, "AND") {
			if !matchesExplorerClause(entry, andPart) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
