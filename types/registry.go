package avflow

import "reflect"

// Config registries: the declarative mapping from each component wire type to
// the struct its `config` field parses into. cmd/gen-schema uses these to emit
// per-type config schemas (discriminated unions keyed on `type`) into
// schemas/job.schema.json, so client SDKs and the edge validator share the
// exact config shapes instead of a loose `"config": true`.
//
// A nil return means "no typed config registered for this type". gen-schema
// fails the build for any enum value without an entry, so adding a component
// type without registering its config cannot pass `make schema-check`.

// SourceConfigType returns the config struct type that source type t parses
// its config into.
func SourceConfigType(t SourceType) reflect.Type {
	switch t {
	case SourceTypeAxonkit:
		return reflect.TypeOf(AxonkitSourceConfig{})
	case SourceTypeLivekit:
		return reflect.TypeOf(LivekitSourceConfig{})
	case SourceTypeJitsi:
		return reflect.TypeOf(JitsiSourceConfig{})
	case SourceTypeDaily:
		return reflect.TypeOf(DailySourceConfig{})
	case SourceTypeAgora:
		return reflect.TypeOf(AgoraSourceConfig{})
	case SourceTypeWHEP, SourceTypeRTMPPull, SourceTypeHTTPFLVPull, SourceTypeHLSPull,
		SourceTypeSRTPull, SourceTypeRTSPPull, SourceTypeFilePull:
		return reflect.TypeOf(URLPullSourceConfig{})
	case SourceTypeWebCapture:
		return reflect.TypeOf(WebCaptureSourceConfig{})
	case SourceTypeVideoGenerator:
		return reflect.TypeOf(VideoGeneratorSourceConfig{})
	}
	return nil
}

// SinkConfigType returns the config struct type that sink type t parses its
// config into.
func SinkConfigType(t SinkType) reflect.Type {
	switch t {
	case SinkTypeAxonkit:
		return reflect.TypeOf(AxonkitSinkConfig{})
	case SinkTypeLivekit:
		return reflect.TypeOf(LivekitSinkConfig{})
	case SinkTypeJitsi:
		return reflect.TypeOf(JitsiSinkConfig{})
	case SinkTypeDaily:
		return reflect.TypeOf(DailySinkConfig{})
	case SinkTypeAgora:
		return reflect.TypeOf(AgoraSinkConfig{})
	case SinkTypeRTMPPush:
		return reflect.TypeOf(RTMPPushConfig{})
	case SinkTypeSRTPush:
		return reflect.TypeOf(SRTPushConfig{})
	case SinkTypeWHIP:
		return reflect.TypeOf(WHIPPushConfig{})
	case SinkTypeSegment:
		return reflect.TypeOf(SegmentSinkConfig{})
	case SinkTypeImage:
		return reflect.TypeOf(ImageSinkConfig{})
	case SinkTypeWebSocket:
		return reflect.TypeOf(WebSocketSinkConfig{})
	}
	return nil
}

// NodeConfigType returns the config struct type that node type t parses its
// config into.
func NodeConfigType(t NodeType) reflect.Type {
	switch t {
	case NodeTypeVideoMixer:
		return reflect.TypeOf(VideoMixerConfig{})
	case NodeTypeAudioMixer:
		return reflect.TypeOf(AudioMixerConfig{})
	case NodeTypeAudioResample:
		return reflect.TypeOf(AudioResampleConfig{})
	case NodeTypeVideoEncoder:
		return reflect.TypeOf(VideoEncoderConfig{})
	case NodeTypeAudioEncoder:
		return reflect.TypeOf(AudioEncoderConfig{})
	case NodeTypeASR:
		return reflect.TypeOf(ASRNodeConfig{})
	case NodeTypeTranslate:
		return reflect.TypeOf(TranslateNodeConfig{})
	case NodeTypeVoiceAgent:
		return reflect.TypeOf(VoiceAgentNodeConfig{})
	}
	return nil
}

// StorageConfigType returns the storageConfig struct type for a segment/image
// sink storageType. The "local" storage is only usable on self-hosted runs
// (the edge rejects it; see jobValidator / validate.go) but stays part of the
// public contract for the single-binary / Docker deployments.
func StorageConfigType(storageType string) reflect.Type {
	switch storageType {
	case "s3":
		return reflect.TypeOf(S3StorageConfig{})
	case "gcp":
		return reflect.TypeOf(GCPStorageConfig{})
	case "azure":
		return reflect.TypeOf(AzureStorageConfig{})
	case "local":
		return reflect.TypeOf(LocalStorageConfig{})
	}
	return nil
}
