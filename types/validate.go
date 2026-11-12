package avflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/text/language"
)

// MaxSources / MaxSinks cap how many sources and sinks one job may declare.
// The whole pipeline runs in a single headless-Chrome page, so each RTC source
// and each encoding sink competes for the same CPU/GPU and socket budget; these
// caps keep a single job within what one container can sustain. The JSON Schema
// mirrors them via maxItems so the worker rejects oversize jobs before Go.
const (
	MaxSources = 5
	MaxSinks   = 3
)

// MaxNameLength caps job and component names. The same limit applies to job
// Name and to every Source/Node/Sink Name because names propagate into URL
// paths, log keys and metric labels; 128 keeps them human-readable while
// preventing pathological inputs. The JSON Schema mirrors this via
// maxLength so the worker rejects oversize names before they reach Go.
const MaxNameLength = 128

// nameForbiddenChars lists characters that would break URL routing or
// shell-style parsing of per-component paths. JSON Schema uses an
// equivalent pattern (see gen-schema) so the worker layer enforces the
// same rule client-side.
const nameForbiddenChars = "/?# \t\r\n"

// ValidationError describes a structural problem with a job that the caller
// should not silently fix. Validate returns *ValidationError so callers can
// type-assert and surface the Code (e.g. to the HTTP layer).
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newValidationError(msg string) *ValidationError {
	return &ValidationError{Code: "INVALID_JOB", Message: msg}
}

// validateName enforces the shared name rules: non-empty, within
// MaxNameLength, no characters that would break URL routing.
func validateName(kind, name string) *ValidationError {
	if name == "" {
		return newValidationError(fmt.Sprintf("%s name is required", kind))
	}
	if len(name) > MaxNameLength {
		return newValidationError(fmt.Sprintf("%s name %q exceeds %d characters", kind, name, MaxNameLength))
	}
	if strings.ContainsAny(name, nameForbiddenChars) {
		return newValidationError(fmt.Sprintf("%s name %q must not contain whitespace or any of /?#", kind, name))
	}
	return nil
}

func validateSourceConfig(s *Source) *ValidationError {
	switch s.Type {
	case SourceTypeJitsi:
		if len(s.Config) == 0 {
			return nil
		}
		var cfg JitsiSourceConfig
		if err := s.ParseConfig(&cfg); err != nil {
			return newValidationError(fmt.Sprintf("source %q has invalid config: %v", s.Name, err))
		}
		id, password := strings.TrimSpace(cfg.XmppID), cfg.XmppPassword
		if (id == "") != (password == "") {
			return newValidationError(fmt.Sprintf("source %q xmppId and xmppPassword must both be set", s.Name))
		}
	case SourceTypeFilePull:
		var cfg URLPullSourceConfig
		if err := s.ParseConfig(&cfg); err != nil {
			return newValidationError(fmt.Sprintf("source %q has invalid config: %v", s.Name, err))
		}
		if err := cfg.ValidateFilePullURL(); err != nil {
			return newValidationError(fmt.Sprintf("source %q: %v", s.Name, err))
		}
	case SourceTypeVideoGenerator:
		if len(s.Config) == 0 {
			return nil
		}
		var cfg VideoGeneratorSourceConfig
		if err := s.ParseConfig(&cfg); err != nil {
			return newValidationError(fmt.Sprintf("source %q has invalid config: %v", s.Name, err))
		}
	}
	return nil
}

