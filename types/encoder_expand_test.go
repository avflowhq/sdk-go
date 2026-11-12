package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExpandSinkEncodersSharesEquivalentNodes(t *testing.T) {
	encoding := json.RawMessage(`{
		"urls":["rtmp://example/live/key"],
		"encoding":{"videoCodec":"h264","audioCodec":"aac","videoBitrateBps":2000000}
	}`)
	job := Job{
		Name:    "shared-encoders",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Sinks: []Sink{
			{Name: "one", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("input"), Config: encoding},
			{Name: "two", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("input"), Config: encoding},
		},
	}

	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Nodes) != 2 {
		t.Fatalf("generated nodes = %d, want one shared video and audio encoder", len(job.Nodes))
	}
	if len(job.Sinks[0].Inputs) != 2 || len(job.Sinks[1].Inputs) != 2 {
		t.Fatalf("expanded sink inputs = %v / %v", job.Sinks[0].Inputs, job.Sinks[1].Inputs)
	}
	for i := range job.Sinks[0].Inputs {
		if job.Sinks[0].Inputs[i].Name != job.Sinks[1].Inputs[i].Name {
			t.Fatalf("encoder input %d is not shared: %q != %q", i,
				job.Sinks[0].Inputs[i].Name, job.Sinks[1].Inputs[i].Name)
		}
	}
}

func TestRTCAndWHIPRejectEncodedInputs(t *testing.T) {
	for _, sinkType := range []SinkType{SinkTypeLivekit, SinkTypeWHIP} {
		t.Run(string(sinkType), func(t *testing.T) {
			job := Job{
				Name:    "rtc-encoded",
				Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
				Nodes: []Node{{
					Name:   "encoded",
					Type:   NodeTypeVideoEncoder,
					Inputs: InputRefsFromNames("input"),
					Config: json.RawMessage(`{"codec":"h264"}`),
				}},
				Sinks: []Sink{{
					Name:   "out",
					Type:   sinkType,
					Inputs: InputRefsFromNames("encoded"),
					Config: json.RawMessage(`{}`),
				}},
			}
			job.Init()
			if err := job.Normalize(); err != nil {
				t.Fatal(err)
			}
			err := job.Validate()
			validation, ok := err.(*ValidationError)
			if !ok || validation.Code != "INVALID_MEDIA_EDGE" {
				t.Fatalf("Validate() error = %v, want INVALID_MEDIA_EDGE", err)
			}
		})
	}
}

func TestMuxSinkAcceptsExplicitEncoderInputs(t *testing.T) {
	job := Job{
		Name:    "explicit-encoders",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{
			{Name: "video", Type: NodeTypeVideoEncoder, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"codec":"h264"}`)},
			{Name: "audio", Type: NodeTypeAudioEncoder, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"codec":"aac"}`)},
		},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("video", "audio"),
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Nodes) != 2 {
		t.Fatalf("explicit encoder count changed: %d", len(job.Nodes))
	}
}

