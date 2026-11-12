package avflow

import "encoding/json"

type SinkType string

const (
	SinkTypeAxonkit   SinkType = "axonkit" // internal; not in public docs yet
	SinkTypeLivekit   SinkType = "livekit"
	SinkTypeJitsi     SinkType = "jitsi"
	SinkTypeDaily     SinkType = "daily"
	SinkTypeAgora     SinkType = "agora"
	SinkTypeWHIP      SinkType = "whip"
	SinkTypeRTMPPush  SinkType = "rtmp_push"
	SinkTypeSRTPush   SinkType = "srt_push"
	SinkTypeSegment   SinkType = "segment"
	SinkTypeImage     SinkType = "image"
	SinkTypeWebSocket SinkType = "websocket"
)

type Sink struct {
	Name   string          `json:"name"`
	Type   SinkType        `json:"type"`
	Inputs InputRefs       `json:"inputs"`
	Config json.RawMessage `json:"config"`
}

// SupportsSubtitles reports whether this sink type can carry an ASR node's
// caption DataEvent. Validate uses it to reject an ASR data edge when the sink
// cannot render or forward captions.
//
//   - segment: HLS WebVTT rendition (see segment_sink finalize).
//   - axonkit: forwarded as reliable publishData (label avflow.asrText).
//   - livekit: forwarded as reliable publishData (topic avflow.asrText).
//   - jitsi: forwarded as an endpoint message (type avflow.asrText).
//   - daily: forwarded as an app message (type avflow.asrText).
//   - agora: forwarded as an RTM-less data-stream message (type avflow.asrText).
//   - rtmp_push/srt_push/whip: embedded as avflow text metadata in H.26x SEI
//     or AV1 Metadata OBU.
//
// image has no caption path. Flip image here once its sink learns to emit captions.
func (t SinkType) SupportsSubtitles() bool {
	switch t {
	case SinkTypeSegment, SinkTypeAxonkit, SinkTypeLivekit, SinkTypeJitsi, SinkTypeDaily,
		SinkTypeAgora, SinkTypeWHIP, SinkTypeRTMPPush, SinkTypeSRTPush:
		return true
	default:
		return false
	}
}

// SupportsDataEvents reports whether the sink can carry structured non-caption
// events such as avflow.voiceAgentText.
func (t SinkType) SupportsDataEvents() bool {
	switch t {
	case SinkTypeSegment, SinkTypeAxonkit, SinkTypeLivekit, SinkTypeJitsi, SinkTypeDaily,
		SinkTypeAgora, SinkTypeWHIP, SinkTypeRTMPPush, SinkTypeSRTPush:
		return true
	default:
		return false
	}
}

// IsWebRTC reports whether this sink publishes over WebRTC (a browser
// RTCPeerConnection), where VP8 is a valid output codec. Non-WebRTC sinks encode
// through the AVEncoder bridge (h264/h265/vp9/av1) and have no VP8 path.
func (t SinkType) IsWebRTC() bool {
	switch t {
	case SinkTypeAxonkit, SinkTypeLivekit, SinkTypeJitsi, SinkTypeDaily, SinkTypeAgora, SinkTypeWHIP:
		return true
	default:
		return false
	}
}

func (s *Sink) InputMediaTypes() []MediaType {
	switch s.Type {
	case SinkTypeImage:
		return []MediaType{MediaTypeVideo}
	case SinkTypeWebSocket:
		return []MediaType{MediaTypeAudio}
	default:
		media := []MediaType{MediaTypeVideo, MediaTypeAudio}
		if s.Type.SupportsDataEvents() {
			media = append(media, MediaTypeData)
		}
		return media
	}
}

func (s *Sink) acceptsRawMedia() bool {
	switch s.Type {
	case SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeSegment:
		return false
	default:
		return true
	}
}

func (s *Sink) acceptsEncodedMedia() bool {
	switch s.Type {
	case SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeSegment, SinkTypeWebSocket:
		return true
	default:
		return false
	}
}

func (s *Sink) ParseConfig(v any) error {
	return json.Unmarshal(s.Config, v)
}