func validateSinkConfig(s *Sink) *ValidationError {
	switch s.Type {
	case SinkTypeSegment, SinkTypeImage:
		if len(s.Config) == 0 {
			return nil
		}
		var storage struct {
			StorageType   string          `json:"storageType"`
			StorageConfig json.RawMessage `json:"storageConfig"`
		}
		if err := s.ParseConfig(&storage); err != nil {
			return newValidationError(fmt.Sprintf("sink %q has invalid config: %v", s.Name, err))
		}
		if storage.StorageType != "s3" || len(storage.StorageConfig) == 0 {
			if storage.StorageType != "gcp" || len(storage.StorageConfig) == 0 {
				return nil
			}
			var cfg GCPStorageConfig
			if err := json.Unmarshal(storage.StorageConfig, &cfg); err != nil {
				return newValidationError(fmt.Sprintf("sink %q has invalid gcp storageConfig: %v", s.Name, err))
			}
			creds := bytes.TrimSpace(cfg.CredentialsJSON)
			if len(creds) == 0 || bytes.Equal(creds, []byte("null")) {
				return newValidationError(fmt.Sprintf("sink %q gcp credentialsJson is required", s.Name))
			}
			return nil
		}
		var cfg S3StorageConfig
		if err := json.Unmarshal(storage.StorageConfig, &cfg); err != nil {
			return newValidationError(fmt.Sprintf("sink %q has invalid s3 storageConfig: %v", s.Name, err))
		}
		if cfg.Endpoint != "" {
			u, err := url.Parse(cfg.Endpoint)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return newValidationError(fmt.Sprintf(
					"sink %q s3 endpoint must be an absolute http:// or https:// URL", s.Name))
			}
		}
	case SinkTypeWebSocket:
		var cfg WebSocketSinkConfig
		if err := s.ParseConfig(&cfg); err != nil {
			return newValidationError(fmt.Sprintf("sink %q has invalid config: %v", s.Name, err))
		}
		u, err := url.Parse(cfg.URL)
		if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" {
			return newValidationError(fmt.Sprintf(
				"sink %q websocket url must be an absolute ws:// or wss:// URL", s.Name))
		}
	}
	return nil
}

func validateNodeConfig(n *Node) *ValidationError {
	if n.Type != NodeTypeTranslate {
		return nil
	}
	var cfg TranslateNodeConfig
	if err := json.Unmarshal(n.Config, &cfg); err != nil {
		return newValidationError(fmt.Sprintf("node %q has invalid config: %v", n.Name, err))
	}
	target := strings.TrimSpace(cfg.TargetLanguage)
	tag, err := language.Parse(strings.ReplaceAll(target, "_", "-"))
	if target == "" || err != nil || tag == language.Und {
		return newValidationError(fmt.Sprintf(
			"node %q translate targetLanguage must be a valid BCP-47 language tag", n.Name))
	}
	switch cfg.Provider {
	case "", TranslateProviderOpenAI, TranslateProviderGemini:
	default:
		return newValidationError(fmt.Sprintf(
			"node %q has unsupported translate provider %q", n.Name, cfg.Provider))
	}
	return nil
}

// Connection cardinality of a component.
//   - 1:1   single input stream → single output stream
//   - 1:n   single input → multi-stream output (axonkit source subscribes to many tracks)
//   - n:1   multi-stream input → single output (video/audio mixer)
//   - n:n   multi-stream input → multi-stream output (segment / image sink)
const (
	ConnType1To1 = "1:1"
	ConnType1ToN = "1:n"
	ConnTypeNTo1 = "n:1"
	ConnTypeNToN = "n:n"
)

type componentInfo struct {
	name           string
	kind           string // "source", "node", "sink"
	typ            string // SourceType, NodeType, SinkType
	connType       string // ConnType*
	inputs         []string
	inputRefs      InputRefs
	outputMedia    []MediaType
	consumeMedia   []MediaType
	outputEncoded  bool
	consumeRaw     bool
	consumeEncoded bool
}

