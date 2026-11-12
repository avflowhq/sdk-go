package avflow

import (
	"encoding/json"
	"testing"
)

func TestASRNodeConfigJSON(t *testing.T) {
	t.Parallel()

	cfg := ASRNodeConfig{
		Language:       "zh-CN",
		Provider:       ASRProviderDeepgram,
		ProviderConfig: json.RawMessage(`{"model":"nova-3"}`),
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["language"] != "zh-CN" {
		t.Fatalf("language = %v, want zh-CN", got["language"])
	}
	if _, ok := got["config"]; ok {
		t.Fatalf("legacy config field must not be emitted: %s", data)
	}
	providerConfig, ok := got["providerConfig"].(map[string]any)
	if !ok || providerConfig["model"] != "nova-3" {
		t.Fatalf("providerConfig = %#v", got["providerConfig"])
	}
}

func TestASRNodeConfigDoesNotDefaultLanguage(t *testing.T) {
	t.Parallel()

	cfg := ASRNodeConfig{
		Provider:       ASRProviderDeepgram,
		ProviderConfig: json.RawMessage(`{"model":"nova-3"}`),
	}
	cfg.SetDefaults()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["language"]; ok {
		t.Fatalf("optional language must be omitted: %s", data)
	}
}
