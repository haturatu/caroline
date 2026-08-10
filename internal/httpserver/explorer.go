package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"caroline/internal/explorer"
)

const explorerRequestTimeout = 35 * time.Second

func parseExplorerRequest(r *http.Request) (explorer.SearchRequest, error) {
	query := r.URL.Query()
	from, to, duration := explorer.ParseWindow(query.Get("from"), query.Get("to"), query.Get("duration"), time.Now().UTC())
	request := explorer.SearchRequest{
		From:     from,
		To:       to,
		Duration: duration,
		Query:    strings.TrimSpace(query.Get("q")),
		Severity: strings.ToUpper(strings.TrimSpace(query.Get("severity"))),
		Stream:   strings.ToLower(strings.TrimSpace(query.Get("stream"))),
		Sort:     strings.ToLower(strings.TrimSpace(query.Get("sort"))),
		Selected: explorer.RequestedContainers(query.Get("containers")),
		Limit:    queryInt(query.Get("limit"), 100),
		TimelineBuckets: queryInt(
			query.Get("timelineBuckets"),
			explorer.DefaultTimelineBuckets,
		),
	}
	request.TimelineBuckets = explorer.NormalizeTimelineBuckets(request.TimelineBuckets)
	if value := strings.TrimSpace(query.Get("pageToken")); value != "" {
		cursor, err := explorer.DecodeCursor(value)
		if err != nil {
			return explorer.SearchRequest{}, err
		}
		request.Cursor = &cursor
	}
	if request.Limit < 1 {
		request.Limit = 100
	}
	if request.Limit > 1000 {
		request.Limit = 1000
	}
	return request, nil
}

func (s *Server) handleExplorer(w http.ResponseWriter, r *http.Request) {
	request, err := parseExplorerRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), explorerRequestTimeout)
	defer cancel()
	response, err := s.explorer.Search(ctx, request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, dockerUnavailableMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func queryInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