// Validate enforces structural rules that Normalize cannot fix silently:
// required components, name uniqueness, valid input references, DAG acyclicity,
// media-type compatibility, orphan detection and connection cardinality.
//
// Soft caps (bitrates, fps, intervals) are clamped by Normalize, not here.
// Call EnsureReady to run Init + Normalize + Validate in one shot.
func (j *Job) Validate() error {
	if err := j.ExpandSinkEncoders(); err != nil {
		return err
	}
	if err := checkSingleVideoEncoder(j); err != nil {
		return err
	}
	if err := validateName("job", j.Name); err != nil {
		return err
	}
	if len(j.Sources) == 0 {
		return newValidationError("at least 1 source required")
	}
	if len(j.Sources) > MaxSources {
		return newValidationError(fmt.Sprintf("at most %d sources per job (got %d)", MaxSources, len(j.Sources)))
	}
	if len(j.Sinks) == 0 {
		return newValidationError("at least 1 sink required")
	}
	if len(j.Sinks) > MaxSinks {
		return newValidationError(fmt.Sprintf("at most %d sinks per job (got %d)", MaxSinks, len(j.Sinks)))
	}

	components := make(map[string]*componentInfo)
	var orderedIDs []string

	for i := range j.Sources {
		s := &j.Sources[i]
		if err := validateName("source", s.Name); err != nil {
			return err
		}
		if _, dup := components[s.Name]; dup {
			return newValidationError(fmt.Sprintf("duplicate component name: %s", s.Name))
		}
		if err := validateSourceConfig(s); err != nil {
			return err
		}
		ct := ConnType1To1
		if s.Type == SourceTypeAxonkit || s.Type == SourceTypeLivekit || s.Type == SourceTypeJitsi || s.Type == SourceTypeDaily || s.Type == SourceTypeAgora {
			ct = ConnType1ToN
		}
		components[s.Name] = &componentInfo{
			name:           s.Name,
			kind:           "source",
			typ:            string(s.Type),
			connType:       ct,
			outputMedia:    s.OutputMediaTypes(),
			consumeRaw:     false,
			consumeEncoded: false,
		}
		orderedIDs = append(orderedIDs, s.Name)
	}

	for i := range j.Nodes {
		n := &j.Nodes[i]
		if err := validateName("node", n.Name); err != nil {
			return err
		}
		if _, dup := components[n.Name]; dup {
			return newValidationError(fmt.Sprintf("duplicate component name: %s", n.Name))
		}
		if len(n.Inputs) == 0 {
			return newValidationError(fmt.Sprintf("node %q has no inputs", n.Name))
		}
		if err := validateNodeConfig(n); err != nil {
			return err
		}
		for inputIndex, input := range n.Inputs {
			if err := validateSelectFilterMediaTypes(input.Select, n.InputMediaTypes()); err != nil {
				return newValidationError(fmt.Sprintf(
					"node %q input %d (%q): %v", n.Name, inputIndex, input.Name, err))
			}
		}

		ct := ConnType1To1
		switch n.Type {
		case NodeTypeVideoMixer, NodeTypeAudioMixer, NodeTypeASR, NodeTypeVoiceAgent:
			ct = ConnTypeNTo1
		case NodeTypeAudioResample:
			ct = ConnTypeNToN
		}

		components[n.Name] = &componentInfo{
			name:          n.Name,
			kind:          "node",
			typ:           string(n.Type),
			connType:      ct,
			inputs:        n.Inputs.Names(),
			inputRefs:     n.Inputs,
			outputMedia:   n.OutputMediaTypes(),
			consumeMedia:  n.InputMediaTypes(),
			outputEncoded: n.outputsEncodedMedia(),
			consumeRaw:    true,
		}
		orderedIDs = append(orderedIDs, n.Name)
	}

	for i := range j.Sinks {
		sk := &j.Sinks[i]
		if err := validateName("sink", sk.Name); err != nil {
			return err
		}
		if _, dup := components[sk.Name]; dup {
			return newValidationError(fmt.Sprintf("duplicate component name: %s", sk.Name))
		}
		if err := validateSinkConfig(sk); err != nil {
			return err
		}
		if len(sk.Inputs) == 0 {
			return newValidationError(fmt.Sprintf("sink %q has no inputs", sk.Name))
		}
		for inputIndex, input := range sk.Inputs {
			if err := validateSelectFilterMediaTypes(input.Select, sk.InputMediaTypes()); err != nil {
				return newValidationError(fmt.Sprintf(
					"sink %q input %d (%q): %v", sk.Name, inputIndex, input.Name, err))
			}
		}

		// Sinks default to 1:1 (consume a single stream).
		//
		// Image and WebSocket sinks preserve independent upstream streams — n:n.
		//
		// Segment sink is INTENTIONALLY 1:1, not n:n: it runs a full H.264/AAC
		// encoder per upstream stream, which is too resource-heavy for avflow's
		// container budget. Users that want HLS recording of an RTC room must
		// first mux through a video_mixer / audio_mixer (n:1) so segment sees a
		// single composed stream. Connecting segment directly to a multi-stream
		// producer (axonkit, etc.) is rejected by checkConnectionCompatibility
		// with a hint to insert a mixer.
		ct := ConnType1To1
		if sk.Type == SinkTypeImage || sk.Type == SinkTypeWebSocket {
			ct = ConnTypeNToN
		}

		components[sk.Name] = &componentInfo{
			name:           sk.Name,
			kind:           "sink",
			typ:            string(sk.Type),
			connType:       ct,
			inputs:         sk.Inputs.Names(),
			inputRefs:      sk.Inputs,
			consumeMedia:   sk.InputMediaTypes(),
			consumeRaw:     sk.acceptsRawMedia(),
			consumeEncoded: sk.acceptsEncodedMedia(),
		}
		orderedIDs = append(orderedIDs, sk.Name)
	}

	for _, id := range orderedIDs {
		comp := components[id]
		for _, inputID := range comp.inputs {
			upstream, ok := components[inputID]
			if !ok {
				return newValidationError(fmt.Sprintf("component %q references unknown input %q", id, inputID))
			}
			if upstream.kind == "sink" {
				return newValidationError(fmt.Sprintf("component %q cannot reference sink %q as input", id, inputID))
			}
		}
	}

	if err := checkSingleDailyEndpoint(j); err != nil {
		return err
	}
	if err := detectCycles(components); err != nil {
		return err
	}
	if err := checkSubtitleCapability(components); err != nil {
		return err
	}
	if err := checkCaptionsRequireVideo(j); err != nil {
		return err
	}
	if err := checkDataMetadataRequiresVideo(j); err != nil {
		return err
	}
	if err := checkMediaCompatibility(components); err != nil {
		return err
	}
	if err := checkOrphanSources(components); err != nil {
		return err
	}
	if err := checkOrphanNodes(components); err != nil {
		return err
	}
	if err := checkConnectionCompatibility(components); err != nil {
		return err
	}
	if err := checkSinkCodecs(j); err != nil {
		return err
	}
	if err := checkEncoderCodecs(j); err != nil {
		return err
	}
	return checkActiveSpeakerNeedsAudioMixer(j)
}

