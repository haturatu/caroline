package explorer

import (
	"context"
	"time"

	"caroline/internal/docker"
)

const (
	DefaultTimelineBuckets      = 24
	MinTimelineBuckets          = 24
	MaxTimelineBuckets          = 96
	MaxLogTail                  = 1000
	MaxEntries                  = 50000
	MaxConcurrentDockerRequests = 8
	// Live tail shares one Docker follow stream per container with the alert
	// engine, so it does not impose a second container-count cap. A zero value
	// means unlimited in logstream.Manager.Subscribe.
	MaxTailStreams = 0
)

type ContainerInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	NodeID       string            `json:"nodeId,omitempty"`
	NodeName     string            `json:"nodeName,omitempty"`
	Image        string            `json:"image"`
	State        string            `json:"state"`
	Status       string            `json:"status"`
	Created      time.Time         `json:"created"`
	Labels       map[string]string `json:"labels,omitempty"`
	LogCount     int               `json:"logCount"`
	ErrorCount   int               `json:"errorCount"`
	WarningCount int               `json:"warningCount"`
}

type Resource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

type Entry struct {
	InsertID    string            `json:"insertId"`
	Timestamp   time.Time         `json:"timestamp"`
	Severity    string            `json:"severity"`
	LogName     string            `json:"logName"`
	Resource    Resource          `json:"resource"`
	Labels      map[string]string `json:"labels,omitempty"`
	TextPayload string            `json:"textPayload,omitempty"`
	JSONPayload map[string]any    `json:"jsonPayload,omitempty"`
	Summary     string            `json:"summary"`
	Stream      string            `json:"stream"`
}

type TimelineBucket struct {
	Start      time.Time      `json:"start"`
	End        time.Time      `json:"end"`
	Total      int            `json:"total"`
	Severities map[string]int `json:"severities"`
}

type FieldValue struct {
	Name   string         `json:"name"`
	Count  int            `json:"count"`
	Values map[string]int `json:"values,omitempty"`
}

type FieldGroup struct {
	Name   string       `json:"name"`
	Fields []FieldValue `json:"fields"`
}

type Response struct {
	Entries       []Entry          `json:"entries"`
	Containers    []ContainerInfo  `json:"containers"`
	Timeline      []TimelineBucket `json:"timeline"`
	Fields        []FieldGroup     `json:"fields"`
	Total         int              `json:"total"`
	NextPageToken string           `json:"nextPageToken,omitempty"`
	GeneratedAt   time.Time        `json:"generatedAt"`
	From          time.Time        `json:"from"`
	To            time.Time        `json:"to"`
	Duration      string           `json:"duration"`
	Query         string           `json:"query"`
	Approximate   bool             `json:"approximate"`
	LogTail       int              `json:"logTail"`
	EntryLimit    int              `json:"entryLimit"`
	Truncated     bool             `json:"truncated"`
	Errors        []string         `json:"errors,omitempty"`
}

type Cursor struct {
	Timestamp time.Time `json:"timestamp"`
	InsertID  string    `json:"insertId"`
}

type SearchRequest struct {
	From            time.Time
	To              time.Time
	Duration        string
	Query           string
	Severity        string
	Stream          string
	Sort            string
	Selected        map[string]bool
	SelectedNodes   map[string]bool
	Limit           int
	Cursor          *Cursor
	TimelineBuckets int
}

// EntryBatch is the unit of durable ingestion. The batch identity is used for
// at-least-once deduplication at the Hub, while Containers keeps resource
// metadata available even when a container has not emitted a log yet.
type EntryBatch struct {
	AgentID    string
	BootID     string
	Sequence   uint64
	Entries    []Entry
	Containers []ContainerInfo
}

// LogStore is the Hub-side boundary between query processing and persistence.
// Implementations must make WriteBatch idempotent for the batch identity.
type LogStore interface {
	WriteBatch(context.Context, EntryBatch) (bool, error)
	SearchEntries(context.Context, SearchRequest) ([]Entry, error)
	ListContainers(context.Context) ([]ContainerInfo, error)
}

type source interface {
	ListRunning(context.Context) ([]docker.Container, error)
	Logs(context.Context, string, int, time.Time) ([]docker.Frame, error)
	FollowLogs(context.Context, string, time.Time, func(docker.Frame) error) error
}

type Service struct {
	docker source
	store  LogStore
}

func NewService(client source) *Service {
	return &Service{docker: client}
}

func NewStoreService(store LogStore) *Service {
	return &Service{store: store}
}
