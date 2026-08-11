package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"caroline/internal/alert"
)

const maxAlertRequestBytes = 64 << 10

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeError(w, http.StatusServiceUnavailable, "alert engine is unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.alerts.List())
	case http.MethodPost:
		s.createAlert(w, r)
	default:
		allowMethods(s.handleAlerts, http.MethodGet, http.MethodPost)(w, r)
	}
}

func (s *Server) handleAlert(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeError(w, http.StatusServiceUnavailable, "alert engine is unavailable")
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/alerts/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "alert rule was not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		for _, view := range s.alerts.List() {
			if view.ID == id {
				writeJSON(w, http.StatusOK, view)
				return
			}
		}
		writeError(w, http.StatusNotFound, "alert rule was not found")
	case http.MethodPut:
		s.updateAlert(w, r, id)
	case http.MethodPatch:
		s.patchAlert(w, r, id)
	case http.MethodDelete:
		if err := s.alerts.Delete(id); err != nil {
			if errors.Is(err, alert.ErrRuleNotFound) {
				writeError(w, http.StatusNotFound, "alert rule was not found")
				return
			}
			if errors.Is(err, alert.ErrPersistence) {
				writeError(w, http.StatusInternalServerError, "could not persist alert rule")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not delete alert rule")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		allowMethods(s.handleAlert, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)(w, r)
	}
}

func (s *Server) createAlert(w http.ResponseWriter, r *http.Request) {
	var spec alert.RuleSpec
	if err := decodeAlertSpec(w, r, &spec); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := s.alerts.Create(spec)
	if err != nil {
		if errors.Is(err, alert.ErrPersistence) {
			writeError(w, http.StatusInternalServerError, "could not persist alert rule")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) updateAlert(w http.ResponseWriter, r *http.Request, id string) {
	var spec alert.RuleSpec
	if err := decodeAlertSpec(w, r, &spec); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := s.alerts.Update(id, spec)
	if err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			writeError(w, http.StatusNotFound, "alert rule was not found")
			return
		}
		if errors.Is(err, alert.ErrPersistence) {
			writeError(w, http.StatusInternalServerError, "could not persist alert rule")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) patchAlert(w http.ResponseWriter, r *http.Request, id string) {
	var patch alert.RulePatchSpec
	if err := decodeAlertPatch(w, r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := s.alerts.Patch(id, patch)
	if err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			writeError(w, http.StatusNotFound, "alert rule was not found")
			return
		}
		if errors.Is(err, alert.ErrPersistence) {
			writeError(w, http.StatusInternalServerError, "could not persist alert rule")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func decodeAlertSpec(w http.ResponseWriter, r *http.Request, spec *alert.RuleSpec) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAlertRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(spec); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func decodeAlertPatch(w http.ResponseWriter, r *http.Request, patch *alert.RulePatchSpec) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAlertRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(patch); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}
	return nil
}