// NormalizeConfig parses Config by Type, calls its SetDefaults (clamping caps
// and applying defaults), and writes the result back to s.Config. This makes
// the normalized values visible to every downstream consumer of s.Config,
// including pipeline code that reads the raw JSON in the browser.
//
// Returns nil for sink types whose config carries no SetDefaults (or for an
// empty Config). JSON errors propagate so validators can reject the job.
func (s *Sink) NormalizeConfig() error {
	if len(s.Config) == 0 {
		return nil
	}
	var target interface {
		SetDefaults()
	}
	switch s.Type {
	case SinkTypeAxonkit:
		target = &AxonkitSinkConfig{}
	case SinkTypeLivekit:
		target = &LivekitSinkConfig{}
	case SinkTypeJitsi:
		target = &JitsiSinkConfig{}
	case SinkTypeDaily:
		target = &DailySinkConfig{}
	case SinkTypeAgora:
		target = &AgoraSinkConfig{}
	case SinkTypeRTMPPush:
		target = &RTMPPushConfig{}
	case SinkTypeSRTPush:
		target = &SRTPushConfig{}
	case SinkTypeWHIP:
		target = &WHIPPushConfig{}
	case SinkTypeSegment:
		target = &SegmentSinkConfig{}
	case SinkTypeImage:
		target = &ImageSinkConfig{}
	case SinkTypeWebSocket:
		target = &WebSocketSinkConfig{}
	default:
		return nil
	}
	if err := json.Unmarshal(s.Config, target); err != nil {
		return err
	}
	target.SetDefaults()
	out, err := json.Marshal(target)
	if err != nil {
		return err
	}
	s.Config = out
	return nil
}

// Hard caps for EncodingConfig. SetDefaults clamps values to these; 0 = auto.
const (
	MaxVideoBitrateBps     = 50_000_000
	MaxAudioBitrateBps     = 512_000
	MaxKeyframeIntervalSec = 10
	// DefaultBFrames is the FFmpeg max_b_frames when encoding.enableBFrames is true.
	DefaultBFrames = 3
	// DefaultKeyframeIntervalSec is the GOP interval when keyframeIntervalSec is omitted.
	DefaultKeyframeIntervalSec = 2
	// DefaultEncodingAudioSampleRate is the fallback sink audio encode sample rate
	// when upstream media does not expose a usable rate.
	DefaultEncodingAudioSampleRate = 48_000
)

// AllowedEncodingAudioSampleRates lists sink audio encode rates (Hz). Values
// outside this set are snapped to the nearest supported entry when sink
// encoders derive an output rate from upstream media.
var AllowedEncodingAudioSampleRates = [...]int{8_000, 16_000, 32_000, 44_100, 48_000}

func SnapEncodingAudioSampleRate(sr int) int {
	if sr <= 0 {
		return DefaultEncodingAudioSampleRate
	}
	for _, v := range AllowedEncodingAudioSampleRates {
		if sr == v {
			return sr
		}
	}
	best := AllowedEncodingAudioSampleRates[0]
	bestDiff := sr - best
	if bestDiff < 0 {
		bestDiff = -bestDiff
	}
	for _, v := range AllowedEncodingAudioSampleRates[1:] {
		d := sr - v
		if d < 0 {
			d = -d
		}
		if d < bestDiff {
			best, bestDiff = v, d
		}
	}
	return best
}

type EncodingConfig struct {
	VideoCodec          string `json:"videoCodec,omitempty"`   // Empty defaults per sink type. h264 | vp9 | av1; h265/hevc internal; vp8 on WebRTC.
	AudioCodec          string `json:"audioCodec,omitempty"`   // Empty defaults per sink type. aac | opus.
	AudioProfile        string `json:"audioProfile,omitempty"` // aac only: LC | HEv1 | HEv2
	VideoBitrateBps     int    `json:"videoBitrateBps,omitempty"`
	AudioBitrateBps     int    `json:"audioBitrateBps,omitempty"`
	KeyframeIntervalSec int    `json:"keyframeIntervalSec,omitempty"`
	EnableBFrames       bool   `json:"enableBFrames,omitempty"` // internal; not in public docs yet. false = no B-frames; true → DefaultBFrames
}

