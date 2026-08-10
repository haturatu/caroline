package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"caroline/internal/explorer"
)

func (s *Server) handleTail(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		setSSEHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if s.streams == nil {
		writeError(w, http.StatusServiceUnavailable, "live log streaming is unavailable")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported by this server")
		return
	}

	since := parseTailSince(r.URL.Query().Get("since"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	severity := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("severity")))
	stream := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("stream")))
	selected := explorer.RequestedContainers(r.URL.Query().Get("containers"))

	tailContext, cancel := context.WithCancel(r.Context())
	defer cancel()
	subscription, err := s.streams.Subscribe(
		tailContext,
		selected,
		since,
		explorer.MaxTailStreams,
	)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, dockerUnavailableMessage(err))
		return
	}
	defer subscription.Close()

	setSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	var writeMu sync.Mutex
	send := func(event string, value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := writeSSE(w, event, value); err != nil {
			cancel()
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := send("ready", map[string]any{
		"since":              since,
		"generatedAt":        time.Now().UTC(),
		"selectedContainers": subscription.SelectedContainers,
		"streamedContainers": subscription.StreamedContainers,
	}); err != nil {
		return
	}
	if subscription.StreamedContainers < subscription.SelectedContainers {
		if err := send("warning", map[string]any{
			"message":   fmt.Sprintf("Live tail is limited to %d containers.", explorer.MaxTailStreams),
			"streamed":  subscription.StreamedContainers,
			"requested": subscription.SelectedContainers,
		}); err != nil {
			return
		}
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case entry, open := <-subscription.Entries:
			if !open {
				return
			}
			if entry.Timestamp.Before(since) || !explorer.MatchesFilters(entry, query, severity, stream) {
				continue
			}
			if err := send("log", entry); err != nil {
				return
			}
		case streamError, open := <-subscription.Errors:
			if !open {
				return
			}
			if err := send("error", map[string]string{
				"container": explorer.ContainerName(streamError.Container),
				"message":   streamError.Err.Error(),
			}); err != nil {
				return
			}
		case <-tailContext.Done():
			return
		case <-heartbeat.C:
			if err := sendSSEComment(w, &writeMu, flusher, "keep-alive"); err != nil {
				return
			}
		}
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("X-Accel-Buffering", "no")
}

func parseTailSince(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}

func writeSSE(w io.Writer, event string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	return err
}

func sendSSEComment(w io.Writer, writeMu *sync.Mutex, flusher http.Flusher, comment string) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
