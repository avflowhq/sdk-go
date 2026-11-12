package avflow

import "encoding/json"

// ASR provider types.

type ASRProvider string

const (
	// ASRProviderVolcengine is internal-only; do not expose it in public docs.
	ASRProviderVolcengine ASRProvider = "volcengine"
	// ASRProviderGoogle uses Google Cloud Speech-to-Text streaming (Go side).
	ASRProviderGoogle ASRProvider = "google"
	// ASRProviderElevenLabs uses ElevenLabs Scribe v2 realtime streaming STT.
	ASRProviderElevenLabs ASRProvider = "elevenlabs"
	// ASRProviderDeepgram uses Deepgram live streaming STT.
	ASRProviderDeepgram ASRProvider = "deepgram"
	// ASRProviderOpenAI uses the OpenAI Realtime transcription API.
	ASRProviderOpenAI ASRProvider = "openai"
	// ASRProviderAzure is internal-only; do not expose it in public docs.
	ASRProviderAzure ASRProvider = "azure"
)

// VolcengineASRConfig is the Volcengine SAUC streaming ASR service configuration.
type VolcengineASRConfig struct {
	AppID     string `json:"appId,omitempty"`
	Token     string `json:"token,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	AccessKey string `json:"accessKey"`
	AppKey    string `json:"appKey"`
	WSURL     string `json:"wsUrl,omitempty"`
}

// GoogleASRConfig is the Google Cloud Speech-to-Text streaming configuration.
type GoogleASRConfig struct {
	// CredentialsJSON is the service-account key as a JSON object (required).
	// Prefer object form in the job spec, e.g. {"type":"service_account",...}.
	// A JSON string containing the key document is also accepted for compatibility.
	CredentialsJSON json.RawMessage `json:"credentialsJson"`
	// AlternativeLanguages are extra BCP-47 codes for automatic language detection.
	AlternativeLanguages []string `json:"alternativeLanguages,omitempty"`
	// Model is the recognition model. V1 examples: "latest_long", "telephony".
	// Chirp models ("chirp_3", "chirp_2", …) automatically use Speech-to-Text V2.
	Model string `json:"model,omitempty"`
	// Location is the Speech-to-Text V2 region for Chirp models (e.g. "us", "eu",
	// "global"). Ignored for V1. Empty defaults to "us".
	Location string `json:"location,omitempty"`
	// DisableAutoPunctuation turns off automatic punctuation (enabled by default).
	DisableAutoPunctuation bool `json:"disableAutoPunctuation,omitempty"`
}

// ElevenLabsASRConfig is the ElevenLabs Scribe v2 realtime STT configuration.
type ElevenLabsASRConfig struct {
	// APIKey is the ElevenLabs API key, sent as the xi-api-key header.
	APIKey string `json:"apiKey"`
	// ModelID is the realtime model id (default "scribe_v2_realtime").
	ModelID string `json:"modelId,omitempty"`
	// WSURL overrides the base WebSocket URL (default "wss://api.elevenlabs.io").
	WSURL string `json:"wsUrl,omitempty"`
}

// DeepgramASRConfig is the Deepgram live streaming STT configuration.
type DeepgramASRConfig struct {
	// APIKey is the Deepgram API key, sent as "Authorization: Token <key>".
	APIKey string `json:"apiKey"`
	// Model is the recognition model (default "nova-3").
	Model string `json:"model,omitempty"`
	// WSURL overrides the base WebSocket URL (default "wss://api.deepgram.com").
	WSURL string `json:"wsUrl,omitempty"`
	// EndpointingMs is silence (ms) before finalizing a segment (default 300).
	EndpointingMs int `json:"endpointingMs,omitempty"`
}

// OpenAIASRConfig is the OpenAI Realtime transcription configuration. The
// realtime API requires 24kHz pcm16 input, so 16kHz pipeline audio is resampled.
type OpenAIASRConfig struct {
	// APIKey is the OpenAI API key, sent as "Authorization: Bearer <key>".
	APIKey string `json:"apiKey"`
	// Model is the transcription model (default "gpt-live-transcribe").
	Model string `json:"model,omitempty"`
	// WSURL overrides the base WebSocket URL (default "wss://api.openai.com").
	WSURL string `json:"wsUrl,omitempty"`
}

// AzureASRConfig is the Azure Cognitive Services Speech-to-Text configuration.
type AzureASRConfig struct {
	// APIKey is the subscription key, sent as Ocp-Apim-Subscription-Key.
	APIKey string `json:"apiKey"`
	// Region is the service region, e.g. "eastus" (used to build the host).
	Region string `json:"region"`
	// Endpoint overrides the full wss endpoint (region is then ignored).
	Endpoint string `json:"endpoint,omitempty"`
}

// ASR node

// ASRNodeConfig is the per-node ASR configuration. Language is the only
// user-facing setting. Provider and ProviderConfig are internal runtime fields
// injected by the control plane and must not be exposed in public job specs.
type ASRNodeConfig struct {
	// Language is an optional recognition hint. Empty leaves language selection
	// to the provider and is never replaced with an avflow default.
	Language string `json:"language,omitempty"`
	// Provider is internal-only and selects the managed streaming ASR vendor.
	Provider ASRProvider `json:"provider,omitempty"`
	// ProviderConfig is internal-only and contains managed credentials and
	// vendor-specific runtime options.
	ProviderConfig json.RawMessage `json:"providerConfig,omitempty"`
}

func (c *ASRNodeConfig) SetDefaults() {}
