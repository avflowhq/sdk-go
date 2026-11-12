package avflow

import (
	"encoding/json"
	"fmt"
)

// MainPriorityKind names a built-in speaker-layout main-tile selection rule.
type MainPriorityKind string

const (
	MainPriorityScreenShare   MainPriorityKind = "screen_share"
	MainPriorityActiveSpeaker MainPriorityKind = "active_speaker"
)

// MainPriorityRule is one entry in speaker.mainPriority. JSON is either a kind
// string ("screen_share", "active_speaker") or a MatchSource object.
type MainPriorityRule struct {
	Kind  MainPriorityKind `json:"-"`
	Match *MatchSource     `json:"-"`
}

func (r *MainPriorityRule) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		switch MainPriorityKind(s) {
		case MainPriorityScreenShare, MainPriorityActiveSpeaker:
			r.Kind = MainPriorityKind(s)
			r.Match = nil
			return nil
		default:
			return fmt.Errorf("mainPriority: unknown kind %q", s)
		}
	}
	if b[0] == '{' {
		var m MatchSource
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		r.Match = &m
		r.Kind = ""
		return nil
	}
	return fmt.Errorf("mainPriority: expected string or object")
}

func (r MainPriorityRule) MarshalJSON() ([]byte, error) {
	if r.Match != nil {
		return json.Marshal(r.Match)
	}
	if r.Kind != "" {
		return json.Marshal(string(r.Kind))
	}
	return []byte("null"), nil
}
