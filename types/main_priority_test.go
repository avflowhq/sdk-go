package avflow

import (
	"encoding/json"
	"testing"
)

func TestMainPriorityRule_UnmarshalJSON(t *testing.T) {
	var rule MainPriorityRule
	if err := json.Unmarshal([]byte(`"screen_share"`), &rule); err != nil {
		t.Fatalf("kind: %v", err)
	}
	if rule.Kind != MainPriorityScreenShare || rule.Match != nil {
		t.Fatalf("got %+v", rule)
	}

	if err := json.Unmarshal([]byte(`{"identity":"alice"}`), &rule); err != nil {
		t.Fatalf("match: %v", err)
	}
	if rule.Kind != "" || rule.Match == nil || rule.Match.Identity != "alice" {
		t.Fatalf("got %+v", rule)
	}

	if err := json.Unmarshal([]byte(`"pinned"`), &rule); err == nil {
		t.Fatal("expected error for removed pinned kind")
	}
}

func TestMainPriorityRule_MarshalJSON(t *testing.T) {
	b, err := json.Marshal(MainPriorityRule{Kind: MainPriorityActiveSpeaker})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"active_speaker"` {
		t.Fatalf("got %s", b)
	}

	b, err = json.Marshal(MainPriorityRule{Match: &MatchSource{Identity: "bob"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"identity":"bob"}` {
		t.Fatalf("got %s", b)
	}
}

func TestMatchSource_SourceName(t *testing.T) {
	var m MatchSource
	if err := json.Unmarshal([]byte(`{"sourceName":"room_src"}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.SourceName != "room_src" {
		t.Fatalf("got %q", m.SourceName)
	}
}

func TestMatchSource_Index(t *testing.T) {
	var m MatchSource
	if err := json.Unmarshal([]byte(`{"index":2}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.Index == nil || *m.Index != 2 {
		t.Fatalf("got %+v", m.Index)
	}
}
