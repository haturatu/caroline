package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort    = "8080"
	maxLogTail     = 1000
	maxLogPayload  = 8 * 1024 * 1024
	requestTimeout = 35 * time.Second
)

type dockerClient struct {
	client  *http.Client
	baseURL string
}

type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created int64             `json:"Created"`
	Labels  map[string]string `json:"Labels"`
}

type containerInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	State        string            `json:"state"`
	Status       string            `json:"status"`
	Created      time.Time         `json:"created"`
	Labels       map[string]string `json:"labels,omitempty"`
	LogCount     int               `json:"logCount"`
	ErrorCount   int               `json:"errorCount"`
	WarningCount int               `json:"warningCount"`
}

type logLine struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Container   string    `json:"container"`
	ContainerID string    `json:"containerId"`
	Severity    string    `json:"severity"`
	Stream      string    `json:"stream"`
	Message     string    `json:"message"`
}

type cloudResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

type explorerEntry struct {
	InsertID    string            `json:"insertId"`
	Timestamp   time.Time         `json:"timestamp"`
	Severity    string            `json:"severity"`
	LogName     string            `json:"logName"`
	Resource    cloudResource     `json:"resource"`
	Labels      map[string]string `json:"labels,omitempty"`
	TextPayload string            `json:"textPayload,omitempty"`
	JSONPayload map[string]any    `json:"jsonPayload,omitempty"`
	Summary     string            `json:"summary"`
	Stream      string            `json:"stream"`
}

type timelineBucket struct {
	Start      time.Time      `json:"start"`
	End        time.Time      `json:"end"`
	Total      int            `json:"total"`
	Severities map[string]int `json:"severities"`
}

type fieldValue struct {
	Name   string         `json:"name"`
	Count  int            `json:"count"`
	Values map[string]int `json:"values,omitempty"`
}

type fieldGroup struct {
	Name   string       `json:"name"`
	Fields []fieldValue `json:"fields"`
}

type explorerResponse struct {
	Entries       []explorerEntry  `json:"entries"`
	Containers    []containerInfo  `json:"containers"`
	Timeline      []timelineBucket `json:"timeline"`
	Fields        []fieldGroup     `json:"fields"`
	Total         int              `json:"total"`
	NextPageToken string           `json:"nextPageToken,omitempty"`
	GeneratedAt   time.Time        `json:"generatedAt"`
	From          time.Time        `json:"from"`
	To            time.Time        `json:"to"`
	Duration      string           `json:"duration"`
	Query         string           `json:"query"`
	Approximate   bool             `json:"approximate"`
	Errors        []string         `json:"errors,omitempty"`
}

type statusResponse struct {
	Connected     bool      `json:"connected"`
	DockerVersion string    `json:"dockerVersion,omitempty"`
	APIVersion    string    `json:"apiVersion,omitempty"`
	CheckedAt     time.Time `json:"checkedAt"`
	Message       string    `json:"message,omitempty"`
}

type dockerVersion struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
}

type dockerFrame struct {
	stream string
	data   []byte
}

type server struct {
	docker *dockerClient
}

func newDockerClient() *dockerClient {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	transport := &http.Transport{
		Proxy:                 nil,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	baseURL := "http://docker"

	switch {
	case strings.HasPrefix(host, "unix://"):
		socketPath := strings.TrimPrefix(host, "unix://")
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "unix", socketPath)
		}
	case strings.HasPrefix(host, "tcp://"):
		baseURL = "http://" + strings.TrimPrefix(host, "tcp://")
	case strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://"):
		baseURL = strings.TrimRight(host, "/")
	default:
		log.Printf("unsupported DOCKER_HOST %q, using default Docker socket", host)
		socketPath := "/var/run/docker.sock"
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "unix", socketPath)
		}
	}

	return &dockerClient{
		client:  &http.Client{Transport: transport, Timeout: requestTimeout},
		baseURL: baseURL,
	}
}

func (d *dockerClient) do(ctx context.Context, method, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, d.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *dockerClient) check(ctx context.Context) (dockerVersion, error) {
	var version dockerVersion
	err := d.do(ctx, http.MethodGet, "/version", &version)
	return version, err
}

func (d *dockerClient) listRunning(ctx context.Context) ([]dockerContainer, error) {
	filters, _ := json.Marshal(map[string][]string{"status": {"running"}})
	endpoint := "/containers/json?all=0&filters=" + url.QueryEscape(string(filters))
	var containers []dockerContainer
	err := d.do(ctx, http.MethodGet, endpoint, &containers)
	return containers, err
}