func (c *EncodingConfig) SetDefaults() {
	c.VideoCodec = normalizeSinkCodec(c.VideoCodec)
	c.AudioCodec = normalizeSinkCodec(c.AudioCodec)
	if c.VideoBitrateBps < 0 {
		c.VideoBitrateBps = 0
	}
	if c.VideoBitrateBps > MaxVideoBitrateBps {
		c.VideoBitrateBps = MaxVideoBitrateBps
	}
	if c.AudioBitrateBps < 0 {
		c.AudioBitrateBps = 0
	}
	if c.AudioBitrateBps > MaxAudioBitrateBps {
		c.AudioBitrateBps = MaxAudioBitrateBps
	}
	if c.KeyframeIntervalSec < 0 {
		c.KeyframeIntervalSec = 0
	}
	if c.KeyframeIntervalSec > MaxKeyframeIntervalSec {
		c.KeyframeIntervalSec = MaxKeyframeIntervalSec
	}
}

// ensureEncodingCodecs fills empty videoCodec/audioCodec. Sinks encode every
// audio/video track they receive; omit a track with select.mediaTypes.
func ensureEncodingCodecs(enc *EncodingConfig, video, audio string) *EncodingConfig {
	if enc == nil {
		enc = &EncodingConfig{}
	}
	if enc.VideoCodec == "" {
		enc.VideoCodec = video
	}
	if enc.AudioCodec == "" {
		enc.AudioCodec = audio
	}
	enc.SetDefaults()
	return enc
}

func ensureMuxEncoding(enc *EncodingConfig) *EncodingConfig {
	return ensureEncodingCodecs(enc, "h264", "aac")
}

// Room sinks use vp8+opus; WHIP uses h264+opus. AVFlow pins these so jobs
// do not depend on the bundled SDK version.
func ensureRTCEncoding(enc *EncodingConfig, video, audio string) *EncodingConfig {
	return ensureEncodingCodecs(enc, video, audio)
}

// BFrames returns FFmpeg max_b_frames: DefaultBFrames when EnableBFrames, else 0.
func (c *EncodingConfig) BFrames() int {
	if c == nil || !c.EnableBFrames {
		return 0
	}
	return DefaultBFrames
}

// AxonkitSinkConfig publishes the composed media into an Axonkit room
// (https://axonkit.io). The room is determined by the JWT's room grant.
type AxonkitSinkConfig struct {
	ServerURL      string          `json:"serverUrl"`
	Token          string          `json:"token"`
	VideoTrackName string          `json:"videoTrackName,omitempty"`
	AudioTrackName string          `json:"audioTrackName,omitempty"`
	Encoding       *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to vp8 + opus.
}

func (c *AxonkitSinkConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "vp8", "opus")
}

// LivekitSinkConfig publishes the composed media into a standard LiveKit room.
// Room, participant identity and display name are all carried by the JWT (the
// livekit-client SDK derives them from Token and offers no connect-time
// override), so only the credentials and published track names live here.
type LivekitSinkConfig struct {
	ServerURL      string          `json:"serverUrl"`
	Token          string          `json:"token"`
	VideoTrackName string          `json:"videoTrackName,omitempty"`
	AudioTrackName string          `json:"audioTrackName,omitempty"`
	Encoding       *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to vp8 + opus.
}

func (c *LivekitSinkConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "vp8", "opus")
}

// JitsiSinkConfig publishes the composed media into a Jitsi conference (via
// lib-jitsi-meet). Connection/conference options are passed through to the
// browser SDK. Domain identifies the Jitsi deployment and derives connection
// defaults when ConnectionOptions is omitted. Encoding is the AVFlow codec /
// bitrate knob; overlapping conferenceOptions videoQuality / audioQuality
// codec and bitrate fields are overwritten by encoding.
type JitsiSinkConfig struct {
	Domain            string          `json:"domain"`
	RoomName          string          `json:"roomName"`
	Token             string          `json:"token,omitempty"`
	AppID             string          `json:"appId,omitempty"`
	DisplayName       string          `json:"displayName,omitempty"`
	ConnectionOptions map[string]any  `json:"connectionOptions,omitempty"`
	ConferenceOptions map[string]any  `json:"conferenceOptions,omitempty"`
	Encoding          *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to vp8 + opus.
}

func (c *JitsiSinkConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "vp8", "opus")
}

// DailySinkConfig publishes the composed media into a Daily room (via
// @daily-co/daily-js). The composite video is published as the camera track and
// the composite audio as the microphone track, so any standard Daily client
// auto-subscribes to it. RoomURL is the full Daily room URL; Token authorizes
// private rooms.
type DailySinkConfig struct {
	RoomURL  string          `json:"roomUrl"`
	Token    string          `json:"token,omitempty"`
	UserName string          `json:"userName,omitempty"`
	Encoding *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to vp8 + opus.
}

