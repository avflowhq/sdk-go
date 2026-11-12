# sdk-go

Go SDK for the [avflow](https://avflow.dev) control plane: typed Job
submission, lifecycle queries, and webhook signature verification.

- **Typed wire contract** — `types/` and `event/` are verbatim mirrors of the
  engine's pure type libraries (pkg/avflow, pkg/event), synced by
  `make sync-sdks` in the avflow engine repository. Zero hand-written drift.
- **Webhook verification** — constant-time HMAC-SHA256 check of
  `X-Avflow-Signature`, a faithful port of the engine's own verifier.
- **Stdlib only** at the API surface; the only dependencies are the mirrored
  type libraries' `rs/xid` and `golang.org/x/text`.

## Install

```sh
go get github.com/avflowhq/sdk-go
```

Requires Go 1.26+.

## Submit and manage jobs

```go
package main

import (
	"context"
	"log"

	sdk "github.com/avflowhq/sdk-go"
)

func main() {
	client := sdk.NewClient("https://avflow-api.example.workers.dev", "avf_<appId>.<random>")

	// POST /v1/jobs is an upsert keyed by Name: re-submitting the same name
	// hot-updates the running pipeline instead of restarting it.
	job, err := client.SubmitJob(context.Background(), &sdk.Job{
		Name: "single-stream-relay",
		Sources: []sdk.Source{{
			Name: "room_src", Type: "livekit",
			Config: []byte(`{"serverUrl":"https://live.example.com","token":"eyJ..."}`),
		}},
		Sinks: []sdk.Sink{{
			Name: "rtmp_out", Type: "rtmp_push", Inputs: sdk.InputRefs{"room_src"},
			Config: []byte(`{"urls":["rtmp://live.example.com/app/key"]}`),
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("job %s is %s", job.ID, job.Status)
}
```

Against a self-hosted engine control server (no `/v1` prefix):

```go
client := sdk.NewClient("http://localhost:8080", "local", sdk.WithBasePath(""))
```

Non-2xx responses return `*sdk.ErrorResponse` with the HTTP status and the
control plane's machine-readable code:

```go
var apiErr *sdk.ErrorResponse
if errors.As(err, &apiErr) && apiErr.Code == "INVALID_JOB" { ... }
```

## Verify webhook events

avflow signs every events-ingest delivery with HMAC-SHA256 over the **raw**
request body:

```go
func webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if !sdk.VerifyHMACSHA256(os.Getenv("AVFLOW_HMAC_SECRET"), body, r.Header.Get(sdk.SignatureHeader)) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var ev sdk.LifecycleEvent // avflow.events.v1 envelope, typed
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "bad envelope", http.StatusBadRequest)
		return
	}
	// ev.Event == "job.started" | "job.failed" | "usage.tick" | ...
	w.WriteHeader(http.StatusOK)
}
```

An empty secret skips verification (dev mode), matching the engine.

## Where the types come from

`types/` and `event/` are generated mirrors — **do not edit them here**. They
are synced from the (private) avflow engine repository by its
`make sync-sdks` target, which also rewrites the engine-internal import path
to `github.com/avflowhq/sdk-go/types`. Changing the wire contract happens in
the engine, then a sync commit lands here.

## Development

```sh
make test    # go test ./... -race
make lint    # golangci-lint
```

Releases are plain git tags (`v*`); `go get` resolves them through the module
proxy — no artifacts to publish.

## License

MIT