func (d *dockerClient) logs(ctx context.Context, containerID string, tail int, since time.Time) ([]dockerFrame, error) {
	params := url.Values{}
	params.Set("stdout", "1")
	params.Set("stderr", "1")
	params.Set("timestamps", "1")
	params.Set("follow", "0")
	params.Set("tail", strconv.Itoa(tail))
	if !since.IsZero() {
		params.Set("since", strconv.FormatInt(since.Unix(), 10))
	}

	endpoint := "/containers/" + url.PathEscape(containerID) + "/logs?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return readDockerFrames(resp.Body)
}

func readDockerFrames(body io.Reader) ([]dockerFrame, error) {
	reader := bufio.NewReaderSize(body, 32*1024)
	peek, err := reader.Peek(8)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(peek) < 8 {
		payload, readErr := io.ReadAll(io.LimitReader(reader, maxLogPayload))
		return []dockerFrame{{stream: "stdout", data: payload}}, readErr
	}

	// Docker sends an 8-byte header for non-TTY containers: stream, three padding
	// bytes, then a big-endian payload size. TTY containers return raw bytes.
	multiplexed := (peek[0] == 1 || peek[0] == 2) && peek[1] == 0 && peek[2] == 0 && peek[3] == 0
	if !multiplexed {
		payload, readErr := io.ReadAll(io.LimitReader(reader, maxLogPayload))
		return []dockerFrame{{stream: "stdout", data: payload}}, readErr
	}

	frames := make([]dockerFrame, 0, 8)
	var total int
	for {
		header := make([]byte, 8)
		_, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return frames, err
		}
		length := int(binary.BigEndian.Uint32(header[4:]))
		if length < 0 || length > maxLogPayload || total+length > maxLogPayload {
			return frames, fmt.Errorf("docker log frame is too large (%d bytes)", length)
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return frames, err
		}
		stream := "stdout"
		if header[0] == 2 {
			stream = "stderr"
		}
		frames = append(frames, dockerFrame{stream: stream, data: payload})
		total += length
	}
	return frames, nil
}

func containerName(container dockerContainer) string {
	if len(container.Names) == 0 {
		return container.ID[:min(12, len(container.ID))]
	}
	return strings.TrimPrefix(container.Names[0], "/")
}

func toContainerInfo(container dockerContainer) containerInfo {
	created := time.Unix(container.Created, 0).UTC()
	return containerInfo{
		ID:      container.ID,
		Name:    containerName(container),
		Image:   container.Image,
		State:   container.State,
		Status:  container.Status,
		Created: created,
		Labels:  container.Labels,
	}
}

func parseLogFrame(frame dockerFrame, container dockerContainer) []logLine {
	contents := strings.ReplaceAll(string(frame.data), "\r\n", "\n")
	contents = strings.TrimSuffix(contents, "\n")
	if contents == "" {
		return nil
	}
	lines := strings.Split(contents, "\n")
	result := make([]logLine, 0, len(lines))
	for index, raw := range lines {
		if raw == "" {
			continue
		}
		timestamp := time.Now().UTC()
		message := raw
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) == 2 {
			if parsed, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				timestamp = parsed.UTC()
				message = parts[1]
			}
		}
		result = append(result, logLine{
			ID:          lineID(container.ID, frame.stream, timestamp, message, index),
			Timestamp:   timestamp,
			Container:   containerName(container),
			ContainerID: container.ID,
			Severity:    detectSeverity(message),
			Stream:      frame.stream,
			Message:     message,
		})
	}
	return result
}

func lineID(containerID, stream string, timestamp time.Time, message string, index int) string {
	value := fmt.Sprintf("%s|%s|%s|%d|%s", containerID, stream, timestamp.Format(time.RFC3339Nano), index, message)
	return fmt.Sprintf("%x", sha1.Sum([]byte(value)))[:16]
}

func detectSeverity(message string) string {
	upper := strings.ToUpper(message)
	for _, marker := range []string{"PANIC", "FATAL", "CRITICAL", "ERROR", "EXCEPTION"} {
		if strings.Contains(upper, marker) {
			return "ERROR"
		}
	}
	for _, marker := range []string{"WARN", "DEPRECATED"} {
		if strings.Contains(upper, marker) {
			return "WARNING"
		}
	}
	for _, marker := range []string{"DEBUG", "TRACE"} {
		if strings.Contains(upper, marker) {
			return "DEBUG"
		}
	}
	return "INFO"
}

var explorerClause = regexp.MustCompile("^\\s*([A-Za-z0-9_.-]+)\\s*(>=|<=|!=|=|:|>|<)\\s*(?:\"([^\"]*)\"|`([^`]*)`|([^\\s]+))\\s*$")

