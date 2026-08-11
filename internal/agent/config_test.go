package agent

import "testing"

func TestConfigDerivesHubURLFromEnrollmentURL(t *testing.T) {
	t.Setenv("CAROLINE_AGENT_STATE_DIR", t.TempDir())
	t.Setenv("CAROLINE_HUB_URL", "")
	t.Setenv("CAROLINE_ENROLL_URL", "https://caroline.example.com/api/v1/agent/enroll/token-123")
	t.Setenv("CAROLINE_ENROLLMENT_TOKEN", "")
	t.Setenv("CAROLINE_HUB_PUBLIC_KEY", "")

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if config.HubURL != "https://caroline.example.com" {
		t.Fatalf("HubURL = %q, want https://caroline.example.com", config.HubURL)
	}
	if config.EnrollURL != "https://caroline.example.com/api/v1/agent/enroll/token-123" {
		t.Fatalf("EnrollURL = %q", config.EnrollURL)
	}
}

func TestConfigLoadsPersistedHubURLAfterEnrollmentURLIsRemoved(t *testing.T) {
	stateDir := t.TempDir()
	config := Config{StateDir: stateDir, HubURL: "https://caroline.example.com"}
	key := make([]byte, 31)
	if err := saveHubPin(config.HubPinPath(), "hub-key", key, config.HubURL); err == nil {
		t.Fatal("saveHubPin accepted an invalid key")
	}
	// Use a valid persisted pin without requiring a network call.
	if err := saveHubPin(config.HubPinPath(), "hub-key", make([]byte, 32), config.HubURL); err != nil {
		t.Fatalf("saveHubPin: %v", err)
	}
	t.Setenv("CAROLINE_AGENT_STATE_DIR", stateDir)
	t.Setenv("CAROLINE_HUB_URL", "")
	t.Setenv("CAROLINE_ENROLL_URL", "")
	t.Setenv("CAROLINE_ENROLLMENT_TOKEN", "")
	t.Setenv("CAROLINE_HUB_PUBLIC_KEY", "")

	loaded, err := ConfigFromEnv()
	if err != nil || loaded.HubURL != config.HubURL {
		t.Fatalf("ConfigFromEnv HubURL = %q, err=%v", loaded.HubURL, err)
	}
}
