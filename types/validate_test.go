package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJob_Validate(t *testing.T) {
	toConfig := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}

	tests := []struct {
		name    string
		job     *Job
		wantErr string
	}{
		{
			name: "Valid Simple DAG",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
		},
		{
			name: "Valid Mixer DAG",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit}, // 1:n
				},
				Nodes: []Node{
					{Name: "mix1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")}, // n:1
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("mix1")}, // 1:1
				},
			},
		},
		{
			// Segment sink must be 1:1: a multi-stream producer (axonkit) directly
			// upstream would force per-identity encoding (multiple H.264 encoders),
			// which exceeds avflow's container budget. Users must insert a mixer.
			name: "Segment Sink Direct From Multi-Stream Source Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit}, // 1:n
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeSegment, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "insert a video_mixer and/or audio_mixer",
		},
		{
			name: "Valid Segment Sink After Mixer",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit}, // 1:n
				},
				Nodes: []Node{
					{Name: "vmix", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")}, // n:1
					{Name: "amix", Type: NodeTypeAudioMixer, Inputs: InputRefsFromNames("src1")}, // n:1
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeSegment, Inputs: InputRefsFromNames("vmix", "amix")}, // 1:1
				},
			},
		},
		{
			name: "Valid Segment Sink From Single-Stream Source",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull}, // 1:1
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeSegment, Inputs: InputRefsFromNames("src1")}, // 1:1
				},
			},
		},
		{
			name: "Segment S3 Storage Empty Endpoint Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "s3",
							"storageConfig": map[string]any{
								"bucket": "recordings",
							},
						}),
					},
				},
			},
		},
		{
			name: "Segment S3 Storage Absolute Endpoint Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "s3",
							"storageConfig": map[string]any{
								"bucket":   "recordings",
								"endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
							},
						}),
					},
				},
			},
		},
		{
			name: "Segment S3 Storage Endpoint Missing Scheme Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "s3",
							"storageConfig": map[string]any{
								"bucket":   "recordings",
								"endpoint": "oss-cn-hangzhou.aliyuncs.com",
							},
						}),
					},
				},
			},
			wantErr: "s3 endpoint must be an absolute http:// or https:// URL",
		},
		{
			name: "Image S3 Storage Endpoint Missing Scheme Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeImage,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "s3",
							"storageConfig": map[string]any{
								"bucket":   "snapshots",
								"endpoint": "oss-cn-hangzhou.aliyuncs.com",
							},
						}),
					},
				},
			},
			wantErr: "s3 endpoint must be an absolute http:// or https:// URL",
		},
		{
			name: "Segment GCP Storage Missing Credentials Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "gcp",
							"storageConfig": map[string]any{
								"bucket": "recordings",
							},
						}),
					},
				},
			},
			wantErr: "gcp credentialsJson is required",
		},
		{
			name: "Image GCP Storage Missing Credentials Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeImage,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "gcp",
							"storageConfig": map[string]any{
								"bucket": "snapshots",
							},
						}),
					},
				},
			},
			wantErr: "gcp credentialsJson is required",
		},
		{
			name: "Segment GCP Storage Credentials Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"storageType": "gcp",
							"storageConfig": map[string]any{
								"bucket":          "recordings",
								"credentialsJson": map[string]any{"type": "service_account", "project_id": "demo"},
							},
						}),
					},
				},
			},
		},
		{
			name: "File Pull HTTP URL Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeFilePull, Config: toConfig(URLPullSourceConfig{URL: "https://cdn.example.com/in.mp4"})},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
		},
		{
			name: "File Pull Missing Protocol Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeFilePull, Config: toConfig(URLPullSourceConfig{URL: "/tmp/in.mp4"})},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "file_pull url must include protocol",
		},
		{
			name: "File Pull File Protocol Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeFilePull, Config: toConfig(URLPullSourceConfig{URL: "file:///tmp/in.mp4"})},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "file_pull url must not use file protocol",
		},
		{
			name: "Jitsi Xmpp Credentials Allowed",
			job: &Job{
				Sources: []Source{
					{
						Name: "src1", Type: SourceTypeJitsi,
						Config: toConfig(JitsiSourceConfig{
							Domain:       "meet.example.com",
							RoomName:     "room",
							XmppID:       "recorder@recorder.meet.example.com",
							XmppPassword: "secret",
						}),
					},
				},
				Nodes: []Node{
					{Name: "mix1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("mix1")},
				},
			},
		},
		{
			name: "Jitsi XmppID Without Password Rejected",
			job: &Job{
				Sources: []Source{
					{
						Name: "src1", Type: SourceTypeJitsi,
						Config: toConfig(JitsiSourceConfig{
							Domain:   "meet.example.com",
							RoomName: "room",
							XmppID:   "recorder@recorder.meet.example.com",
						}),
					},
				},
				Nodes: []Node{
					{Name: "mix1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("mix1")},
				},
			},
			wantErr: "xmppId and xmppPassword must both be set",
		},
		{
			name: "Jitsi XmppPassword Without ID Rejected",
			job: &Job{
				Sources: []Source{
					{
						Name: "src1", Type: SourceTypeJitsi,
						Config: toConfig(JitsiSourceConfig{
							Domain:       "meet.example.com",
							RoomName:     "room",
							XmppPassword: "secret",
						}),
					},
				},
				Nodes: []Node{
					{Name: "mix1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("mix1")},
				},
			},
			wantErr: "xmppId and xmppPassword must both be set",
		},
		{
			name: "Missing Source",
			job: &Job{
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "at least 1 source required",
		},
		{
			name: "Missing Sink",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
			},
			wantErr: "at least 1 sink required",
		},
		{
			name: "Too Many Sources",
			job: &Job{
				Sources: []Source{
					{Name: "s1", Type: SourceTypeRTMPPull},
					{Name: "s2", Type: SourceTypeRTMPPull},
					{Name: "s3", Type: SourceTypeRTMPPull},
					{Name: "s4", Type: SourceTypeRTMPPull},
					{Name: "s5", Type: SourceTypeRTMPPull},
					{Name: "s6", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("s1")},
				},
			},
			wantErr: "at most 5 sources per job",
		},
		{
			name: "Too Many Sinks",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "k1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
					{Name: "k2", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
					{Name: "k3", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
					{Name: "k4", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "at most 3 sinks per job",
		},
		{
			name: "Duplicate ID",
			job: &Job{
				Sources: []Source{
					{Name: "id1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "id1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("id1")},
				},
			},
			wantErr: "duplicate component name: id1",
		},
		{
			name: "Empty Source ID",
			job: &Job{
				Sources: []Source{
					{Name: "", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("")},
				},
			},
			wantErr: "source name is required",
		},
		{
			name: "Empty Node ID",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("")},
				},
			},
			wantErr: "node name is required",
		},
		{
			name: "Empty Sink ID",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "sink name is required",
		},
		{
			name: "Unknown Input",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("unknown")},
				},
			},
			wantErr: "references unknown input",
		},
		{
			name: "Sink as Input",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "node1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("sink1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "cannot reference sink",
		},
		{
			name: "Cycle Detected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "node1", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("node2")},
					{Name: "node2", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("node1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("node1")},
				},
			},
			wantErr: "cycle detected",
		},
		{
			name: "Orphan Source",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
					{Name: "orphan", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "source \"orphan\" is not referenced",
		},
		{
			name: "Orphan Node",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "orphan", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "node \"orphan\" output is not consumed",
		},
		{
			name: "Connection Type Mismatch (1:n -> 1:1)",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit}, // 1:n
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")}, // 1:1
				},
			},
			wantErr: "expects single stream input but upstream",
		},
		{
			name: "Media Type Mismatch (Audio -> VideoOnly)",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "audio_mix", Type: NodeTypeAudioMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeImage,
						Inputs: InputRefsFromNames("audio_mix"),
						Config: toConfig(map[string]string{"format": "jpeg"}),
					},
				},
			},
			wantErr: "none of its inputs provide matching media types",
		},
		{
			name: "Valid Media Type (Video -> VideoOnly)",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{Name: "video_mix", Type: NodeTypeVideoMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeImage,
						Inputs: InputRefsFromNames("video_mix"),
						Config: toConfig(map[string]string{"format": "jpeg"}),
					},
				},
			},
		},
		{
			name: "Invalid Select MediaType for AudioMixer",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Nodes: []Node{
					{
						Name: "audio_mix",
						Type: NodeTypeAudioMixer,
						Inputs: InputRefs{{
							Name:   "src1",
							Select: &SelectFilter{MediaTypes: []MediaType{MediaTypeVideo}},
						}},
					},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("audio_mix")},
				},
			},
			wantErr: `select filter mediaTypes contains "video", incompatible with`,
		},
		{
			name: "Job Name Required",
			job: &Job{
				Name: "",
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "job name is required",
		},
		{
			name: "Job Name Too Long",
			job: &Job{
				Name: strings.Repeat("a", MaxNameLength+1),
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "exceeds 128 characters",
		},
		{
			name: "Job Name Has Slash",
			job: &Job{
				Name: "with/slash",
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "must not contain whitespace or any of /?#",
		},
		{
			name: "Source Name Too Long",
			job: &Job{
				Sources: []Source{
					{Name: strings.Repeat("s", MaxNameLength+1), Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1")},
				},
			},
			wantErr: "exceeds 128 characters",
		},
		{
			name: "Invalid Select Audio MediaType for Image Sink",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name: "sink1",
						Type: SinkTypeImage,
						Inputs: InputRefs{{
							Name:   "src1",
							Select: &SelectFilter{MediaTypes: []MediaType{MediaTypeVideo, MediaTypeAudio}},
						}},
						Config: toConfig(map[string]string{"format": "jpeg"}),
					},
				},
			},
			wantErr: `select filter mediaTypes contains "audio", incompatible with`,
		},
		{
			name: "ASR Into Segment Sink Allowed",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "rec", Type: SinkTypeSegment, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
		},
		{
			name: "ASR Into Axonkit Sink Allowed",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "room", Type: SinkTypeAxonkit, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
		},
		{
			// WHIP carries captions as text metadata in the encoded video bitstream.
			name: "ASR Into WHIP Sink Allowed",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "push", Type: SinkTypeWHIP, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
		},
		{
			// RTMP carries captions as text metadata in H.26x SEI or AV1 Metadata OBU.
			name: "ASR Into RTMP Sink Allowed",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "push", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
		},
		{
			// SRT carries captions as text metadata in H.26x SEI or AV1 Metadata OBU.
			name: "ASR Into SRT Sink Allowed",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "push", Type: SinkTypeSRTPush, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
		},
		{
			// image has no caption path, so wiring ASR into it is rejected.
			name: "ASR Into Image Sink Rejected",
			job: &Job{
				Sources: []Source{{Name: "src1", Type: SourceTypeRTMPPull}},
				Nodes:   []Node{{Name: "asr", Type: NodeTypeASR, Inputs: InputRefsFromNames("src1")}},
				Sinks: []Sink{
					{Name: "snap", Type: SinkTypeImage, Inputs: InputRefsFromNames("src1", "asr")},
				},
			},
			wantErr: "does not support subtitles",
		},
		{
			// Explicit active_speaker without an audio_mixer is a dead config:
			// nothing computes loudness, so the main tile never switches.
			name: "Explicit Active Speaker Without Audio Mixer Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit},
				},
				Nodes: []Node{
					{
						Name:   "vmix",
						Type:   NodeTypeVideoMixer,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"layout": map[string]any{
								"mode":    "speaker",
								"speaker": map[string]any{"mainPriority": []any{"active_speaker"}},
							},
						}),
					},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("vmix")},
				},
			},
			wantErr: "no audio_mixer node",
		},
		{
			name: "Explicit Active Speaker With Audio Mixer OK",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit},
				},
				Nodes: []Node{
					{
						Name:   "vmix",
						Type:   NodeTypeVideoMixer,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"layout": map[string]any{
								"mode":    "speaker",
								"speaker": map[string]any{"mainPriority": []any{"active_speaker"}},
							},
						}),
					},
					{Name: "amix", Type: NodeTypeAudioMixer, Inputs: InputRefsFromNames("src1")},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("vmix", "amix")},
				},
			},
		},
		{
			// An unset mainPriority leaves active_speaker only as a soft fallback,
			// so a speaker layout without an audio_mixer must NOT be rejected.
			name: "Implicit Speaker Layout Without Audio Mixer OK",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeAxonkit},
				},
				Nodes: []Node{
					{
						Name:   "vmix",
						Type:   NodeTypeVideoMixer,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{
							"layout": map[string]any{"mode": "speaker"},
						}),
					},
				},
				Sinks: []Sink{
					{Name: "sink1", Type: SinkTypeRTMPPush, Inputs: InputRefsFromNames("vmix")},
				},
			},
		},
		{
			name: "Empty Video Codec Allowed When Audio Codec Present",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"audioCodec": "aac"}}),
					},
				},
			},
		},
		{
			name: "Empty Audio Codec Allowed When Video Codec Present",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h264"}}),
					},
				},
			},
		},
		{
			name: "VP8 On Non-WebRTC Sink Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "vp8", "audioCodec": "aac"}}),
					},
				},
			},
			wantErr: "does not support VP8",
		},
		{
			name: "VP8 On WebRTC Sink Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeLivekit,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "vp8", "audioCodec": "opus"}}),
					},
				},
			},
		},
		{
			name: "H265 On WebRTC Sink Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeLivekit,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h265", "audioCodec": "opus"}}),
					},
				},
			},
		},
		{
			name: "HEVC Alias On WebRTC Sink Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeLivekit,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "HEVC", "audioCodec": "opus"}}),
					},
				},
			},
		},
		{
			name: "H265 On Non-WebRTC Sink Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h265", "audioCodec": "aac"}}),
					},
				},
			},
		},
		{
			name: "AAC On WebRTC Sink Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeLivekit,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h264", "audioCodec": "aac"}}),
					},
				},
			},
			wantErr: "does not support AAC",
		},
		{
			name: "Non-Opus Audio On WebRTC Sink Rejected",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeLivekit,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h264", "audioCodec": "mp3"}}),
					},
				},
			},
			wantErr: "does not support mp3",
		},
		{
			name: "H264 On Non-WebRTC Sink Allowed",
			job: &Job{
				Sources: []Source{
					{Name: "src1", Type: SourceTypeRTMPPull},
				},
				Sinks: []Sink{
					{
						Name:   "sink1",
						Type:   SinkTypeSegment,
						Inputs: InputRefsFromNames("src1"),
						Config: toConfig(map[string]any{"encoding": map[string]any{"videoCodec": "h264", "audioCodec": "aac"}}),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All cases need a Job.Name to clear the new name check; the
			// "Job Name *" cases set it explicitly (including empty) to
			// assert on the name validator itself.
			if tt.job.Name == "" && !strings.HasPrefix(tt.name, "Job Name") {
				tt.job.Name = "test-job"
			}
			// Most table cases predate explicit codec enablement and exercise
			// unrelated graph rules. Keep their media behavior explicit.
			for i := range tt.job.Sinks {
				sink := &tt.job.Sinks[i]
				if !sinkNeedsEncodedInput(sink.Type) || sinkEncoding(sink) != nil {
					continue
				}
				var cfg map[string]any
				if len(sink.Config) > 0 {
					_ = json.Unmarshal(sink.Config, &cfg)
				}
				if cfg == nil {
					cfg = make(map[string]any)
				}
				cfg["encoding"] = map[string]any{"videoCodec": "h264", "audioCodec": "aac"}
				sink.Config = toConfig(cfg)
			}
			err := tt.job.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Validate() error = nil, want %q", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestJob_EnsureReady_FullPipeline(t *testing.T) {
	job := &Job{
		Name: "demo",
		Sources: []Source{
			{Name: "src1", Type: SourceTypeRTMPPull},
		},
		Sinks: []Sink{
			{
				Name:   "sink1",
				Type:   SinkTypeRTMPPush,
				Inputs: InputRefsFromNames("src1"),
				Config: json.RawMessage(`{"urls":["rtmp://x"],"encoding":{"videoCodec":"h264","audioCodec":"aac","keyframeIntervalSec":999}}`),
			},
		},
	}
	if err := job.EnsureReady(); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if job.ID == "" {
		t.Errorf("Init should have assigned ID")
	}
	var sinkCfg RTMPPushConfig
	if err := json.Unmarshal(job.Sinks[0].Config, &sinkCfg); err != nil {
		t.Fatalf("unmarshal sink: %v", err)
	}
	if sinkCfg.Encoding == nil || sinkCfg.Encoding.KeyframeIntervalSec != MaxKeyframeIntervalSec {
		t.Errorf("KeyframeIntervalSec = %v, want %d", sinkCfg.Encoding, MaxKeyframeIntervalSec)
	}
}

func TestJob_EnsureReady_ValidateError(t *testing.T) {
	job := &Job{Name: "demo"} // no sources, no sinks
	err := job.EnsureReady()
	if err == nil {
		t.Fatal("expected validation error for empty job")
	}
	var vErr *ValidationError
	if !errorsAs(err, &vErr) {
		t.Errorf("expected *ValidationError, got %T", err)
	}
}

// errorsAs is a tiny local errors.As shim so this test file doesn't have to
// import the errors package just for one call site.
func errorsAs(err error, target any) bool {
	v, ok := target.(**ValidationError)
	if !ok {
		return false
	}
	cast, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	*v = cast
	return true
}
