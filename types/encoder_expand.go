package avflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ExpandSinkEncoders turns the legacy encoding shortcut on mux sinks into
// explicit encoder nodes. Nodes with identical upstream/filter/config tuples
// are shared across sinks.
func (j *Job) ExpandSinkEncoders() error {
	outputs := make(map[string][]MediaType, len(j.Sources)+len(j.Nodes))
	encodedOutputs := make(map[string]bool, len(j.Nodes))
	usedNames := make(map[string]struct{}, len(j.Sources)+len(j.Nodes)+len(j.Sinks))
	for i := range j.Sources {
		outputs[j.Sources[i].Name] = j.Sources[i].OutputMediaTypes()
		usedNames[j.Sources[i].Name] = struct{}{}
	}
	for i := range j.Nodes {
		outputs[j.Nodes[i].Name] = j.Nodes[i].OutputMediaTypes()
		encodedOutputs[j.Nodes[i].Name] = j.Nodes[i].outputsEncodedMedia()
		usedNames[j.Nodes[i].Name] = struct{}{}
	}
	for i := range j.Sinks {
		usedNames[j.Sinks[i].Name] = struct{}{}
	}

	shared := make(map[string]string)
	var generated []Node
	for i := range j.Sinks {
		sink := &j.Sinks[i]
		if !sinkNeedsEncodedInput(sink.Type) {
			continue
		}
		enc := effectiveSinkEncoding(sink)
		var expanded InputRefs
		seen := make(map[string]struct{})
		appendInput := func(ref InputRef) {
			key, _ := json.Marshal(ref)
			if _, ok := seen[string(key)]; ok {
				return
			}
			seen[string(key)] = struct{}{}
			expanded = append(expanded, ref)
		}

		sinkMedia := sink.InputMediaTypes()
		for _, input := range sink.Inputs {
			mediaTypes, ok := outputs[input.Name]
			if !ok {
				appendInput(input)
				continue
			}
			if containsMediaType(mediaTypes, MediaTypeData) &&
				consumesMedia(input.Select, sinkMedia, MediaTypeData) {
				appendInput(inputForMedia(input, MediaTypeData))
			}
			if encodedOutputs[input.Name] {
				if containsMediaType(mediaTypes, MediaTypeVideo) &&
					consumesMedia(input.Select, sinkMedia, MediaTypeVideo) {
					appendInput(inputForMedia(input, MediaTypeVideo))
				}
				if containsMediaType(mediaTypes, MediaTypeAudio) &&
					consumesMedia(input.Select, sinkMedia, MediaTypeAudio) {
					appendInput(inputForMedia(input, MediaTypeAudio))
				}
				continue
			}
			if containsMediaType(mediaTypes, MediaTypeVideo) &&
				enc.VideoCodec != "" &&
				consumesMedia(input.Select, sinkMedia, MediaTypeVideo) {
				cfg := VideoEncoderConfig{
					Codec:               enc.VideoCodec,
					BitrateBps:          enc.VideoBitrateBps,
					KeyframeIntervalSec: enc.KeyframeIntervalSec,
					EnableBFrames:       enc.EnableBFrames,
				}
				cfg.SetDefaults()
				ref, node, key, err := generatedEncoder(input, NodeTypeVideoEncoder, cfg)
				if err != nil {
					return err
				}
				name, exists := shared[key]
				if !exists {
					name = uniqueGeneratedEncoderName("video", key, usedNames)
					node.Name = name
					generated = append(generated, node)
					outputs[name] = node.OutputMediaTypes()
					encodedOutputs[name] = true
					shared[key] = name
				}
				ref.Name = name
				appendInput(ref)
			}
			if containsMediaType(mediaTypes, MediaTypeAudio) &&
				enc.AudioCodec != "" &&
				consumesMedia(input.Select, sinkMedia, MediaTypeAudio) {
				cfg := AudioEncoderConfig{
					Codec:      enc.AudioCodec,
					Profile:    enc.AudioProfile,
					BitrateBps: enc.AudioBitrateBps,
					SampleRate: DefaultEncodingAudioSampleRate,
					Channels:   2,
				}
				cfg.SetDefaults()
				ref, node, key, err := generatedEncoder(input, NodeTypeAudioEncoder, cfg)
				if err != nil {
					return err
				}
				name, exists := shared[key]
				if !exists {
					name = uniqueGeneratedEncoderName("audio", key, usedNames)
					node.Name = name
					generated = append(generated, node)
					outputs[name] = node.OutputMediaTypes()
					encodedOutputs[name] = true
					shared[key] = name
				}
				ref.Name = name
				appendInput(ref)
			}
		}
		sink.Inputs = expanded
	}
	j.Nodes = append(j.Nodes, generated...)
	return nil
}