func toExplorerEntry(line logLine, container dockerContainer) explorerEntry {
	textPayload := line.Message
	var jsonPayload map[string]any
	var decoded map[string]any
	if json.Unmarshal([]byte(line.Message), &decoded) == nil {
		jsonPayload = decoded
		if value, ok := decoded["log"].(string); ok {
			textPayload = value
		} else if value, ok := decoded["message"].(string); ok {
			textPayload = value
		}
	}
	labels := map[string]string{
		"container_id":   container.ID,
		"container_name": containerName(container),
		"stream":         line.Stream,
	}
	return explorerEntry{
		InsertID:  line.ID,
		Timestamp: line.Timestamp,
		Severity:  line.Severity,
		LogName:   fmt.Sprintf("containers/%s/%s", containerName(container), line.Stream),
		Resource: cloudResource{
			Type: "docker_container",
			Labels: map[string]string{
				"container_name": containerName(container),
				"container_id":   container.ID,
				"image":          container.Image,
			},
		},
		Labels:      labels,
		TextPayload: textPayload,
		JSONPayload: jsonPayload,
		Summary:     textPayload,
		Stream:      line.Stream,
	}
}

func explorerSearchText(entry explorerEntry) string {
	parts := []string{entry.Summary, entry.TextPayload, entry.LogName, entry.Resource.Type}
	for key, value := range entry.Resource.Labels {
		parts = append(parts, key, value)
	}
	if entry.JSONPayload != nil {
		if encoded, err := json.Marshal(entry.JSONPayload); err == nil {
			parts = append(parts, string(encoded))
		}
	}
	return strings.Join(parts, " ")
}

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