func checkSingleVideoEncoder(j *Job) error {
	count := 0
	for i := range j.Nodes {
		if j.Nodes[i].Type == NodeTypeVideoEncoder {
			count++
		}
	}
	if count > 1 {
		return newValidationError(fmt.Sprintf(
			"at most one video_encoder per job, including encoders expanded from sink encoding (got %d)",
			count))
	}
	return nil
}

// Captions on real-time transports are carried inside encoded video. HLS is
// the only sink with an independent subtitle rendition and therefore the only
// sink that accepts captions without a video input.
func checkCaptionsRequireVideo(j *Job) error {
	outputs := make(map[string][]MediaType, len(j.Sources)+len(j.Nodes))
	for i := range j.Sources {
		outputs[j.Sources[i].Name] = j.Sources[i].OutputMediaTypes()
	}
	for i := range j.Nodes {
		outputs[j.Nodes[i].Name] = j.Nodes[i].OutputMediaTypes()
	}
	for i := range j.Sinks {
		sink := &j.Sinks[i]
		if sink.Type == SinkTypeSegment {
			continue
		}
		hasCaption := false
		hasVideo := false
		sinkMedia := sink.InputMediaTypes()
		for _, input := range sink.Inputs {
			mediaTypes := outputs[input.Name]
			if node := j.NodeByName(input.Name); node != nil &&
				node.Type == NodeTypeASR &&
				consumesMedia(input.Select, sinkMedia, MediaTypeData) {
				hasCaption = true
			}
			if containsMediaType(mediaTypes, MediaTypeVideo) &&
				consumesMedia(input.Select, sinkMedia, MediaTypeVideo) {
				hasVideo = true
			}
		}
		if hasCaption && !hasVideo {
			return newValidationError(fmt.Sprintf(
				"sink %q (type %s) requires a video input to carry captions; only segment/HLS supports captions without video",
				sink.Name, sink.Type))
		}
	}
	return nil
}

