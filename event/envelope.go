package event

import (
	"log/slog"

	"github.com/avflowhq/sdk-go/types"
	"github.com/rs/xid"
)

// EventsSchemaVersion identifies the wire format of lifecycle / usage.tick
// envelopes delivered through the events ingest channel.
const EventsSchemaVersion = "avflow.events.v1"

// Lifecycle event types emitted by the engine.
const (
	EventJobStarting           = "job.starting"
	EventJobStarted            = "job.started"
	EventJobCompleted          = "job.completed"
	EventJobFailed             = "job.failed"
	EventJobMaxDurationReached = "job.max_duration_reached"
	EventJobIdleTimeout        = "job.idle_timeout"
	EventComponentError        = "component.error"
	EventUsageTick             = "usage.tick"
	EventInstanceHeartbeat     = "instance.heartbeat"
)

// ComponentRole discriminates the coarse topology slot in the job DAG.
const (
	ComponentRoleSource = "source"
	ComponentRoleNode   = "node"
	ComponentRoleSink   = "sink"
)

// Instance identifies the avflow process that emitted an event.
type Instance struct {
	ID         string `json:"id"`
	Deployment string `json:"deployment,omitempty"`
	Region     string `json:"region,omitempty"`
	Version    string `json:"version,omitempty"`
}

// JobRef is the minimal job identifier carried on every event.
type JobRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// ComponentRef identifies the component an event is about.
type ComponentRef struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Type string `json:"type,omitempty"`
}

// LifecycleEvent is one structured events-ingest payload (job.* /
// component.* / usage.tick).
type LifecycleEvent struct {
	Schema     string             `json:"schema"`
	EventID    string             `json:"eventId"`
	EventTime  string             `json:"eventTime"`
	Instance   Instance           `json:"instance"`
	AppID      string             `json:"appId,omitempty"`
	Job        JobRef             `json:"job"`
	Component  *ComponentRef      `json:"component,omitempty"`
	Event      string             `json:"event"`
	SequenceNo int64              `json:"sequenceNo,omitempty"`
	Reason     string             `json:"reason,omitempty"`
	Error      *avflow.ErrorInfo  `json:"error,omitempty"`
	Usage      *UsageBatch        `json:"usage,omitempty"`
	Heartbeat  *InstanceHeartbeat `json:"heartbeat,omitempty"`
}

// LogValue implements slog.LogValuer for audit logging.
func (e LifecycleEvent) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", e.Event),
		slog.String("eventId", e.EventID),
		slog.Int64("sequenceNo", e.SequenceNo),
		slog.Any("payload", lifecycleEventPayload(e)),
	)
}

type lifecycleEventPayload LifecycleEvent

// InstanceHeartbeat advertises a single-job avflow process to the Router.
// WSS identity is the connection id (process UUID). HTTP registration uses
// the HMAC node id as the registry key. The Router schedules jobs onto
// avflow processes by compute class / busy state; instanceId is cloud
// grouping only. It intentionally carries no credentials.
type InstanceHeartbeat struct {
	Provider   string `json:"provider"`
	Active     bool   `json:"active"`
	InstanceID string `json:"instanceId"`
	// Compute is "cpu" or "gpu" (host class for job placement).
	Compute string `json:"compute"`
	// Endpoint is the dialable control-plane base URL (http://host:port) when
	// transport is HTTP. Empty for WSS reverse-tunnel nodes (control stays on
	// the tunnel; LocalBase is loopback).
	Endpoint string `json:"endpoint,omitempty"`
}

// UsageBatch is the payload of an EventUsageTick lifecycle event.
type UsageBatch struct {
	Window         UsageWindow      `json:"window,omitempty"`
	Components     []UsageComponent `json:"components"`
	JobDurationSec int64            `json:"jobDurationSec,omitempty"`
}

// UsageWindow brackets one billing window sealed by usage.tick.
type UsageWindow struct {
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Sec   int64  `json:"sec,omitempty"`
}

// UsageComponent is one row of a UsageBatch.
// Values is embedded so source/node/sink serialize at the top level
// (not under a "values" wrapper).
type UsageComponent struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Type  string `json:"type,omitempty"`
	Since string `json:"since,omitempty"`
	Final bool   `json:"final,omitempty"`
	Values
}

// NewEventID generates a globally unique event identifier.
func NewEventID() string {
	return "evt_" + xid.New().String()
}
