package avflow

import (
	"encoding/json"
	"testing"
)

func TestVoiceAgentNodeConfigJSON(t *testing.T) {
	t.Parallel()

	cfg := VoiceAgentNodeConfig{
		Language:       "en-US",
		Provider:       VoiceAgentProviderOpenAI,
		ProviderConfig: json.RawMessage(`{"model":"gpt-realtime"}`),
		Voice:          "verse",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["voice"] != "verse" {
		t.Fatalf("voice = %v, want verse", got["voice"])
	}
	if _, ok := got["config"]; ok {
		t.Fatalf("legacy config field must not be emitted: %s", data)
	}
	if got["language"] != "en-US" {
		t.Fatalf("language = %v, want en-US", got["language"])
	}
	providerConfig, ok := got["providerConfig"].(map[string]any)
	if !ok || providerConfig["model"] != "gpt-realtime" {
		t.Fatalf("providerConfig = %#v", got["providerConfig"])
	}
}

func TestVoiceAgentNodeConfigInterruptDefault(t *testing.T) {
	t.Parallel()

	cfg := VoiceAgentNodeConfig{}
	cfg.SetDefaults()
	if cfg.Interrupt == nil || !*cfg.Interrupt {
		t.Fatalf("Interrupt = %v, want true", cfg.Interrupt)
	}

	disabled := false
	cfg = VoiceAgentNodeConfig{Interrupt: &disabled}
	cfg.SetDefaults()
	if cfg.Interrupt == nil || *cfg.Interrupt {
		t.Fatalf("Interrupt = %v, want false", cfg.Interrupt)
	}
}
