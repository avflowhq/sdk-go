package avflow

import "encoding/json"

// Voice agent provider types.

type VoiceAgentProvider string

const (
	// VoiceAgentProviderOpenAI uses the OpenAI Realtime API: a single
	// full-duplex WebSocket that ingests participant audio and emits the bot's
	// synthesized speech (speech-to-speech, with server-side VAD and barge-in).
	VoiceAgentProviderOpenAI VoiceAgentProvider = "openai"
	// VoiceAgentProviderGemini uses the Google Gemini Live API
	// (BidiGenerateContent): a full-duplex WebSocket ingesting pcm16 16kHz mono
	// and emitting pcm16 24kHz, with built-in automatic activity (VAD) detection
	// and interruption signals.
	VoiceAgentProviderGemini VoiceAgentProvider = "gemini"
	// VoiceAgentProviderCloudflare dials a self-deployed @cloudflare/voice agent
	// over WebSocket: participant audio is sent as binary mono 16kHz s16le PCM
	// frames; the agent (STT + LLM + TTS runs in the Worker) returns binary audio
	// plus JSON transcript/status messages. The return format is whatever the
	// agent's TTS emits — mp3 (the default WorkersAITTS, decoded in the browser
	// via WebCodecs) or raw PCM — and is detected automatically (audio_config,
	// else by sniffing the first frame).
	VoiceAgentProviderCloudflare VoiceAgentProvider = "cloudflare"
	// VoiceAgentProviderDeepgram uses the Deepgram Voice Agent API: a single
	// WebSocket that orchestrates STT, LLM inference, and TTS with native
	// turn-taking and barge-in.
	VoiceAgentProviderDeepgram VoiceAgentProvider = "deepgram"
)

// Hard caps for voice agent configs.
const (
	MaxVoiceAgentInstructions = 8_000
	MaxVoiceAgentGreeting     = 2_000
)

// OpenAIRealtimeConfig is the OpenAI Realtime speech-to-speech configuration.
// The Realtime API speaks pcm16 at 24kHz in both directions.
type OpenAIRealtimeConfig struct {
	// APIKey is the OpenAI API key, sent as "Authorization: Bearer <key>".
	APIKey string `json:"apiKey"`
	// Model is the realtime model (default "gpt-realtime-2.1").
	Model string `json:"model,omitempty"`
	// WSURL overrides the base WebSocket URL (default "wss://api.openai.com").
	WSURL string `json:"wsUrl,omitempty"`
}

// GeminiLiveConfig is the Google Gemini Live speech-to-speech configuration.
// The Live API ingests pcm16 16kHz mono and emits pcm16 24kHz mono.
type GeminiLiveConfig struct {
	// APIKey is the Google AI Studio API key, passed as the "key" query param.
	APIKey string `json:"apiKey"`
	// Model is the live model, e.g. "gemini-3.1-flash-live-preview"
	// (default "gemini-3.1-flash-live-preview"). A "models/" prefix is added
	// automatically when absent.
	Model string `json:"model,omitempty"`
	// WSURL overrides the base WebSocket URL
	// (default "wss://generativelanguage.googleapis.com").
	WSURL string `json:"wsUrl,omitempty"`
}

// CloudflareConfig configures the @cloudflare/voice provider, which connects to
// a separately deployed agent. The actual return audio format is decided by that
// agent's TTS, not by this config: the default WorkersAITTS (zero external keys)
// emits mp3, and only a PCM-capable TTS (e.g. ElevenLabs "pcm_16000") yields raw
// PCM. The engine handles both — mp3 is decoded in the browser via WebCodecs.
type CloudflareConfig struct {
	// WSURL is the full WebSocket URL of the deployed voice agent, e.g.
	// "wss://<worker>/agents/<agent>/<name>".
	WSURL string `json:"wsUrl"`
	// Token, when set, is sent as "Authorization: Bearer <token>".
	Token string `json:"token,omitempty"`
	// AudioFormat is only an advisory hint sent as the start_call preferred_format
	// ("mp3", default, or "pcm16"); the agent may ignore it. The effective format
	// is resolved from the server's audio_config, else by sniffing the first
	// frame, so a mismatch here does not cause misdecoding.
	AudioFormat string `json:"audioFormat,omitempty"`
	// OutputSampleRate is the assumed PCM sample rate when the return format is
	// pcm and the server does not announce one via audio_config (default 16000).
	OutputSampleRate int `json:"outputSampleRate,omitempty"`
}

// DeepgramVoiceAgentConfig configures Deepgram's end-to-end Voice Agent API.
// The node-level language, instructions, greeting, and voice are applied to
// the agent settings sent when each session opens.
type DeepgramVoiceAgentConfig struct {
	// APIKey is the Deepgram API key, sent as "Authorization: Token <key>".
	APIKey string `json:"apiKey"`
	// ListenModel is the Deepgram STT model (default "flux-general-en").
	ListenModel string `json:"listenModel,omitempty"`
	// ThinkProvider is the LLM provider type understood by Deepgram
	// (default "open_ai").
	ThinkProvider string `json:"thinkProvider,omitempty"`
	// ThinkModel is the LLM model (default "gpt-4o-mini").
	ThinkModel string `json:"thinkModel,omitempty"`
	// WSURL overrides the full Voice Agent WebSocket endpoint
	// (default "wss://agent.deepgram.com/v1/agent/converse").
	WSURL string `json:"wsUrl,omitempty"`
}

// Voice agent node

// VoiceAgentNodeConfig is the per-node voice agent configuration. The behavioral
// fields are provider-independent; Provider/ProviderConfig select and configure
// the speech-to-speech vendor. Platform-credential jobs may omit both provider
// fields because the control plane injects them into the runtime spec.
type VoiceAgentNodeConfig struct {
	// Language is an optional BCP-47 conversation language hint. Empty lets the
	// provider use its language detection or default behavior.
	Language string `json:"language,omitempty"`
	// Instructions is the system prompt steering the agent's behavior.
	Instructions string `json:"instructions,omitempty"`
	// Greeting, when set, is spoken by the agent as soon as it joins.
	Greeting string `json:"greeting,omitempty"`
	// Voice is the provider output voice id (e.g. OpenAI "alloy", Gemini "Puck").
	// Empty lets the provider use its default.
	Voice string `json:"voice,omitempty"`
	// Interrupt enables barge-in: when a participant starts speaking the agent
	// stops talking and the bot playout buffer is flushed. Defaults to true when
	// omitted; set false to let the agent always finish its turn. nil = default.
	Interrupt *bool `json:"interrupt,omitempty"`
	// Provider selects the streaming voice agent vendor.
	Provider VoiceAgentProvider `json:"provider,omitempty"`
	// ProviderConfig contains provider-specific credentials and options.
	ProviderConfig json.RawMessage `json:"providerConfig,omitempty"`
}

func (c *VoiceAgentNodeConfig) SetDefaults() {
	if len(c.Instructions) > MaxVoiceAgentInstructions {
		c.Instructions = c.Instructions[:MaxVoiceAgentInstructions]
	}
	if len(c.Greeting) > MaxVoiceAgentGreeting {
		c.Greeting = c.Greeting[:MaxVoiceAgentGreeting]
	}
	if c.Interrupt == nil {
		t := true
		c.Interrupt = &t
	}
}