func checkDataMetadataRequiresVideo(j *Job) error {
	outputs := make(map[string][]MediaType, len(j.Sources)+len(j.Nodes))
	for i := range j.Sources {
		outputs[j.Sources[i].Name] = j.Sources[i].OutputMediaTypes()
	}
	for i := range j.Nodes {
		outputs[j.Nodes[i].Name] = j.Nodes[i].OutputMediaTypes()
	}
	for i := range j.Sinks {
		sink := &j.Sinks[i]
		switch sink.Type {
		case SinkTypeSegment, SinkTypeRTMPPush, SinkTypeSRTPush, SinkTypeWHIP:
		default:
			continue
		}
		hasData := false
		hasVideo := false
		sinkMedia := sink.InputMediaTypes()
		for _, input := range sink.Inputs {
			mediaTypes := outputs[input.Name]
			if containsMediaType(mediaTypes, MediaTypeData) &&
				consumesMedia(input.Select, sinkMedia, MediaTypeData) {
				if sink.Type == SinkTypeSegment {
					if n := j.NodeByName(input.Name); n != nil && n.Type == NodeTypeASR {
						continue
					}
				}
				hasData = true
			}
			if containsMediaType(mediaTypes, MediaTypeVideo) &&
				consumesMedia(input.Select, sinkMedia, MediaTypeVideo) {
				hasVideo = true
			}
		}
		if hasData && !hasVideo {
			return newValidationError(fmt.Sprintf(
				"sink %q (type %s) requires a video input to carry data-event metadata",
				sink.Name, sink.Type))
		}
	}
	return nil
}

// checkSinkCodecs enforces codec/sink-type compatibility:
//
//   - VP8 is a WebRTC-only output codec. WebRTC sinks (livekit/axonkit/jitsi/
//     daily/whip) encode it in-browser, but non-WebRTC sinks (segment/
//     rtmp_push/srt_push/image) go through the AVEncoder bridge which has no VP8
//     path, so VP8 there is rejected up front.
//   - WebRTC sink audio is always Opus. A non-empty audioCodec other than opus
//     is rejected up front because browser RTCPeerConnections do not publish
//     AAC (or any other mux audio codec).
func checkSinkCodecs(j *Job) error {
	for i := range j.Sinks {
		sk := &j.Sinks[i]
		enc := sinkEncoding(sk)
		if enc == nil {
			continue
		}

		videoCodec := normalizeSinkCodec(enc.VideoCodec)
		audioCodec := normalizeSinkCodec(enc.AudioCodec)

		if videoCodec == "vp8" && !sk.Type.IsWebRTC() {
			return &ValidationError{
				Code: "unsupported_sink_codec",
				Message: fmt.Sprintf(
					"sink %q (type %s) does not support VP8; VP8 is only available on WebRTC sinks ",
					sk.Name, sk.Type),
			}
		}
		if sk.Type.IsWebRTC() && audioCodec != "" && audioCodec != "opus" {
			label := enc.AudioCodec
			if audioCodec == "aac" {
				label = "AAC"
			}
			return &ValidationError{
				Code: "unsupported_sink_codec",
				Message: fmt.Sprintf(
					"sink %q (type %s) does not support %s; WebRTC sinks publish Opus audio",
					sk.Name, sk.Type, label),
			}
		}
	}
	return nil
}

