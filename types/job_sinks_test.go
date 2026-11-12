package avflow

import "testing"

func TestJobAllSinksFailed(t *testing.T) {
	job := &Job{Sinks: []Sink{{Name: "a"}, {Name: "b"}}}
	job.Init()
	if job.AllSinksFailed() {
		t.Fatal("expected false with no errors")
	}
	job.SetComponentStatus("a", StatusError, nil)
	if job.AllSinksFailed() {
		t.Fatal("expected false with one sink failed")
	}
	job.SetComponentStatus("b", StatusError, nil)
	if !job.AllSinksFailed() {
		t.Fatal("expected true when all sinks failed")
	}
}

func TestJobAllSourcesFailed(t *testing.T) {
	job := &Job{Sources: []Source{{Name: "a"}, {Name: "b"}}}
	job.Init()
	if job.AllSourcesFailed() {
		t.Fatal("expected false with no errors")
	}
	job.SetComponentStatus("a", StatusError, nil)
	if job.AllSourcesFailed() {
		t.Fatal("expected false with one source failed")
	}
	job.SetComponentStatus("b", StatusError, nil)
	if !job.AllSourcesFailed() {
		t.Fatal("expected true when all sources failed")
	}
}
