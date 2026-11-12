package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	avflow "github.com/avflowhq/sdk-go/types"
)

// ErrorResponse is returned by client calls for any non-2xx response. Code is
// the machine-readable error code from the control plane, or "request_failed"
// when the body carried none (proxy errors, gateway timeouts, ...).
type ErrorResponse struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("avflow: %d %s: %s", e.StatusCode, e.Code, e.Message)
}

// Client talks to the avflow control plane. The zero value is not usable;
// construct with NewClient. Safe for concurrent use.
type Client struct {
	baseURL  string
	apiKey   string
	basePath string
	http     *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithBasePath overrides the job API path prefix. The hosted control plane
// serves "/v1/jobs" (default); a raw self-hosted engine control server serves
// "/jobs" — pass "" for that.
func WithBasePath(p string) Option {
	return func(c *Client) { c.basePath = p }
}

// WithHTTPClient overrides the underlying *http.Client (timeouts, transports).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// NewClient returns a Client for the given API base URL and bearer API key
// (avf_<appId>.<random>).
func NewClient(apiURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:  strings.TrimRight(apiURL, "/"),
		apiKey:   apiKey,
		basePath: "/v1",
		http:     http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SubmitJob creates or hot-updates a job. POST /v1/jobs is an upsert keyed by
// name: re-submitting the same name reconfigures the running pipeline instead
// of restarting it.
func (c *Client) SubmitJob(ctx context.Context, job *avflow.Job) (*avflow.Job, error) {
	return do[*avflow.Job](ctx, c, http.MethodPost, "/jobs", job)
}

// GetJob fetches a job by name.
func (c *Client) GetJob(ctx context.Context, name string) (*avflow.Job, error) {
	return do[*avflow.Job](ctx, c, http.MethodGet, "/jobs/"+url.PathEscape(name), nil)
}

// ListJobsResult is the response shape of ListJobs.
type ListJobsResult struct {
	Items []*avflow.Job `json:"items"`
	Total int           `json:"total"`
}

// ListJobs lists jobs, newest first, with optional pagination.
func (c *Client) ListJobs(ctx context.Context, limit, page int) (*ListJobsResult, error) {
	path := "/jobs"
	if limit > 0 || page > 0 {
		q := url.Values{}
		if limit > 0 {
			q.Set("limit", fmt.Sprint(limit))
		}
		if page > 0 {
			q.Set("page", fmt.Sprint(page))
		}
		path += "?" + q.Encode()
	}
	return do[*ListJobsResult](ctx, c, http.MethodGet, path, nil)
}

// StopJob stops (deletes) a job by name.
func (c *Client) StopJob(ctx context.Context, name string) (*avflow.Job, error) {
	return do[*avflow.Job](ctx, c, http.MethodDelete, "/jobs/"+url.PathEscape(name), nil)
}

func do[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var zero T

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return zero, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+c.basePath+path, reader)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return zero, err
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return zero, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var env struct {
			Code    string `json:"code"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env) // non-JSON bodies keep the zero envelope
		code := env.Code
		if code == "" {
			code = "request_failed"
		}
		msg := env.Message
		if msg == "" {
			msg = env.Error
		}
		if msg == "" {
			msg = fmt.Sprintf("avflow returned %d", res.StatusCode)
		}
		return zero, &ErrorResponse{StatusCode: res.StatusCode, Code: code, Message: msg}
	}

	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}
