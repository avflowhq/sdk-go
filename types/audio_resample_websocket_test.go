package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAudioResampleConfigSetDefaults(t *testing.T) {
	rateTests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default", want: DefaultResampleRate},
		{name: "below minimum", in: 1000, want: 8000},
		{name: "exact", in: 24000, want: 24000},
		{name: "nearest", in: 22050, want: 24000},
		{name: "above maximum", in: 192000, want: 48000},
	}
	for _, test := range rateTests {
		t.Run(test.name, func(t *testing.T) {
			cfg := AudioResampleConfig{SampleRate: test.in}
			cfg.SetDefaults()
			if cfg.SampleRate != test.want {
				t.Fatalf("sampleRate = %d, want %d", cfg.SampleRate, test.want)
			}
		})
	}

	channelTests := []struct {
		name string
		in   int
		want int
	}{
		{name: "preserve", want: 0},
		{name: "minimum", in: -1, want: 1},
		{name: "mono", in: 1, want: 1},
		{name: "stereo", in: 2, want: 2},
		{name: "maximum", in: 8, want: MaxAudioChannels},
	}
	for _, test := range channelTests {
		t.Run("channels/"+test.name, func(t *testing.T) {
			cfg := AudioResampleConfig{Channels: test.in}
			cfg.SetDefaults()
			if cfg.Channels != test.want {
				t.Fatalf("channels = %d, want %d", cfg.Channels, test.want)
			}
		})
	}
}

func TestValidateAudioResampleToWebSocket(t *testing.T) {
	job := &Job{
		Name: "audio-to-websocket",
		Sources: []Source{{
			Name: "room",
			Type: SourceTypeLivekit,
		}},
		Nodes: []Node{{
			Name:   "pcm16k",
			Type:   NodeTypeAudioResample,
			Inputs: InputRefsFromNames("room"),
			Config: json.RawMessage(`{"sampleRate":16000,"channels":1}`),
		}},
		Sinks: []Sink{{
			Name:   "audio_ws",
			Type:   SinkTypeWebSocket,
			Inputs: InputRefsFromNames("pcm16k"),
			Config: json.RawMessage(`{"url":"wss://example.com/audio"}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
}

func TestValidateAudioEncoderToWebSocket(t *testing.T) {
	job := &Job{
		Name:    "encoded-audio-to-websocket",
		Sources: []Source{{Name: "room", Type: SourceTypeRTMPPull}},
		Nodes: []Node{{
			Name:   "opus",
			Type:   NodeTypeAudioEncoder,
			Inputs: InputRefsFromNames("room"),
			Config: json.RawMessage(`{"codec":"opus","sampleRate":48000,"channels":2}`),
		}},
		Sinks: []Sink{{
			Name:   "audio_ws",
			Type:   SinkTypeWebSocket,
			Inputs: InputRefsFromNames("opus"),
			Config: json.RawMessage(`{"url":"wss://example.com/audio"}`),
		}},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
}

func TestValidateWebSocketRejectsVideoAndInvalidURL(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr string
	}{
		{
			name: "video input",
			job: &Job{
				Name:    "bad-media",
				Sources: []Source{{Name: "src", Type: SourceTypeRTMPPull}},
				Nodes: []Node{{
					Name: "video", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src"),
				}},
				Sinks: []Sink{{
					Name: "audio_ws", Type: SinkTypeWebSocket,
					Inputs: InputRefsFromNames("video"),
					Config: json.RawMessage(`{"url":"wss://example.com/audio"}`),
				}},
			},
			wantErr: "none of its inputs provide matching media types",
		},
		{
			name: "invalid URL",
			job: &Job{
				Name:    "bad-url",
				Sources: []Source{{Name: "src", Type: SourceTypeRTMPPull}},
				Sinks: []Sink{{
					Name: "audio_ws", Type: SinkTypeWebSocket,
					Inputs: InputRefsFromNames("src"),
					Config: json.RawMessage(`{"url":"https://example.com/audio"}`),
				}},
			},
			wantErr: "absolute ws:// or wss:// URL",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.job.EnsureReady()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}
