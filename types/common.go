// Package avflow defines the wire-format protocol types for AVFlow.
//
// # Error-code naming convention
//
// ErrorInfo.Code is a free-form string on the wire (for forward-compat —
// new codes don't need a protocol bump), but producers SHOULD follow a
// dotted lowercase namespace:
//
//	<scope>.<reason>
//
// where <scope> is one of: source, sink, node, engine, pipeline.
// Examples: "source.connect_failed", "sink.storage_error",
// "engine.timeout", "pipeline.crash", "source.screencast_no_bridge".
//
// The high-frequency codes (connect/disconnect/timeout/internal) live as
// constants below so call sites get spell-checking; long-tail per-site
// codes (e.g. "source.screencast_no_bridge") may use string literals
// directly. Consumers MUST treat unknown codes as opaque — switch on
// known prefixes (source.* / sink.* / engine.*) and surface everything
// else through a generic "unknown" bucket.
package avflow

import "encoding/json"

type MediaType string

const (
	MediaTypeAudio MediaType = "audio"
	MediaTypeVideo MediaType = "video"
	MediaTypeData  MediaType = "data"
)

// Hard caps for Policies. SetDefaults clamps values to these.
const (
	MaxJobDurationSec  = 24 * 3600 // 24h
	MaxIdleTimeoutSec  = 300       // 5m
	DefaultIdleTimeout = 60        // typical job idle; pull sources may use startTimeoutMs separately
)

type Policies struct {
	MaxDurationSec int `json:"maxDurationSec,omitempty"`
	IdleTimeoutSec int `json:"idleTimeoutSec,omitempty"`
}

func (p *Policies) SetDefaults() {
	if p.MaxDurationSec < 0 {
		p.MaxDurationSec = 0
	}
	if p.MaxDurationSec > MaxJobDurationSec {
		p.MaxDurationSec = MaxJobDurationSec
	}
	if p.IdleTimeoutSec == 0 {
		p.IdleTimeoutSec = DefaultIdleTimeout
	}
	if p.IdleTimeoutSec < 0 {
		p.IdleTimeoutSec = 0
	}
	if p.IdleTimeoutSec > MaxIdleTimeoutSec {
		p.IdleTimeoutSec = MaxIdleTimeoutSec
	}
}

type SelectFilter struct {
	MediaTypes        []MediaType `json:"mediaTypes,omitempty" jsonschema:"uniqueItems=true"`
	Identities        []string    `json:"identities,omitempty"`
	ExcludeIdentities []string    `json:"excludeIdentities,omitempty"`
	TrackSources      []string    `json:"trackSources,omitempty"`
	TrackNames        []string    `json:"trackNames,omitempty"`
	TrackSids         []string    `json:"trackSids,omitempty"`
}

func (s *SelectFilter) SetDefaults(allowedTypes []MediaType) {
	if len(s.MediaTypes) == 0 {
		s.MediaTypes = append([]MediaType(nil), allowedTypes...)
	}
}

func containsMediaType(mediaTypes []MediaType, wanted MediaType) bool {
	for _, mediaType := range mediaTypes {
		if mediaType == wanted {
			return true
		}
	}
	return false
}

// InputRef is one incoming DAG edge. Select filters audio, video, and data
// arriving through this edge, and is the only place a node or sink can filter
// its input; what survives is further limited by the consumer's own
// InputMediaTypes.
//
// On the wire, an input without an edge filter keeps the legacy string form:
//
//	"source"
//
// An edge with a filter uses the object form:
//
//	{"name":"source","select":{"identities":["alice"]}}
//	{"name":"agent","select":{"mediaTypes":["data"]}}
type InputRef struct {
	Name   string        `json:"name"`
	Select *SelectFilter `json:"select,omitempty"`
}

func (r *InputRef) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		r.Name = name
		r.Select = nil
		return nil
	}

	type inputRef InputRef
	var ref inputRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return err
	}
	*r = InputRef(ref)
	return nil
}

func (r InputRef) MarshalJSON() ([]byte, error) {
	if r.Select == nil {
		return json.Marshal(r.Name)
	}
	type inputRef InputRef
	return json.Marshal(inputRef(r))
}

type InputRefs []InputRef

func InputRefsFromNames(names ...string) InputRefs {
	refs := make(InputRefs, 0, len(names))
	for _, name := range names {
		refs = append(refs, InputRef{Name: name})
	}
	return refs
}

func (refs InputRefs) Names() []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}

type ErrorInfo struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}

type ComponentStatus struct {
	Status string     `json:"status"`
	Error  *ErrorInfo `json:"error,omitempty"`
}

const (
	StatusStarting  = "starting"
	StatusRunning   = "running"
	StatusStopping  = "stopping"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusError     = "error"
)

// Known high-frequency error codes. See the package doc for the naming
// convention; per-site codes (e.g. "source.screencast_no_bridge") may
// use string literals directly instead of adding a constant per call site.
const (
	ErrSourceConnectFailed = "source.connect_failed"
	ErrSourceDisconnected  = "source.disconnected"
	ErrSinkConnectFailed   = "sink.connect_failed"
	ErrSinkDisconnected    = "sink.disconnected"
	ErrCodecUnsupported    = "engine.codec_unsupported"
	ErrStorageError        = "sink.storage_error"
	ErrTimeout             = "engine.timeout"
	ErrInternalError       = "engine.internal_error"
)

const (
	TrackSourceCamera           = "camera"
	TrackSourceMicrophone       = "microphone"
	TrackSourceScreenShare      = "screen_share"
	TrackSourceScreenShareAudio = "screen_share_audio"
)

// TemplateVars carries the substitution values for sink storage path /
// filename templates (see protocol section 9.8). The engine populates it
// from the Job; per-track fields (identity, trackSid, …) are filled from
// each frame's metadata.
type TemplateVars struct {
	JobID       string // Job ID
	JobName     string // Human-readable job name
	Date        string // YYYY-MM-DD
	Time        string // HH-MM-SS
	Timestamp   string // Unix milliseconds
	Identity    string // Participant identity
	TrackSource string // Track source
	TrackSid    string // Track SID
	TrackName   string // Track name
	SourceName  string // Source component name (e.g. room_src, rtmp_src)
}