func TestCaptionOnlyRTMPRequiresExplicitVideo(t *testing.T) {
	job := Job{
		Name:    "caption-only",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{{
			Name:   "captions",
			Type:   NodeTypeASR,
			Inputs: InputRefsFromNames("input"),
			Config: json.RawMessage(`{}`),
		}},
		Sinks: []Sink{{
			Name:   "out",
			Type:   SinkTypeRTMPPush,
			Inputs: InputRefsFromNames("captions"),
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	err := job.EnsureReady()
	if err == nil || !strings.Contains(err.Error(), "requires a video input to carry captions") {
		t.Fatalf("EnsureReady() error = %v, want caption video requirement", err)
	}
}

func TestCaptionOnlySegmentDoesNotRequireVideo(t *testing.T) {
	job := Job{
		Name:    "caption-only-hls",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{{
			Name:   "captions",
			Type:   NodeTypeASR,
			Inputs: InputRefsFromNames("input"),
			Config: json.RawMessage(`{}`),
		}},
		Sinks: []Sink{{
			Name:   "out",
			Type:   SinkTypeSegment,
			Inputs: InputRefsFromNames("captions"),
			Config: json.RawMessage(`{}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Nodes) != 1 || len(job.Sinks[0].Inputs) != 1 {
		t.Fatalf("caption-only HLS graph changed: nodes=%v inputs=%v", job.Nodes, job.Sinks[0].Inputs)
	}
}

func TestExplicitVideoGeneratorCarriesRTMPCaptions(t *testing.T) {
	job := Job{
		Name: "explicit-caption-carrier",
		Sources: []Source{
			{Name: "input", Type: SourceTypeRTMPPull},
			{Name: "carrier", Type: SourceTypeVideoGenerator, Config: json.RawMessage(`{}`)},
		},
		Nodes: []Node{
			{Name: "captions", Type: NodeTypeASR, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{}`)},
			{Name: "video", Type: NodeTypeVideoEncoder, Inputs: InputRefsFromNames("carrier"),
				Config: json.RawMessage(`{"codec":"h264"}`)},
		},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("captions", "video"),
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Sinks[0].Inputs) != 2 {
		t.Fatalf("sink inputs = %v, want caption and explicit video", job.Sinks[0].Inputs)
	}
	var cfg VideoGeneratorSourceConfig
	if err := json.Unmarshal(job.Sources[1].Config, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 16 || cfg.Height != 16 || cfg.Fps != 15 {
		t.Fatalf("video generator defaults = %+v", cfg)
	}
}

func TestExpandSinkEncodersPreservesVoiceAgentDataEdge(t *testing.T) {
	job := Job{
		Name:    "voice-agent-metadata",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{
			{Name: "voice", Type: NodeTypeVoiceAgent, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"provider":"deepgram"}`)},
			{Name: "video", Type: NodeTypeVideoEncoder, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"codec":"h264"}`)},
		},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeSegment, Inputs: InputRefsFromNames("voice", "video"),
			Config: json.RawMessage(`{"encoding":{"audioCodec":"aac"}}`),
		}},
	}

	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	foundVoice := false
	foundAudioEncoder := false
	for _, input := range job.Sinks[0].Inputs {
		foundVoice = foundVoice || input.Name == "voice"
		if node := job.NodeByName(input.Name); node != nil && node.Type == NodeTypeAudioEncoder {
			foundAudioEncoder = true
		}
	}
	if !foundVoice || !foundAudioEncoder {
		t.Fatalf("sink inputs = %v, want voice data edge plus generated audio encoder",
			job.Sinks[0].Inputs)
	}
}

func TestExpandSinkEncodersRequiresExplicitUpstreamVoiceAgentDataEdge(t *testing.T) {
	for _, tc := range []struct {
		name          string
		explicitData  bool
		wantVoiceData bool
	}{
		{name: "not declared", wantVoiceData: false},
		{name: "explicit data input", explicitData: true, wantVoiceData: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sinkInputs := InputRefsFromNames("video", "agent_audio")
			if tc.explicitData {
				sinkInputs = append(sinkInputs, InputRef{
					Name:   "voice",
					Select: &SelectFilter{MediaTypes: []MediaType{MediaTypeData}},
				})
			}
			job := Job{
				Name:    "voice-agent-upstream-metadata",
				Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
				Nodes: []Node{
					{Name: "voice", Type: NodeTypeVoiceAgent, Inputs: InputRefsFromNames("input"),
						Config: json.RawMessage(`{"provider":"deepgram"}`)},
					{Name: "video", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("input"),
						Config: json.RawMessage(`{}`)},
					{Name: "agent_audio", Type: NodeTypeAudioMixer, Inputs: InputRefsFromNames("input", "voice"),
						Config: json.RawMessage(`{}`)},
				},
				Sinks: []Sink{{
					Name:   "out",
					Type:   SinkTypeSegment,
					Inputs: sinkInputs,
					Config: json.RawMessage(`{"encoding":{"videoCodec":"h264","audioCodec":"aac"}}`),
				}},
			}

			if err := job.EnsureReady(); err != nil {
				t.Fatal(err)
			}

			foundVoiceData := false
			foundVideoEncoder := false
			foundAudioEncoder := false
			for _, input := range job.Sinks[0].Inputs {
				if input.Name == "voice" {
					foundVoiceData = input.Select != nil &&
						len(input.Select.MediaTypes) == 1 &&
						input.Select.MediaTypes[0] == MediaTypeData
				}
				if node := job.NodeByName(input.Name); node != nil {
					foundVideoEncoder = foundVideoEncoder || node.Type == NodeTypeVideoEncoder
					foundAudioEncoder = foundAudioEncoder || node.Type == NodeTypeAudioEncoder
				}
			}
			if foundVoiceData != tc.wantVoiceData || !foundVideoEncoder || !foundAudioEncoder {
				t.Fatalf("sink inputs = %v, want voice data=%t plus generated encoders",
					job.Sinks[0].Inputs, tc.wantVoiceData)
			}
		})
	}
}

