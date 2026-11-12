package avflow

import (
	"reflect"
	"testing"
)

// The lists below duplicate the type constants on purpose: a new component
// type that is added to the wire enums but not to the registry fails here
// (and in `make schema-check`, which refuses to generate a schema with an
// uncovered type) instead of silently degrading to a loose config.

func TestSourceConfigTypeCoversAllSourceTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []SourceType{
		SourceTypeAxonkit, SourceTypeLivekit, SourceTypeJitsi, SourceTypeDaily,
		SourceTypeAgora, SourceTypeWHEP, SourceTypeWebCapture,
		SourceTypeVideoGenerator, SourceTypeRTMPPull, SourceTypeHTTPFLVPull,
		SourceTypeHLSPull, SourceTypeSRTPull, SourceTypeRTSPPull, SourceTypeFilePull,
	} {
		if SourceConfigType(typ) == nil {
			t.Errorf("SourceConfigType(%q) = nil, want a registered config struct", typ)
		}
	}
	if SourceConfigType(SourceType("nope")) != nil {
		t.Error(`SourceConfigType("nope") = non-nil, want nil for unknown types`)
	}
}

func TestSinkConfigTypeCoversAllSinkTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []SinkType{
		SinkTypeAxonkit, SinkTypeLivekit, SinkTypeJitsi, SinkTypeDaily, SinkTypeAgora,
		SinkTypeWHIP, SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeSegment,
		SinkTypeImage, SinkTypeWebSocket,
	} {
		if SinkConfigType(typ) == nil {
			t.Errorf("SinkConfigType(%q) = nil, want a registered config struct", typ)
		}
	}
	if SinkConfigType(SinkType("nope")) != nil {
		t.Error(`SinkConfigType("nope") = non-nil, want nil for unknown types`)
	}
}

func TestNodeConfigTypeCoversAllNodeTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []NodeType{
		NodeTypeVideoMixer, NodeTypeAudioMixer, NodeTypeAudioResample,
		NodeTypeVideoEncoder, NodeTypeAudioEncoder, NodeTypeASR,
		NodeTypeTranslate, NodeTypeVoiceAgent,
	} {
		if NodeConfigType(typ) == nil {
			t.Errorf("NodeConfigType(%q) = nil, want a registered config struct", typ)
		}
	}
	if NodeConfigType(NodeType("nope")) != nil {
		t.Error(`NodeConfigType("nope") = non-nil, want nil for unknown types`)
	}
}

func TestStorageConfigTypeCoversAllStorageTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"s3", "gcp", "azure", "local"} {
		if StorageConfigType(typ) == nil {
			t.Errorf("StorageConfigType(%q) = nil, want a registered config struct", typ)
		}
	}
	if StorageConfigType("nope") != nil {
		t.Error(`StorageConfigType("nope") = non-nil, want nil for unknown types`)
	}
}

// Every registered type must be a plain struct so gen-schema can reflect it
// into a JSON object schema.
func TestRegisteredConfigTypesAreStructs(t *testing.T) {
	t.Parallel()
	all := map[string][]reflect.Type{}
	for _, typ := range []SourceType{
		SourceTypeAxonkit, SourceTypeLivekit, SourceTypeJitsi, SourceTypeDaily,
		SourceTypeAgora, SourceTypeWHEP, SourceTypeWebCapture,
		SourceTypeVideoGenerator, SourceTypeRTMPPull, SourceTypeHTTPFLVPull,
		SourceTypeHLSPull, SourceTypeSRTPull, SourceTypeRTSPPull, SourceTypeFilePull,
	} {
		all["source"] = append(all["source"], SourceConfigType(typ))
	}
	for _, typ := range []SinkType{
		SinkTypeAxonkit, SinkTypeLivekit, SinkTypeJitsi, SinkTypeDaily, SinkTypeAgora,
		SinkTypeWHIP, SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeSegment,
		SinkTypeImage, SinkTypeWebSocket,
	} {
		all["sink"] = append(all["sink"], SinkConfigType(typ))
	}
	for _, typ := range []NodeType{
		NodeTypeVideoMixer, NodeTypeAudioMixer, NodeTypeAudioResample,
		NodeTypeVideoEncoder, NodeTypeAudioEncoder, NodeTypeASR,
		NodeTypeTranslate, NodeTypeVoiceAgent,
	} {
		all["node"] = append(all["node"], NodeConfigType(typ))
	}
	for kind, types := range all {
		for _, rt := range types {
			if rt == nil || rt.Kind() != reflect.Struct {
				t.Errorf("%s config registry returned non-struct %v", kind, rt)
			}
		}
	}
}
