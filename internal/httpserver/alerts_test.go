package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"caroline/internal/alert"
)

func TestAlertCRUD(t *testing.T) {
	server := New(nil, nil, nil, alert.NewEngine(nil, nil))
	body := `{"name":"API errors","query":"severity >= ERROR","threshold":2,"windowSeconds":60,"cooldownSeconds":300}`
	createRecorder := httptest.NewRecorder()
	server.handleAlerts(createRecorder, httptest.NewRequest(http.MethodPost, "/api/alerts", strings.NewReader(body)))
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create returned status %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created alert.RuleView
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created rule: %v", err)
	}
	if created.Name != "API errors" || created.WebhookConfigured || created.Status != alert.StatusOK {
		t.Fatalf("unexpected created rule: %#v", created)
	}

	listRecorder := httptest.NewRecorder()
	server.handleAlerts(listRecorder, httptest.NewRequest(http.MethodGet, "/api/alerts", nil))
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), created.ID) {
		t.Fatalf("list returned status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	server.handleAlert(updateRecorder, httptest.NewRequest(
		http.MethodPatch,
		"/api/alerts/"+created.ID,
		strings.NewReader(`{"name":"API failures","query":"severity >= ERROR","threshold":1,"windowSeconds":120,"cooldownSeconds":0}`),
	))
	if updateRecorder.Code != http.StatusOK || !strings.Contains(updateRecorder.Body.String(), "API failures") {
		t.Fatalf("update returned status=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	server.handleAlert(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/api/alerts/"+created.ID, nil))
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete returned status %d", deleteRecorder.Code)
	}
	missingRecorder := httptest.NewRecorder()
	server.handleAlert(missingRecorder, httptest.NewRequest(http.MethodGet, "/api/alerts/"+created.ID, nil))
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("missing alert returned status %d", missingRecorder.Code)
	}
}

func TestAlertRejectsUnknownFields(t *testing.T) {
	server := New(nil, nil, nil, alert.NewEngine(nil, nil))
	recorder := httptest.NewRecorder()
	server.handleAlerts(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/alerts",
		strings.NewReader(`{"name":"invalid","threshold":1,"windowSeconds":60,"unexpected":true}`),
	))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown field returned status %d: %s", recorder.Code, recorder.Body.String())
	}
}
