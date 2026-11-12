package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"short", "***"},
		{"12345678", "***"},
		{"abcdefghij", "abc****hij"},
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig", "eyJ****sig"},
	}
	for _, tt := range tests {
		if got := maskSecret(tt.in); got != tt.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJobLogPayload_redactsNestedSecrets(t *testing.T) {
	j := &Job{
		Name: "demo",
		Sources: []Source{{
			Name: "room",
			Type: SourceTypeLivekit,
			Config: json.RawMessage(`{
				"serverUrl":"wss://example.livekit.cloud",
				"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaaa.bbbb",
				"apiKey":"APIabcdefghij"
			}`),
		}},
	}
	payload := jobLogPayload(j)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaaa.bbbb") {
		t.Fatalf("token leaked: %s", s)
	}
	if strings.Contains(s, "APIabcdefghij") {
		t.Fatalf("apiKey leaked: %s", s)
	}
	if !strings.Contains(s, "eyJ****bbb") {
		t.Fatalf("expected masked token, got %s", s)
	}
	if !strings.Contains(s, "API****hij") {
		t.Fatalf("expected masked apiKey, got %s", s)
	}
	if !strings.Contains(s, "wss://example.livekit.cloud") {
		t.Fatalf("serverUrl should remain: %s", s)
	}
}
