package avflow

import (
	"encoding/json"
	"os"
	"time"

	"github.com/rs/xid"
)

type Job struct {
	// Request fields
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Sources  []Source          `json:"sources"`
	Nodes    []Node            `json:"nodes"`
	Sinks    []Sink            `json:"sinks"`
	Policies Policies          `json:"policies,omitempty"`

	// Response fields (read-only, populated at runtime)
	ID              string                      `json:"id,omitempty"`
	Status          string                      `json:"status,omitempty"`
	CreatedAt       string                      `json:"createdAt,omitempty"`
	UpdatedAt       string                      `json:"updatedAt,omitempty"`
	Error           *ErrorInfo                  `json:"error,omitempty"`
	ComponentStatus map[string]*ComponentStatus `json:"componentStatus,omitempty"`
}

func (j *Job) Init() {
	if j.ID == "" {
		j.ID = "job_" + xid.New().String()
	}
	if j.Status == "" {
		j.Status = StatusStarting
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if j.CreatedAt == "" {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	if j.Nodes == nil {
		j.Nodes = []Node{}
	}
	if j.ComponentStatus == nil {
		j.ComponentStatus = make(map[string]*ComponentStatus)
	}
	j.SetDefaults()
}

func (j *Job) SetDefaults() {
	// Promote nil slices to empty slices so JSON marshals them as [], not
	// null — the web pipeline relies on iterating these unconditionally.
	if j.Sources == nil {
		j.Sources = []Source{}
	}
	if j.Nodes == nil {
		j.Nodes = []Node{}
	}
	if j.Sinks == nil {
		j.Sinks = []Sink{}
	}
	j.Policies.SetDefaults()

	for i := range j.Nodes {
		n := &j.Nodes[i]
		for inputIndex := range n.Inputs {
			if n.Inputs[inputIndex].Select != nil {
				n.Inputs[inputIndex].Select.SetDefaults(n.InputMediaTypes())
			}
		}
	}

	for i := range j.Sinks {
		sk := &j.Sinks[i]
		for inputIndex := range sk.Inputs {
			if sk.Inputs[inputIndex].Select != nil {
				sk.Inputs[inputIndex].Select.SetDefaults(sk.InputMediaTypes())
			}
		}
	}
}

// Normalize clamps every source, node and sink Config to its protocol-level caps so
// that any downstream reader (Go ctors, pipeline JS over the bridge) sees the
// post-SetDefaults value. Returns the first JSON error encountered.
func (j *Job) Normalize() error {
	for i := range j.Sources {
		if err := j.Sources[i].NormalizeConfig(); err != nil {
			return err
		}
	}
	for i := range j.Nodes {
		if err := j.Nodes[i].NormalizeConfig(); err != nil {
			return err
		}
	}
	for i := range j.Sinks {
		if err := j.Sinks[i].NormalizeConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (j *Job) SetComponentStatus(componentName, status string, err *ErrorInfo) {
	j.ComponentStatus[componentName] = &ComponentStatus{
		Status: status,
		Error:  err,
	}
	j.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// AllSinksFailed reports whether every configured sink is in error state.
func (j *Job) AllSinksFailed() bool {
	if len(j.Sinks) == 0 {
		return false
	}
	for i := range j.Sinks {
		st := j.ComponentStatus[j.Sinks[i].Name]
		if st == nil || st.Status != StatusError {
			return false
		}
	}
	return true
}

// AllSourcesFailed reports whether every configured source is in error state.
func (j *Job) AllSourcesFailed() bool {
	if len(j.Sources) == 0 {
		return false
	}
	for i := range j.Sources {
		st := j.ComponentStatus[j.Sources[i].Name]
		if st == nil || st.Status != StatusError {
			return false
		}
	}
	return true
}

func (j *Job) SourceByName(name string) *Source {
	for i := range j.Sources {
		if j.Sources[i].Name == name {
			return &j.Sources[i]
		}
	}
	return nil
}

func (j *Job) NodeByName(name string) *Node {
	for i := range j.Nodes {
		if j.Nodes[i].Name == name {
			return &j.Nodes[i]
		}
	}
	return nil
}

// ASRNodes returns all ASR-type nodes in declaration order.
func (j *Job) ASRNodes() []*Node {
	var out []*Node
	for i := range j.Nodes {
		if j.Nodes[i].Type == NodeTypeASR {
			out = append(out, &j.Nodes[i])
		}
	}
	return out
}

// VoiceAgentNodes returns all voice_agent-type nodes in declaration order.
func (j *Job) VoiceAgentNodes() []*Node {
	var out []*Node
	for i := range j.Nodes {
		if j.Nodes[i].Type == NodeTypeVoiceAgent {
			out = append(out, &j.Nodes[i])
		}
	}
	return out
}

// SinkConsumesASR reports whether sk lists any ASR node among its inputs,
// i.e. this sink should render subtitles. Used instead of a job-global
// "has ASR" flag so a sibling sink without an ASR edge keeps its plain layout.
func (j *Job) SinkConsumesASR(sk *Sink) bool {
	for _, n := range j.ASRNodes() {
		for _, in := range sk.Inputs {
			if in.Name == n.Name {
				return true
			}
		}
	}
	return false
}

// SinkConsumesASRNode reports whether sk lists the named ASR node as an input.
func (j *Job) SinkConsumesASRNode(sk *Sink, asrNode string) bool {
	if asrNode == "" {
		return false
	}
	for _, in := range sk.Inputs {
		if in.Name == asrNode {
			return true
		}
	}
	return false
}

func (j *Job) SinkConsumesMedia(sk *Sink, mediaType MediaType) bool {
	if sk == nil {
		return false
	}
	outputs := make(map[string][]MediaType, len(j.Sources)+len(j.Nodes))
	for i := range j.Sources {
		outputs[j.Sources[i].Name] = j.Sources[i].OutputMediaTypes()
	}
	for i := range j.Nodes {
		outputs[j.Nodes[i].Name] = j.Nodes[i].OutputMediaTypes()
	}
	sinkMedia := sk.InputMediaTypes()
	for _, input := range sk.Inputs {
		if containsMediaType(outputs[input.Name], mediaType) &&
			consumesMedia(input.Select, sinkMedia, mediaType) {
			return true
		}
	}
	return false
}

func (j *Job) SinkByName(name string) *Sink {
	for i := range j.Sinks {
		if j.Sinks[i].Name == name {
			return &j.Sinks[i]
		}
	}
	return nil
}

// IsEmpty returns true when the job has no sources, nodes, or sinks.
func (j *Job) IsEmpty() bool {
	return len(j.Sources) == 0 && len(j.Nodes) == 0 && len(j.Sinks) == 0
}

// Dependents returns the names of nodes and sinks that reference componentName as an input.
func (j *Job) Dependents(componentName string) []string {
	var deps []string
	for i := range j.Nodes {
		for _, inp := range j.Nodes[i].Inputs {
			if inp.Name == componentName {
				deps = append(deps, j.Nodes[i].Name)
				break
			}
		}
	}
	for i := range j.Sinks {
		for _, inp := range j.Sinks[i].Inputs {
			if inp.Name == componentName {
				deps = append(deps, j.Sinks[i].Name)
				break
			}
		}
	}
	return deps
}

// MergeNode applies non-zero fields from patch onto dst.
func MergeNode(dst, patch *Node) {
	if len(patch.Inputs) > 0 {
		dst.Inputs = patch.Inputs
	}
	if len(patch.Config) > 0 {
		dst.Config = patch.Config
	}
}

// MergeSink applies non-zero fields from patch onto dst.
func MergeSink(dst, patch *Sink) {
	if len(patch.Inputs) > 0 {
		dst.Inputs = patch.Inputs
	}
	if len(patch.Config) > 0 {
		dst.Config = patch.Config
	}
}

func LoadJobFromFile(path string) (*Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJob(data)
}

func LoadJobFromStdin() (*Job, error) {
	data, err := os.ReadFile("/dev/stdin")
	if err != nil {
		return nil, err
	}
	return ParseJob(data)
}

func ParseJob(data []byte) (*Job, error) {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}