func TestVoiceAgentMetadataRequiresVideoCarrier(t *testing.T) {
	job := Job{
		Name:    "voice-agent-metadata-no-video",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{{
			Name: "voice", Type: NodeTypeVoiceAgent, Inputs: InputRefsFromNames("input"),
			Config: json.RawMessage(`{"provider":"deepgram"}`),
		}},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeSegment, Inputs: InputRefsFromNames("voice"),
			Config: json.RawMessage(`{"encoding":{"audioCodec":"aac"}}`),
		}},
	}

	err := job.EnsureReady()
	if err == nil || !strings.Contains(err.Error(), "requires a video input") {
		t.Fatalf("EnsureReady() error = %v, want voice-agent video carrier requirement", err)
	}
}

func TestExpandSinkEncodersDefaultsOmittedCodecs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		encoding string
	}{
		{name: "video codec only", encoding: `{"videoCodec":"h264"}`},
		{name: "audio codec only", encoding: `{"audioCodec":"aac"}`},
		{name: "encoding omitted", encoding: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := Job{
				Name:    "default-codecs",
				Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
				Sinks: []Sink{{
					Name: "out", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("input"),
					Config: json.RawMessage(`{"urls":["rtmp://example/live/key"],"encoding":` +
						tc.encoding + `}`),
				}},
			}
			if err := job.EnsureReady(); err != nil {
				t.Fatal(err)
			}
			got := map[NodeType]bool{}
			for _, n := range job.Nodes {
				got[n.Type] = true
			}
			if !got[NodeTypeVideoEncoder] || !got[NodeTypeAudioEncoder] {
				t.Fatalf("generated nodes = %+v, want video+audio encoders", job.Nodes)
			}
		})
	}
}

func TestExpandSinkEncodersSelectsAudioOnly(t *testing.T) {
	job := Job{
		Name:    "audio-only-select",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeRTMPPush,
			Inputs: InputRefs{{
				Name:   "input",
				Select: &SelectFilter{MediaTypes: []MediaType{MediaTypeAudio}},
			}},
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Nodes) != 1 || job.Nodes[0].Type != NodeTypeAudioEncoder {
		t.Fatalf("generated nodes = %+v, want one audio encoder", job.Nodes)
	}
}

func TestExpandSinkEncodersDefaultsWhenEncodingOmitted(t *testing.T) {
	job := Job{
		Name:    "mux-default-encoding",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("input"),
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatal(err)
	}
	if len(job.Nodes) != 2 {
		t.Fatalf("generated nodes = %+v, want video+audio encoders", job.Nodes)
	}
	got := map[NodeType]string{}
	for _, n := range job.Nodes {
		got[n.Type] = n.Name
	}
	if _, ok := got[NodeTypeVideoEncoder]; !ok {
		t.Fatal("missing video encoder")
	}
	if _, ok := got[NodeTypeAudioEncoder]; !ok {
		t.Fatal("missing audio encoder")
	}
}

func TestValidateRejectsMoreThanOneVideoEncoder(t *testing.T) {
	job := Job{
		Name:    "two-video-encoders",
		Sources: []Source{{Name: "input", Type: SourceTypeRTMPPull}},
		Nodes: []Node{
			{Name: "video1", Type: NodeTypeVideoEncoder, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"codec":"h264"}`)},
			{Name: "video2", Type: NodeTypeVideoEncoder, Inputs: InputRefsFromNames("input"),
				Config: json.RawMessage(`{"codec":"h264"}`)},
		},
		Sinks: []Sink{{
			Name: "out", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("video1", "video2"),
			Config: json.RawMessage(`{"urls":["rtmp://example/live/key"]}`),
		}},
	}
	err := job.EnsureReady()
	if err == nil || !strings.Contains(err.Error(), "at most one video_encoder") {
		t.Fatalf("EnsureReady() error = %v, want single video encoder limit", err)
	}
}
