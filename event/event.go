// Package event holds the canonical usage value structs and event envelopes
// shared by bridge (wire receive), metrics (Prometheus) and webhook
// (windowed aggregate / lifecycle ingest).
package event

// Values is the Role-typed oneof — exactly one of Source / Node / Sink
// is non-nil, picked by the component's Role. Embedded into
// UsageComponent / bridge.ComponentMetrics so the JSON fields sit at
// the component top level (no "values" wrapper).
type Values struct {
	Source *SourceValues `json:"source,omitempty"`
	Node   *NodeValues   `json:"node,omitempty"`
	Sink   *SinkValues   `json:"sink,omitempty"`
}

// SourceValues is the window-local payload for source-role components.
//
// VideoPixels is the canonical billing input for resolution-bucketed
// pricing: Σ(width_i × height_i) across all active video tracks at flush
// time. VideoFrames is also Σ across tracks; billing recovers per-track
// fps via ActiveVideoTracks (9×30fps → 30, not 270). VideoWidth /
// VideoHeight are monitoring-only maxima.
//
// VideoBytes / AudioBytes count encoded payload bytes before decode on
// bridge sources that forward compressed frames. Paths that already decode
// on the Go side (ffmpeg_pull, HEVC soft-transcode) emit raw frames and do
// not populate byte totals. AudioFrames / AudioMillis are Σ across audio tracks.
type SourceValues struct {
	VideoFrames       int64  `json:"videoFrames,omitempty"`
	VideoBytes        int64  `json:"videoBytes,omitempty"`
	VideoMillis       int64  `json:"videoMillis,omitempty"`
	VideoPixels       int64  `json:"videoPixels,omitempty"`
	VideoWidth        int32  `json:"videoWidth,omitempty"`
	VideoHeight       int32  `json:"videoHeight,omitempty"`
	VideoCodec        string `json:"videoCodec,omitempty"`
	ActiveVideoTracks int32  `json:"activeVideoTracks,omitempty"`
	VideoFps          int64  `json:"videoFps,omitempty"`
	VideoBitrateBps   int64  `json:"videoBitrateBps,omitempty"`

	AudioFrames       int64  `json:"audioFrames,omitempty"`
	AudioBytes        int64  `json:"audioBytes,omitempty"`
	AudioMillis       int64  `json:"audioMillis,omitempty"`
	AudioCodec        string `json:"audioCodec,omitempty"`
	ActiveAudioTracks int32  `json:"activeAudioTracks,omitempty"`
	AudioBitrateBps   int64  `json:"audioBitrateBps,omitempty"`
}

// NodeValues is the window-local payload for node-role components.
type NodeValues struct {
	VideoFrames int64  `json:"videoFrames,omitempty"`
	VideoMillis int64  `json:"videoMillis,omitempty"`
	VideoWidth  int32  `json:"videoWidth,omitempty"`
	VideoHeight int32  `json:"videoHeight,omitempty"`
	VideoPixels int64  `json:"videoPixels,omitempty"`
	VideoCodec  string `json:"videoCodec,omitempty"`
	AudioFrames int64  `json:"audioFrames,omitempty"`
	AudioMillis int64  `json:"audioMillis,omitempty"`
	AudioCodec  string `json:"audioCodec,omitempty"`

	InputCount int32 `json:"inputCount,omitempty"`

	ActiveConnectionSeconds int64 `json:"activeConnectionSeconds,omitempty"`

	ASRProcessedMillis int64                      `json:"asrProcessedMillis,omitempty"`
	ASRUtterances      int64                      `json:"asrUtterances,omitempty"`
	ASRModel           string                     `json:"asrModel,omitempty"`
	ASRProvider        string                     `json:"asrProvider,omitempty"`
	ASRSpeakers        map[string]ASRSpeakerStats `json:"asrSpeakers,omitempty"`

	TranslateInputMillis  int64  `json:"translateInputMillis,omitempty"`
	TranslateOutputMillis int64  `json:"translateOutputMillis,omitempty"`
	TranslateProvider     string `json:"translateProvider,omitempty"`
	TranslateModel        string `json:"translateModel,omitempty"`

	VoiceAgentInputMillis  int64  `json:"voiceAgentInputMillis,omitempty"`
	VoiceAgentOutputMillis int64  `json:"voiceAgentOutputMillis,omitempty"`
	VoiceAgentProvider     string `json:"voiceAgentProvider,omitempty"`
	VoiceAgentModel        string `json:"voiceAgentModel,omitempty"`

	// Video mixer monitoring: OffscreenCanvas composite wall time per 1s pipeline tick.
	MixerComposeMsMax int64 `json:"mixerComposeMsMax,omitempty"`
	MixerComposeMsSum int64 `json:"mixerComposeMsSum,omitempty"`
}

// ASRSpeakerStats accumulates per-speaker stats inside NodeValues.
type ASRSpeakerStats struct {
	SpeechDurationMs int64 `json:"speechDurationMs,omitempty"`
	Utterances       int64 `json:"utterances,omitempty"`
}

// SinkValues is the window-local payload for sink-role components.
//
// ImageBuckets maps "WxH" → upload count for image sinks. Populated when
// the Go ImageSink drains successful uploads into the usage aggregator.
type SinkValues struct {
	// EgressBytes is the number of bytes successfully written to the sink's
	// destination during this window. Failed writes and locally queued data are
	// excluded.
	EgressBytes int64 `json:"egressBytes,omitempty"`

	VideoFrames     int64  `json:"videoFrames,omitempty"`
	VideoBytes      int64  `json:"videoBytes,omitempty"`
	VideoMillis     int64  `json:"videoMillis,omitempty"`
	VideoPixels     int64  `json:"videoPixels,omitempty"`
	VideoWidth      int32  `json:"videoWidth,omitempty"`
	VideoHeight     int32  `json:"videoHeight,omitempty"`
	VideoCodec      string `json:"videoCodec,omitempty"`
	VideoFps        int64  `json:"videoFps,omitempty"`
	VideoBitrateBps int64  `json:"videoBitrateBps,omitempty"`

	AudioFrames     int64  `json:"audioFrames,omitempty"`
	AudioBytes      int64  `json:"audioBytes,omitempty"`
	AudioMillis     int64  `json:"audioMillis,omitempty"`
	AudioCodec      string `json:"audioCodec,omitempty"`
	AudioBitrateBps int64  `json:"audioBitrateBps,omitempty"`

	SinkOutputCount int32            `json:"sinkOutputCount,omitempty"`
	ImageBuckets    map[string]int64 `json:"imageBuckets,omitempty"`
}