func checkEncoderCodecs(j *Job) error {
	for i := range j.Nodes {
		node := &j.Nodes[i]
		switch node.Type {
		case NodeTypeVideoEncoder:
			var cfg VideoEncoderConfig
			if err := json.Unmarshal(node.Config, &cfg); err != nil {
				return newValidationError(fmt.Sprintf(
					"node %q has invalid video encoder config: %v", node.Name, err))
			}
			switch normalizeSinkCodec(cfg.Codec) {
			case "h264", "h265", "vp9", "av1":
			default:
				return &ValidationError{Code: "unsupported_encoder_codec", Message: fmt.Sprintf(
					"node %q (video_encoder) has unsupported codec %q", node.Name, cfg.Codec)}
			}
		case NodeTypeAudioEncoder:
			var cfg AudioEncoderConfig
			if err := json.Unmarshal(node.Config, &cfg); err != nil {
				return newValidationError(fmt.Sprintf(
					"node %q has invalid audio encoder config: %v", node.Name, err))
			}
			switch normalizeSinkCodec(cfg.Codec) {
			case "aac", "opus":
			default:
				return &ValidationError{Code: "unsupported_encoder_codec", Message: fmt.Sprintf(
					"node %q (audio_encoder) has unsupported codec %q", node.Name, cfg.Codec)}
			}
		}
	}
	return nil
}

// sinkEncoding returns config.encoding of a sink, or nil if unset/unparsable.
func sinkEncoding(sk *Sink) *EncodingConfig {
	if len(sk.Config) == 0 {
		return nil
	}
	var cfg struct {
		Encoding *EncodingConfig `json:"encoding"`
	}
	if err := json.Unmarshal(sk.Config, &cfg); err != nil || cfg.Encoding == nil {
		return nil
	}
	return cfg.Encoding
}

// normalizeSinkCodec lowercases codec names, trims whitespace, and normalizes
// hevc to h265 to match the rest of the system.
func normalizeSinkCodec(codec string) string {
	c := strings.ToLower(strings.TrimSpace(codec))
	if c == "hevc" {
		return "h265"
	}
	return c
}

// checkActiveSpeakerNeedsAudioMixer rejects a video_mixer that explicitly asks
// for the active_speaker main priority while the job has no audio_mixer node.
// active_speaker switches the main tile by voice level, and that loudness is
// only computed by the audio_mixer (a video_mixer never subscribes audio) — so
// without one the feature silently does nothing. We only flag the *explicit*
// opt-in: an unset mainPriority leaves active_speaker as a soft fallback that
// degrades to screen_share / first slot, which is fine.
func checkActiveSpeakerNeedsAudioMixer(j *Job) error {
	for i := range j.Nodes {
		if j.Nodes[i].Type == NodeTypeAudioMixer {
			return nil
		}
	}
	for i := range j.Nodes {
		n := &j.Nodes[i]
		if n.Type == NodeTypeVideoMixer && explicitlyUsesActiveSpeaker(n) {
			return newValidationError(fmt.Sprintf(
				"video_mixer %q lists 'active_speaker' in speaker layout mainPriority but the job has no "+
					"audio_mixer node; active_speaker follows the loudest speaker, which requires an audio_mixer "+
					"to analyze audio. Add an audio_mixer node, or remove 'active_speaker' from mainPriority",
				n.Name))
		}
	}
	return nil
}

// explicitlyUsesActiveSpeaker reports whether a video_mixer's speaker layout
// names "active_speaker" as a string entry in mainPriority. Object entries
// (e.g. a pinned source) are ignored.
func explicitlyUsesActiveSpeaker(n *Node) bool {
	if len(n.Config) == 0 {
		return false
	}
	var cfg VideoMixerConfig
	if err := json.Unmarshal(n.Config, &cfg); err != nil {
		return false
	}
	if cfg.Layout.Mode != LayoutModeSpeaker || cfg.Layout.Speaker == nil {
		return false
	}
	for _, rule := range cfg.Layout.Speaker.MainPriority {
		if rule.Kind == MainPriorityActiveSpeaker {
			return true
		}
	}
	return false
}