func sinkNeedsEncodedInput(t SinkType) bool {
	switch t {
	case SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeSegment:
		return true
	default:
		return false
	}
}

func effectiveSinkEncoding(sink *Sink) EncodingConfig {
	var enc EncodingConfig
	if configured := sinkEncoding(sink); configured != nil {
		enc = *configured
	}
	return *ensureMuxEncoding(&enc)
}

func generatedEncoder(
	input InputRef,
	typ NodeType,
	config any,
) (InputRef, Node, string, error) {
	mediaType := MediaTypeVideo
	if typ == NodeTypeAudioEncoder {
		mediaType = MediaTypeAudio
	}
	edgeSelect := cloneSelectForMedia(input.Select, mediaType)
	rawConfig, err := json.Marshal(config)
	if err != nil {
		return InputRef{}, Node{}, "", fmt.Errorf("marshal generated %s config: %w", typ, err)
	}
	keyBytes, err := json.Marshal(struct {
		Type   NodeType
		Input  InputRef
		Config json.RawMessage
	}{typ, InputRef{Name: input.Name, Select: edgeSelect}, rawConfig})
	if err != nil {
		return InputRef{}, Node{}, "", fmt.Errorf("hash generated %s: %w", typ, err)
	}
	node := Node{
		Type:   typ,
		Inputs: InputRefs{{Name: input.Name, Select: edgeSelect}},
		Config: rawConfig,
	}
	return InputRef{Select: &SelectFilter{MediaTypes: []MediaType{mediaType}}},
		node, string(keyBytes), nil
}

func uniqueGeneratedEncoderName(kind, key string, used map[string]struct{}) string {
	sum := sha256.Sum256([]byte(key))
	base := "__" + kind + "_encoder_" + hex.EncodeToString(sum[:6])
	name := base
	for suffix := 2; ; suffix++ {
		if _, exists := used[name]; !exists {
			used[name] = struct{}{}
			return name
		}
		name = fmt.Sprintf("%s_%d", base, suffix)
	}
}

func cloneSelectForMedia(src *SelectFilter, mediaType MediaType) *SelectFilter {
	out := &SelectFilter{MediaTypes: []MediaType{mediaType}}
	if src == nil {
		return out
	}
	out.Identities = append([]string(nil), src.Identities...)
	out.ExcludeIdentities = append([]string(nil), src.ExcludeIdentities...)
	out.TrackSources = append([]string(nil), src.TrackSources...)
	out.TrackNames = append([]string(nil), src.TrackNames...)
	out.TrackSids = append([]string(nil), src.TrackSids...)
	return out
}

// selectAllowsMedia reports whether an edge filter admits mediaType. A nil
// filter, or one that leaves mediaTypes empty, admits everything.
func selectAllowsMedia(edge *SelectFilter, mediaType MediaType) bool {
	if edge == nil || len(edge.MediaTypes) == 0 {
		return true
	}
	return containsMediaType(edge.MediaTypes, mediaType)
}

// consumesMedia reports whether mediaType survives both the edge filter and the
// set of media types the consuming component can take at all.
func consumesMedia(edge *SelectFilter, allowed []MediaType, mediaType MediaType) bool {
	return containsMediaType(allowed, mediaType) && selectAllowsMedia(edge, mediaType)
}

func inputForMedia(input InputRef, mediaType MediaType) InputRef {
	input.Select = cloneSelectForMedia(input.Select, mediaType)
	return input
}
