package httpserver

import (
	"net/http/httptest"
	"testing"
)

func TestBuildEnrollmentURLUsesConfiguredPublicURL(t *testing.T) {
	t.Setenv("CAROLINE_PUBLIC_URL", "https://caroline.example.com/")
	request := httptest.NewRequest("POST", "http://internal:8080/api/nodes", nil)
	got := buildEnrollmentURL(request, "car_enroll_token")
	want := "https://caroline.example.com/api/v1/agent/enroll/car_enroll_token"
	if got != want {
		t.Fatalf("buildEnrollmentURL = %q, want %q", got, want)
	}
}

func TestBuildEnrollmentURLUsesForwardedProtocolAndHost(t *testing.T) {
	t.Setenv("CAROLINE_PUBLIC_URL", "")
	request := httptest.NewRequest("POST", "http://internal:8080/api/nodes", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Host = "caroline.example.com"
	got := buildEnrollmentURL(request, "token")
	want := "https://caroline.example.com/api/v1/agent/enroll/token"
	if got != want {
		t.Fatalf("buildEnrollmentURL = %q, want %q", got, want)
	}
}
