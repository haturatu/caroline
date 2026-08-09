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
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported by this server")
		return
	}

	request := explorer.TailRequest{
		Since:    parseTailSince(r.URL.Query().Get("since")),
		Query:    strings.TrimSpace(r.URL.Query().Get("q")),
		Severity: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("severity"))),
		Stream:   strings.ToLower(strings.TrimSpace(r.URL.Query().Get("stream"))),
		Selected: explorer.RequestedContainers(r.URL.Query().Get("containers")),
	}

	setSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	tailContext, cancel := context.WithCancel(r.Context())
	defer cancel()

	events := make(chan explorer.StreamEvent)
	serviceDone := make(chan error, 1)
	go func() {
		serviceDone <- s.explorer.Tail(tailContext, request, func(event explorer.StreamEvent) error {
			select {
			case events <- event:
				return nil
			case <-tailContext.Done():
				return tailContext.Err()
			}
		})
		close(events)
	}()

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

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, open := <-events:
			if !open {
				if err := <-serviceDone; err != nil && tailContext.Err() == nil {
					_ = send("error", map[string]string{"message": dockerUnavailableMessage(err)})
				}
				return
			}
			if err := send(event.Name, event.Data); err != nil {
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