// checkSingleDailyEndpoint enforces at most one Daily endpoint per job. The
// Daily SDK (@daily-co/daily-js) forbids more than one call object per browser
// page, and the whole pipeline runs in a single page — so two Daily sources, two
// Daily sinks, or a Daily source + Daily sink would collide at runtime with
// "Duplicate DailyIframe instances are not allowed". Reject it up front.
func checkSingleDailyEndpoint(j *Job) error {
	n := 0
	for i := range j.Sources {
		if j.Sources[i].Type == SourceTypeDaily {
			n++
		}
	}
	for i := range j.Sinks {
		if j.Sinks[i].Type == SinkTypeDaily {
			n++
		}
	}
	if n > 1 {
		return newValidationError("at most one daily endpoint per job: @daily-co/daily-js " +
			"allows only one call object per page (no daily↔daily relay in a single job)")
	}
	return nil
}

// EnsureReady prepares the job for submission by running the full pipeline:
// Init (assign IDs / timestamps), Normalize (clamp soft caps) and Validate
// (enforce structural rules). Callers should invoke this once at the boundary
// where a job enters the system; engine and worker layers should not repeat
// the individual steps.
func (j *Job) EnsureReady() error {
	j.Init()
	if err := j.Normalize(); err != nil {
		return err
	}
	if err := j.ExpandSinkEncoders(); err != nil {
		return err
	}
	j.SetDefaults()
	if err := j.Normalize(); err != nil {
		return err
	}
	return j.Validate()
}

func validateSelectFilterMediaTypes(selectFilter *SelectFilter, allowedTypes []MediaType) error {
	if selectFilter == nil || len(selectFilter.MediaTypes) == 0 {
		return nil
	}
	seen := make(map[MediaType]struct{}, len(selectFilter.MediaTypes))
	for _, mediaType := range selectFilter.MediaTypes {
		switch mediaType {
		case MediaTypeAudio, MediaTypeVideo, MediaTypeData:
		default:
			return fmt.Errorf("unknown media type: %s", mediaType)
		}
		if _, ok := seen[mediaType]; ok {
			return fmt.Errorf("duplicate media type: %s", mediaType)
		}
		seen[mediaType] = struct{}{}
		if !containsMediaType(allowedTypes, mediaType) {
			return fmt.Errorf("select filter mediaTypes contains %q, incompatible with %v",
				mediaType, allowedTypes)
		}
	}
	return nil
}

// checkConnectionCompatibility ensures single-input consumers (1:1 / 1:n) do
// not consume multi-stream producers (1:n / n:n) directly.
func checkConnectionCompatibility(components map[string]*componentInfo) error {
	for _, comp := range components {
		if comp.kind == "source" {
			continue
		}
		if comp.connType != ConnType1To1 && comp.connType != ConnType1ToN {
			continue
		}
		for _, inputID := range comp.inputs {
			up, ok := components[inputID]
			if !ok {
				continue
			}
			if up.connType == ConnType1ToN || up.connType == ConnTypeNToN {
				return newValidationError(connectionMismatchMessage(comp, up))
			}
		}
	}
	return nil
}

// connectionMismatchMessage formats a clear error when a single-stream consumer
// is wired to a multi-stream producer. For segment sinks the message points the
// user at the standard fix (insert a mixer); other components get the generic
// cardinality complaint.
func connectionMismatchMessage(consumer, producer *componentInfo) string {
	base := fmt.Sprintf(
		"component %q (type %s, %s) expects single stream input but upstream %q (type %s, %s) produces multiple streams",
		consumer.name, consumer.typ, consumer.connType,
		producer.name, producer.typ, producer.connType)
	if consumer.kind == "sink" && consumer.typ == string(SinkTypeSegment) {
		return base + "; segment sinks require a single composed input — insert a video_mixer and/or audio_mixer node between the multi-stream producer and the sink"
	}
	if consumer.kind == "node" &&
		(consumer.typ == string(NodeTypeVideoEncoder) ||
			consumer.typ == string(NodeTypeAudioEncoder)) {
		return base + "; encoder nodes require a single composed input — insert a video_mixer and/or audio_mixer node between the multi-stream producer and the encoder"
	}
	return base
}

