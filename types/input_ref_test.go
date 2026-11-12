package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputRefsMixedWireForms(t *testing.T) {
	var node Node
	err := json.Unmarshal([]byte(`{
		"name":"mix",
		"type":"video_mixer",
		"inputs":[
			"room_a",
			{"name":"room_b","select":{"identities":["bob"]}},
			{"name":"agent","select":{"mediaTypes":["data"]}}
		],
		"config":{}
	}`), &node)
	if err != nil {
		t.Fatal(err)
	}
	if got := node.Inputs.Names(); len(got) != 3 ||
		got[0] != "room_a" || got[1] != "room_b" || got[2] != "agent" {
		t.Fatalf("input names = %v", got)
	}
	if node.Inputs[0].Select != nil {
		t.Fatalf("legacy string input select = %#v, want nil", node.Inputs[0].Select)
	}
	if got := node.Inputs[1].Select; got == nil || len(got.Identities) != 1 || got.Identities[0] != "bob" {
		t.Fatalf("object input select = %#v", got)
	}
	if got := node.Inputs[2].Select.MediaTypes; len(got) != 1 || got[0] != MediaTypeData {
		t.Fatalf("object input mediaTypes = %#v, want data", got)
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Inputs []json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire.Inputs[0]) != `"room_a"` {
		t.Fatalf("legacy input marshaled as %s", wire.Inputs[0])
	}
	if !strings.Contains(string(wire.Inputs[1]), `"name":"room_b"`) ||
		!strings.Contains(string(wire.Inputs[1]), `"identities":["bob"]`) {
		t.Fatalf("filtered input marshaled as %s", wire.Inputs[1])
	}
	if !strings.Contains(string(wire.Inputs[2]), `"name":"agent"`) ||
		!strings.Contains(string(wire.Inputs[2]), `"mediaTypes":["data"]`) {
		t.Fatalf("data input marshaled as %s", wire.Inputs[2])
	}
}

func TestJobSetDefaultsAppliesToInputSelect(t *testing.T) {
	job := Job{
		Nodes: []Node{{
			Name: "mix",
			Type: NodeTypeVideoMixer,
			Inputs: InputRefs{{
				Name:   "room",
				Select: &SelectFilter{Identities: []string{"alice"}},
			}},
		}},
	}
	job.SetDefaults()

	if got := job.Nodes[0].Inputs[0].Select.MediaTypes; len(got) != 1 || got[0] != MediaTypeVideo {
		t.Fatalf("edge select mediaTypes = %v, want [video]", got)
	}
}

func TestJobValidateRejectsIncompatibleInputSelectMediaType(t *testing.T) {
	job := Job{
		Name: "edge-select",
		Sources: []Source{{
			Name: "src",
			Type: SourceTypeRTMPPull,
		}},
		Nodes: []Node{{
			Name: "mix",
			Type: NodeTypeAudioMixer,
			Inputs: InputRefs{{
				Name:   "src",
				Select: &SelectFilter{MediaTypes: []MediaType{MediaTypeVideo}},
			}},
		}},
		Sinks: []Sink{{
			Name:   "out",
			Type:   SinkTypeRTMPPush,
			Inputs: InputRefsFromNames("mix"),
		}},
	}

	err := job.Validate()
	if err == nil || !strings.Contains(err.Error(), `node "mix" input 0 ("src")`) {
		t.Fatalf("Validate() error = %v", err)
	}
}