func (c *DailySinkConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "vp8", "opus")
}

// AgoraSinkConfig publishes the composed media into an Agora RTC channel (via
// agora-rtc-sdk-ng). The composite video is published as the camera track and
// the composite audio as the microphone track, so any standard Agora client
// auto-subscribes to it. Agora identifies an endpoint by AppID + Channel; Token
// authorizes the join (omit for testing/no-auth projects). UID is the local
// publisher identity (number or user account; omit or 0 lets Agora assign).
type AgoraSinkConfig struct {
	AppID    string          `json:"appId"`
	Channel  string          `json:"channel"`
	Token    string          `json:"token,omitempty"`
	UID      AgoraUID        `json:"uid,omitempty"`
	Encoding *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to vp8 + opus.
}

func (c *AgoraSinkConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "vp8", "opus")
}

// RTMP push sink

type RTMPPushConfig struct {
	URLs     []string        `json:"urls"`
	Encoding *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to h264 + aac.
}

func (c *RTMPPushConfig) SetDefaults() {
	c.Encoding = ensureMuxEncoding(c.Encoding)
}

// SRTPushConfig pushes MPEG-TS over SRT in caller mode.
//
// URL form: srt://host:port/?streamid=publish:/live/stream&latency=200&passphrase=...
// SRT defaults to latency=120ms; we default to 200ms for a sturdier loss window.
type SRTPushConfig struct {
	URL string `json:"url"`
	// Encoding controls sink-side encoding. Empty codecs default to h264 + aac.
	Encoding *EncodingConfig `json:"encoding,omitempty"`
}

func (c *SRTPushConfig) SetDefaults() {
	c.Encoding = ensureMuxEncoding(c.Encoding)
}

// WHIP push sink

type WHIPPushConfig struct {
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	Encoding *EncodingConfig   `json:"encoding,omitempty"` // Empty codecs default to h264 + opus.
}

func (c *WHIPPushConfig) SetDefaults() {
	c.Encoding = ensureRTCEncoding(c.Encoding, "h264", "opus")
}

// WebSocketSinkConfig streams each input audio track over its own WebSocket
// connection. Payloads are PCM S16LE for raw inputs or access units for an
// audio_encoder input. Media metadata is sent as handshake headers.
type WebSocketSinkConfig struct {
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Subprotocols []string          `json:"subprotocols,omitempty"`
}

func (c *WebSocketSinkConfig) SetDefaults() {}

// Hard caps for Segment and Image sinks.
const (
	MaxSegmentDurationSec = 60   // HLS spec recommends 1-60s segments
	MaxPlaylistEntriesCap = 1000 // upper bound for the live sliding-window size (VOD/finalize includes all segments regardless)
	MaxImageIntervalSec   = 3600 // 1 image / hour at most
	MaxImageWidth         = 4096 // 4K
	MaxImageHeight        = 4096 // 4K

	DefaultCaptionTextSplitLength = 42
	DefaultCaptionTextSplitLines  = 2
	MaxCaptionTextSplitLength     = 200
	MaxCaptionTextSplitLines      = 10
)

// Segment sink: HLS segment storage. Filename has no extension; engine appends by format.

type SegmentSinkConfig struct {
	StorageType        string          `json:"storageType"`
	StorageConfig      json.RawMessage `json:"storageConfig"`
	Format             string          `json:"format,omitempty"`   // hls (default)
	Encoding           *EncodingConfig `json:"encoding,omitempty"` // Empty codecs default to h264 + aac.
	SegmentDurationSec int             `json:"segmentDurationSec,omitempty"`
	MaxPlaylistEntries int             `json:"maxPlaylistEntries,omitempty"`
	// Caption controls HLS WebVTT subtitle output. Upstream ASR always carries
	// speaker labels and unsplit text; this config only affects the segment
	// sink's rendered caption rendition.
	Caption *SegmentCaptionConfig `json:"caption,omitempty"`
}

// SegmentCaptionConfig controls HLS WebVTT subtitle output.
type SegmentCaptionConfig struct {
	// ShowSpeaker controls whether HLS captions include the speaker name.
	// Defaults to true when omitted. nil = default.
	ShowSpeaker *bool `json:"showSpeaker,omitempty"`
	// TextSplit optionally splits long HLS WebVTT captions into display-sized
	// cues. Omitted / enabled=false keeps one cue per ASR final.
	TextSplit *CaptionTextSplitConfig `json:"textSplit,omitempty"`
}

