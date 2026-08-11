package explorer

import (
	"fmt"
	"regexp"
	"strconv"
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

type queryFieldValue struct {
	text    string
	numeric bool
}

func newQueryFieldValue(value any) queryFieldValue {
	switch value := value.(type) {
	case float64:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case float32:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case int:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case int8:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case int16:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case int32:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case int64:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case uint:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case uint8:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case uint16:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case uint32:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	case uint64:
		return queryFieldValue{text: fmt.Sprint(value), numeric: true}
	default:
		return queryFieldValue{text: fmt.Sprint(value)}
	}
}

func textQueryFieldValue(value string) queryFieldValue {
	return queryFieldValue{text: value}
}

func queryValue(entry Entry, field string) []queryFieldValue {
	field = strings.TrimSpace(field)
	switch field {
	case "severity":
		return []queryFieldValue{textQueryFieldValue(entry.Severity)}
	case "textPayload", "message":
		return []queryFieldValue{textQueryFieldValue(entry.TextPayload)}
	case "stream", "labels.stream":
		return []queryFieldValue{textQueryFieldValue(entry.Stream)}
	case "logName":
		return []queryFieldValue{textQueryFieldValue(entry.LogName)}
	case "resource.type":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Type)}
	case "resource.labels.container_name", "container", "container.name":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Labels["container_name"])}
	case "resource.labels.container_id", "container.id":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Labels["container_id"])}
	case "resource.labels.image", "container.image":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Labels["image"])}
	case "resource.labels.node_id", "node.id":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Labels["node_id"])}
	case "resource.labels.node_name", "node", "node.name":
		return []queryFieldValue{textQueryFieldValue(entry.Resource.Labels["node_name"])}
	case "timestamp":
		return []queryFieldValue{textQueryFieldValue(entry.Timestamp.Format(time.RFC3339Nano))}
	}
	if strings.HasPrefix(field, "labels.") {
		return []queryFieldValue{textQueryFieldValue(entry.Labels[strings.TrimPrefix(field, "labels.")])}
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
		return []queryFieldValue{newQueryFieldValue(current)}
	}
	return nil
}

func SeverityRank(severity string) int {
	rank, ok := severityRank(severity)
	if !ok {
		return -1
	}
	return rank
}

func severityRank(severity string) (int, bool) {
	switch strings.ToUpper(severity) {
	case "DEBUG":
		return 100, true
	case "INFO":
		return 200, true
	case "NOTICE":
		return 300, true
	case "WARNING":
		return 400, true
	case "ERROR":
		return 500, true
	case "CRITICAL":
		return 600, true
	case "ALERT":
		return 700, true
	case "EMERGENCY":
		return 800, true
	default:
		return 0, false
	}
}

func compareExplorerValue(actual queryFieldValue, operator, expected string, field string) bool {
	actualText := actual.text
	if field == "severity" {
		left, leftKnown := severityRank(actualText)
		right, rightKnown := severityRank(expected)
		switch operator {
		case ">=":
			if !leftKnown || !rightKnown {
				return false
			}
			return left >= right
		case "<=":
			if !leftKnown || !rightKnown {
				return false
			}
			return left <= right
		case ">":
			if !leftKnown || !rightKnown {
				return false
			}
			return left > right
		case "<":
			if !leftKnown || !rightKnown {
				return false
			}
			return left < right
		case "=":
			return strings.EqualFold(actualText, expected)
		case "!=":
			return !strings.EqualFold(actualText, expected)
		}
	}
	if operator == ">" || operator == "<" || operator == ">=" || operator == "<=" {
		if actual.numeric || strings.HasPrefix(field, "jsonPayload.") {
			left, leftErr := strconv.ParseFloat(actualText, 64)
			right, rightErr := strconv.ParseFloat(expected, 64)
			if leftErr == nil && rightErr == nil {
				switch operator {
				case ">":
					return left > right
				case "<":
					return left < right
				case ">=":
					return left >= right
				case "<=":
					return left <= right
				}
			}
		}
	}
	if operator == ":" {
		return strings.Contains(strings.ToLower(actualText), strings.ToLower(expected))
	}
	if field == "timestamp" {
		left, leftErr := time.Parse(time.RFC3339Nano, actualText)
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
		return strings.EqualFold(actualText, expected)
	case "!=":
		return !strings.EqualFold(actualText, expected)
	case ">":
		return actualText > expected
	case "<":
		return actualText < expected
	case ">=":
		return actualText >= expected
	case "<=":
		return actualText <= expected
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
		if actual.text != "" && compareExplorerValue(actual, matches[2], expected, matches[1]) {
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