// checkSubtitleCapability rejects wiring an ASR node into a sink whose type
// cannot render subtitles/captions. ASR output is caption-only; a sink that does
// not support subtitles would silently drop it, so we fail fast with a hint
// pointing at the subtitle-capable sink types.
func checkSubtitleCapability(components map[string]*componentInfo) error {
	for _, comp := range components {
		if comp.kind != "sink" || SinkType(comp.typ).SupportsSubtitles() {
			continue
		}
		for inputIndex, inputID := range comp.inputs {
			up, ok := components[inputID]
			if ok && up.kind == "node" && up.typ == string(NodeTypeASR) &&
				selectAllowsMedia(comp.inputRefs[inputIndex].Select, MediaTypeData) {
				return newValidationError(fmt.Sprintf(
					"sink %q (type %s) does not support subtitles but lists ASR node %q as input; "+
						"remove it or route the ASR node to a subtitle-capable sink (segment, axonkit)",
					comp.name, comp.typ, inputID))
			}
		}
	}
	return nil
}

func detectCycles(components map[string]*componentInfo) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)

	var dfs func(id string) error
	dfs = func(id string) error {
		color[id] = gray
		comp := components[id]
		for _, inputID := range comp.inputs {
			switch color[inputID] {
			case gray:
				return newValidationError(fmt.Sprintf("cycle detected involving %q and %q", id, inputID))
			case white:
				if err := dfs(inputID); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for id := range components {
		if color[id] == white {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkMediaCompatibility(components map[string]*componentInfo) error {
	for _, comp := range components {
		if len(comp.consumeMedia) == 0 || len(comp.inputs) == 0 {
			continue
		}
		for inputIndex, inputID := range comp.inputs {
			upstream := components[inputID]
			outputMedia := effectiveInputMediaTypes(
				comp.inputRefs[inputIndex],
				comp.consumeMedia,
				upstream.outputMedia,
			)
			if mediaEdgeCompatible(comp, upstream, outputMedia) {
				continue
			}
			return &ValidationError{
				Code: "INVALID_MEDIA_EDGE",
				Message: fmt.Sprintf(
					"component %q consumes %v but none of its inputs provide matching media types; "+
						"input %q (type %s) outputs %v (encoded=%t)",
					comp.name, comp.consumeMedia, upstream.name, upstream.typ,
					outputMedia, upstream.outputEncoded),
			}
		}
	}
	return nil
}

func effectiveInputMediaTypes(
	input InputRef,
	allowed []MediaType,
	upstream []MediaType,
) []MediaType {
	out := make([]MediaType, 0, len(upstream))
	for _, mediaType := range upstream {
		if consumesMedia(input.Select, allowed, mediaType) {
			out = append(out, mediaType)
		}
	}
	return out
}

func mediaEdgeCompatible(consumer, producer *componentInfo, mediaTypes []MediaType) bool {
	for _, mediaType := range mediaTypes {
		if !containsMediaType(consumer.consumeMedia, mediaType) {
			continue
		}
		if mediaType == MediaTypeData {
			return true
		}
		if producer.outputEncoded {
			if consumer.consumeEncoded {
				return true
			}
		} else if consumer.consumeRaw {
			return true
		}
	}
	return false
}

func checkOrphanSources(components map[string]*componentInfo) error {
	referenced := make(map[string]bool)
	for _, comp := range components {
		for _, inputID := range comp.inputs {
			referenced[inputID] = true
		}
	}
	for _, comp := range components {
		if comp.kind == "source" && !referenced[comp.name] {
			return newValidationError(fmt.Sprintf("source %q is not referenced by any downstream component", comp.name))
		}
	}
	return nil
}

func checkOrphanNodes(components map[string]*componentInfo) error {
	referenced := make(map[string]bool)
	for _, comp := range components {
		for _, inputID := range comp.inputs {
			referenced[inputID] = true
		}
	}
	for _, comp := range components {
		if comp.kind == "node" && !referenced[comp.name] {
			return newValidationError(fmt.Sprintf("node %q output is not consumed by any downstream component", comp.name))
		}
	}
	return nil
}