// CaptionTextSplitConfig controls HLS WebVTT caption display splitting.
type CaptionTextSplitConfig struct {
	Enabled bool `json:"enabled,omitempty"`
	// Length is max display columns per line (Latin=1, CJK/fullwidth/emoji=2).
	// Default 42 when Enabled and Length is omitted or <=0.
	Length int `json:"length,omitempty"`
	// Lines is max lines per cue. Default 2 when Enabled and Lines is omitted or <=0.
	Lines int `json:"lines,omitempty"`
}

func (c *SegmentSinkConfig) SetDefaults() {
	if c.SegmentDurationSec < 0 {
		c.SegmentDurationSec = 0
	}
	if c.SegmentDurationSec > MaxSegmentDurationSec {
		c.SegmentDurationSec = MaxSegmentDurationSec
	}
	if c.MaxPlaylistEntries < 0 {
		c.MaxPlaylistEntries = 0
	}
	if c.MaxPlaylistEntries > MaxPlaylistEntriesCap {
		c.MaxPlaylistEntries = MaxPlaylistEntriesCap
	}
	c.Encoding = ensureMuxEncoding(c.Encoding)
	if c.Caption != nil {
		c.Caption.SetDefaults()
	}
}

func (c *SegmentCaptionConfig) SetDefaults() {
	if c != nil && c.TextSplit != nil {
		c.TextSplit.SetDefaults()
	}
}

func (c *CaptionTextSplitConfig) SetDefaults() {
	if c == nil || !c.Enabled {
		return
	}
	if c.Length <= 0 {
		c.Length = DefaultCaptionTextSplitLength
	}
	if c.Length > MaxCaptionTextSplitLength {
		c.Length = MaxCaptionTextSplitLength
	}
	if c.Lines <= 0 {
		c.Lines = DefaultCaptionTextSplitLines
	}
	if c.Lines > MaxCaptionTextSplitLines {
		c.Lines = MaxCaptionTextSplitLines
	}
}

type ImageSinkConfig struct {
	StorageType   string          `json:"storageType"`
	StorageConfig json.RawMessage `json:"storageConfig"`
	Format        string          `json:"format,omitempty"` // jpeg | png | webp
	IntervalSec   int             `json:"intervalSec,omitempty"`
	Quality       int             `json:"quality,omitempty"` // 0-100
	Width         int             `json:"width,omitempty"`
	Height        int             `json:"height,omitempty"`
}

func (c *ImageSinkConfig) SetDefaults() {
	if c.IntervalSec < 0 {
		c.IntervalSec = 0
	}
	if c.IntervalSec > MaxImageIntervalSec {
		c.IntervalSec = MaxImageIntervalSec
	}
	if c.Quality < 0 {
		c.Quality = 0
	}
	if c.Quality > 100 {
		c.Quality = 100
	}
	if c.Width < 0 {
		c.Width = 0
	}
	if c.Width > MaxImageWidth {
		c.Width = MaxImageWidth
	}
	if c.Height < 0 {
		c.Height = 0
	}
	if c.Height > MaxImageHeight {
		c.Height = MaxImageHeight
	}
}

type S3StorageConfig struct {
	Bucket          string `json:"bucket"`
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	ForcePathStyle  bool   `json:"forcePathStyle,omitempty"`
	PathPrefix      string `json:"pathPrefix,omitempty"`
	Filename        string `json:"filename,omitempty"`
}

type GCPStorageConfig struct {
	Bucket          string          `json:"bucket"`
	CredentialsJSON json.RawMessage `json:"credentialsJson"`
	PathPrefix      string          `json:"pathPrefix,omitempty"`
	Filename        string          `json:"filename,omitempty"`
}

type AzureStorageConfig struct {
	Container   string `json:"container"`
	AccountName string `json:"accountName"`
	AccountKey  string `json:"accountKey,omitempty"`
	SASToken    string `json:"sasToken,omitempty"`
	PathPrefix  string `json:"pathPrefix,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

// LocalStorageConfig configures on-disk paths when storageType is "local".
// BaseDir is prepended to upload keys; empty BaseDir keeps keys relative to the process working directory.
type LocalStorageConfig struct {
	BaseDir string `json:"baseDir,omitempty"`
}