func queryValue(entry explorerEntry, field string) []string {
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

func severityRank(severity string) int {
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
		left, right := severityRank(actual), severityRank(expected)
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

func matchesExplorerClause(entry explorerEntry, clause string) bool {
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

func matchesExplorerQuery(entry explorerEntry, query string) bool {
	query = strings.TrimSpace(strings.ReplaceAll(query, "\n", " "))
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

func parseExplorerDuration(value string) time.Duration {
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

func explorerWindow(r *http.Request) (time.Time, time.Time, string) {
	to := time.Now().UTC()
	if value := r.URL.Query().Get("to"); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			to = parsed.UTC()
		}
	}
	durationName := r.URL.Query().Get("duration")
	duration := parseExplorerDuration(durationName)
	from := to.Add(-duration)
	if value := r.URL.Query().Get("from"); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			from = parsed.UTC()
			duration = to.Sub(from)
		}
	}
	if durationName == "" {
		durationName = duration.String()
	}
	return from, to, durationName
}

func buildTimeline(entries []explorerEntry, from, to time.Time) []timelineBucket {
	const bucketCount = 24
	duration := to.Sub(from)
	if duration <= 0 {
		return nil
	}
	buckets := make([]timelineBucket, bucketCount)
	span := duration / bucketCount
	for index := range buckets {
		start := from.Add(time.Duration(index) * span)
		end := start.Add(span)
		if index == bucketCount-1 {
			end = to
		}
		buckets[index] = timelineBucket{Start: start, End: end, Severities: map[string]int{"DEBUG": 0, "INFO": 0, "WARNING": 0, "ERROR": 0}}
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
		if severityRank(severity) >= severityRank("ERROR") {
			severity = "ERROR"
		} else if severityRank(severity) >= severityRank("WARNING") {
			severity = "WARNING"
		} else if severityRank(severity) <= severityRank("DEBUG") {
			severity = "DEBUG"
		} else {
			severity = "INFO"
		}
		buckets[index].Severities[severity]++
	}
	return buckets
}

func buildFieldGroups(entries []explorerEntry) []fieldGroup {
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
	result := make([]fieldGroup, 0, 2)
	for _, groupName := range []string{"Pinned", "System Metadata", "Frequent Fields"} {
		fields := make([]fieldValue, 0)
		for name, field := range groups[groupName] {
			fields = append(fields, fieldValue{Name: name, Count: field.count, Values: field.values})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Count > fields[j].Count })
		if len(fields) > 10 {
			fields = fields[:10]
		}
		if len(fields) > 0 {
			result = append(result, fieldGroup{Name: groupName, Fields: fields})
		}
	}
	return result
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "caroline"})
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	version, err := s.docker.check(ctx)
	status := statusResponse{Connected: err == nil, DockerVersion: version.Version, APIVersion: version.APIVersion, CheckedAt: time.Now().UTC()}
	if err != nil {
		status.Message = "Docker daemon is unavailable"
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) handleExplorer(w http.ResponseWriter, r *http.Request) {
	from, to, durationName := explorerWindow(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	severity := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("severity")))
	stream := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("stream")))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	selected := requestedContainers(r.URL.Query().Get("containers"))
	limit := queryInt(r, "limit", 100)
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := queryInt(r, "pageToken", 0)
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	containers, err := s.docker.listRunning(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, dockerUnavailableMessage(err))
		return
	}

	selectedContainers := make([]dockerContainer, 0, len(containers))
	for _, container := range containers {
		if len(selected) == 0 || matchesContainerSelection(container, selected) {
			selectedContainers = append(selectedContainers, container)
		}
	}
	type fetched struct {
		container dockerContainer
		entries   []explorerEntry
		err       error
	}
	results := make(chan fetched, len(selectedContainers))
	var wait sync.WaitGroup
	tail := maxLogTail
	for _, container := range selectedContainers {
		container := container
		wait.Add(1)
		go func() {
			defer wait.Done()
			frames, fetchErr := s.docker.logs(ctx, container.ID, tail, from)
			if fetchErr != nil {
				results <- fetched{container: container, err: fetchErr}
				return
			}
			entries := make([]explorerEntry, 0, tail)
			for _, frame := range frames {
				for _, line := range parseLogFrame(frame, container) {
					entry := toExplorerEntry(line, container)
					if !entry.Timestamp.Before(from) && !entry.Timestamp.After(to) && (severity == "" || severity == "ALL" || severityRank(entry.Severity) >= severityRank(severity)) && (stream == "" || entry.Stream == stream) && matchesExplorerQuery(entry, query) {
						entries = append(entries, entry)
					}
				}
			}
			results <- fetched{container: container, entries: entries}
		}()
	}
	wait.Wait()
	close(results)

	response := explorerResponse{Entries: make([]explorerEntry, 0), Containers: make([]containerInfo, 0, len(containers)), GeneratedAt: time.Now().UTC(), From: from, To: to, Duration: durationName, Query: query, Approximate: true}
	containerInfos := make(map[string]containerInfo, len(containers))
	for _, container := range containers {
		containerInfos[container.ID] = toContainerInfo(container)
	}
	for result := range results {
		info := toContainerInfo(result.container)
		if result.err != nil {
			response.Errors = append(response.Errors, info.Name+": "+result.err.Error())
			containerInfos[result.container.ID] = info
			continue
		}
		info.LogCount = len(result.entries)
		for _, entry := range result.entries {
			if entry.Severity == "ERROR" {
				info.ErrorCount++
			}
			if entry.Severity == "WARNING" {
				info.WarningCount++
			}
		}
		containerInfos[result.container.ID] = info
		response.Entries = append(response.Entries, result.entries...)
	}
	for _, container := range containers {
		response.Containers = append(response.Containers, containerInfos[container.ID])
	}
	if sortOrder == "asc" {
		sort.Slice(response.Entries, func(i, j int) bool { return response.Entries[i].Timestamp.Before(response.Entries[j].Timestamp) })
	} else {
		sort.Slice(response.Entries, func(i, j int) bool { return response.Entries[i].Timestamp.After(response.Entries[j].Timestamp) })
	}
	sort.Slice(response.Containers, func(i, j int) bool { return response.Containers[i].Name < response.Containers[j].Name })
	response.Total = len(response.Entries)
	response.Timeline = buildTimeline(response.Entries, from, to)
	response.Fields = buildFieldGroups(response.Entries)
	if offset < len(response.Entries) {
		end := offset + limit
		if end > len(response.Entries) {
			end = len(response.Entries)
		}
		page := response.Entries[offset:end]
		response.Entries = page
		if end < response.Total {
			response.NextPageToken = strconv.Itoa(end)
		}
	} else {
		response.Entries = []explorerEntry{}
	}
	writeJSON(w, http.StatusOK, response)
}

func requestedContainers(value string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	return result
}

func matchesContainerSelection(container dockerContainer, selected map[string]bool) bool {
	if selected[container.ID] || selected[container.ID[:min(12, len(container.ID))]] {
		return true
	}
	return selected[containerName(container)]
}

func queryInt(r *http.Request, key string, fallback int) int {
	parsed, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return parsed
}

func dockerUnavailableMessage(err error) string {
	return "Docker daemon is unavailable. Mount /var/run/docker.sock or set DOCKER_HOST. Details: " + err.Error()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	app := &server{docker: newDockerClient()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", app.handleHealth)
	mux.HandleFunc("/api/status", app.handleStatus)
	mux.HandleFunc("/api/explorer", app.handleExplorer)
	static := http.FileServer(http.Dir("static"))
	mux.Handle("/", static)

	address := ":" + port
	log.Printf("Caroline listening on http://localhost:%s", port)
	log.Printf("Docker host: %s", dockerHostDescription())
	if err := http.ListenAndServe(address, loggingMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

func dockerHostDescription() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}
	return "unix:///var/run/docker.sock"
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(started).Round(time.Millisecond))
		}
	})
}
