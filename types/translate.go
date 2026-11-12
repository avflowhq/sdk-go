package avflow

import "encoding/json"

// Translate provider types. Provider selection and credentials are injected by
// the control plane; translate nodes do not support customer-supplied keys.
type TranslateProvider string

const (
	TranslateProviderOpenAI TranslateProvider = "openai"
	TranslateProviderGemini TranslateProvider = "gemini"
)

// OpenAITranslateConfig configures the dedicated Realtime Translation API.
type OpenAITranslateConfig struct {
	APIKey string `json:"apiKey"`
	Model  string `json:"model,omitempty"`
	WSURL  string `json:"wsUrl,omitempty"`
}

// GeminiTranslateConfig configures Gemini Live Translation.
type GeminiTranslateConfig struct {
	APIKey string `json:"apiKey"`
	Model  string `json:"model,omitempty"`
	WSURL  string `json:"wsUrl,omitempty"`
}

// TranslateNodeConfig configures continuous speech-to-speech translation.
// TargetLanguage and EchoTargetLanguage are public. Provider and ProviderConfig
// are internal RuntimeSpec fields injected by the control plane.
type TranslateNodeConfig struct {
	// TargetLanguage is the required BCP-47 output language.
	TargetLanguage string `json:"targetLanguage"`
	// EchoTargetLanguage repeats input that is already in the target language.
	// Providers otherwise suppress same-language input where supported.
	EchoTargetLanguage bool              `json:"echoTargetLanguage,omitempty"`
	Provider           TranslateProvider `json:"provider,omitempty"`
	ProviderConfig     json.RawMessage   `json:"providerConfig,omitempty"`
}

func (c *TranslateNodeConfig) SetDefaults() {}
