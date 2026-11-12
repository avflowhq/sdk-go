package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	avflow "github.com/avflowhq/sdk-go/types"
)

func testJob() *avflow.Job {
	return &avflow.Job{
		Name: "relay",
		Sources: []avflow.Source{{
			Name: "src",
			Type: avflow.SourceTypeRTMPPull,
			Config: json.RawMessage(
				`{"url":"rtmp://in/live"}`,
			),
		}},
		Sinks: []avflow.Sink{{
			Name:   "out",
			Type:   avflow.SinkTypeRTMPPush,
			Inputs: avflow.InputRefs{{Name: "src"}},
			Config: json.RawMessage(`{"urls":["rtmp://out/live"]}`),
		}},
	}
}

func TestSubmitJobRoundTrip(t *testing.T) {
	var gotPath, gotMethod, gotAuth string
	var gotBody avflow.Job
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotAuth = r.URL.Path, r.Method, r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "relay", "id": "j_1", "status": "running",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "avf_app1.k1")
	job, err := client.SubmitJob(context.Background(), testJob())
	if err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}

	if gotPath != "/v1/jobs" || gotMethod != http.MethodPost {
		t.Errorf("request = %s %s, want POST /v1/jobs", gotMethod, gotPath)
	}
	if gotAuth != "Bearer avf_app1.k1" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotBody.Name != "relay" || len(gotBody.Sources) != 1 || len(gotBody.Sinks) != 1 {
		t.Errorf("submitted job decoded as %+v", gotBody)
	}
	if job.ID != "j_1" || job.Status != "running" {
		t.Errorf("response job = %+v", job)
	}
}

func TestBasePathOverride(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "local", WithBasePath(""))
	if _, err := client.ListJobs(context.Background(), 5, 2); err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if gotPath != "/jobs" {
		t.Errorf("path = %q, want /jobs (basePath override)", gotPath)
	}
}

func TestGetJobEncodesName(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"name":"a b"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "k")
	if _, err := client.GetJob(context.Background(), "a b"); err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if gotPath != "/v1/jobs/a%20b" {
		t.Errorf("escaped path = %q", gotPath)
	}
}

func TestErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"INVALID_JOB","message":"at least 1 source required"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "k")
	_, err := client.SubmitJob(context.Background(), testJob())

	var apiErr *ErrorResponse
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *ErrorResponse", err)
	}
	if apiErr.StatusCode != 400 || apiErr.Code != "INVALID_JOB" ||
		apiErr.Message != "at least 1 source required" {
		t.Errorf("error envelope = %+v", apiErr)
	}
}

func TestNonJSONErrorBodyFallsBackToGenericCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte("gateway timeout"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "k")
	_, err := client.GetJob(context.Background(), "x")

	var apiErr *ErrorResponse
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *ErrorResponse", err, err)
	}
	if apiErr.Code != "request_failed" || apiErr.StatusCode != 504 {
		t.Errorf("error envelope = %+v", apiErr)
	}
}
