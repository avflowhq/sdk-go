package avflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateNodeContract(t *testing.T) {
	node := Node{
		Name:   "translator",
		Type:   NodeTypeTranslate,
		Inputs: InputRefsFromNames("src"),
		Config: json.RawMessage(`{"targetLanguage":"zh-CN"}`),
	}
	if got := node.InputMediaTypes(); len(got) != 1 || got[0] != MediaTypeAudio {
		t.Fatalf("input media = %v, want [audio]", got)
	}
	got := node.OutputMediaTypes()
	if len(got) != 2 || got[0] != MediaTypeAudio || got[1] != MediaTypeData {
		t.Fatalf("output media = %v, want [audio data]", got)
	}
}

func TestTranslateNodeRequiresBCP47Target(t *testing.T) {
	job := &Job{
		Name:    "translate-test",
		Sources: []Source{{Name: "src", Type: SourceTypeRTMPPull}},
		Nodes: []Node{{
			Name:   "translator",
			Type:   NodeTypeTranslate,
			Inputs: InputRefsFromNames("src"),
			Config: json.RawMessage(`{"targetLanguage":"not a language"}`),
		}},
		Sinks: []Sink{{
			Name:   "out",
			Type:   SinkTypeAxonkit,
			Inputs: InputRefsFromNames("translator"),
		}},
	}
	err := job.Validate()
	if err == nil || !strings.Contains(err.Error(), "BCP-47") {
		t.Fatalf("Validate() error = %v, want BCP-47 error", err)
	}
}
