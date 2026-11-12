package sdk

import (
	event "github.com/avflowhq/sdk-go/event"
	avflow "github.com/avflowhq/sdk-go/types"
)

// Aliases re-export the wire types from the mirrored type libraries so most
// users only need to import this package. The mirrors under types/ and event/
// are synced verbatim from the avflow engine (`make sync-sdks` there) — never
// edit them here.
type (
	Job             = avflow.Job
	Source          = avflow.Source
	Sink            = avflow.Sink
	Node            = avflow.Node
	Policies        = avflow.Policies
	SelectFilter    = avflow.SelectFilter
	InputRef        = avflow.InputRef
	InputRefs       = avflow.InputRefs
	ErrorInfo       = avflow.ErrorInfo
	ComponentStatus = avflow.ComponentStatus

	LifecycleEvent    = event.LifecycleEvent
	Instance          = event.Instance
	JobRef            = event.JobRef
	ComponentRef      = event.ComponentRef
	UsageBatch        = event.UsageBatch
	UsageWindow       = event.UsageWindow
	UsageComponent    = event.UsageComponent
	InstanceHeartbeat = event.InstanceHeartbeat
)

// EventsSchemaVersion identifies the wire format of lifecycle / usage.tick
// envelopes delivered through the events ingest channel.
const EventsSchemaVersion = event.EventsSchemaVersion
