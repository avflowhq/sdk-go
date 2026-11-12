package avflow

import "time"

// Process-level reconnect policy for network sources/sinks (RTMP/SRT/WHEP/WHIP
// push/pull, live protocol pulls). Not part of the job protocol — operators set
// it on the avflow process config (or env), and every transport uses the same
// defaults.
type ReconnectPolicy struct {
	// Enabled controls whether disconnect/IO failure retries. Default true.
	Enabled *bool `json:"enabled,omitempty"`
	// MaxRetries is the number of retries after the first failure. Default 10.
	// 0 means fail immediately after the first failed attempt (no retry).
	MaxRetries int `json:"maxRetries,omitempty"`
	// IntervalMs is the delay before the first retry. Default 200.
	// Later attempts double until MaxIntervalMs.
	IntervalMs int `json:"intervalMs,omitempty"`
	// MaxIntervalMs caps exponential backoff. Default 30000.
	MaxIntervalMs int `json:"maxIntervalMs,omitempty"`
}

const (
	DefaultReconnectMaxRetries    = 10
	DefaultReconnectIntervalMs    = 200
	DefaultReconnectMaxIntervalMs = 30_000
)

// DefaultReconnectPolicy returns the built-in policy (enabled, 10 retries,
// 200ms → 30s exponential backoff).
func DefaultReconnectPolicy() ReconnectPolicy {
	t := true
	return ReconnectPolicy{
		Enabled:       &t,
		MaxRetries:    DefaultReconnectMaxRetries,
		IntervalMs:    DefaultReconnectIntervalMs,
		MaxIntervalMs: DefaultReconnectMaxIntervalMs,
	}
}

// SetDefaults fills zero values and clamps to hard caps.
func (p *ReconnectPolicy) SetDefaults() {
	if p.Enabled == nil {
		t := true
		p.Enabled = &t
	}
	if p.MaxRetries == 0 {
		p.MaxRetries = DefaultReconnectMaxRetries
	}
	if p.MaxRetries < 0 {
		p.MaxRetries = 0
	}
	if p.MaxRetries > MaxReconnectMaxRetries {
		p.MaxRetries = MaxReconnectMaxRetries
	}
	if p.IntervalMs == 0 {
		p.IntervalMs = DefaultReconnectIntervalMs
	}
	if p.IntervalMs < 0 {
		p.IntervalMs = 0
	}
	if p.IntervalMs > MaxReconnectIntervalMs {
		p.IntervalMs = MaxReconnectIntervalMs
	}
	if p.MaxIntervalMs == 0 {
		p.MaxIntervalMs = DefaultReconnectMaxIntervalMs
	}
	if p.MaxIntervalMs < 0 {
		p.MaxIntervalMs = 0
	}
	if p.MaxIntervalMs > MaxReconnectIntervalMs {
		p.MaxIntervalMs = MaxReconnectIntervalMs
	}
	if p.MaxIntervalMs > 0 && p.IntervalMs > p.MaxIntervalMs {
		p.IntervalMs = p.MaxIntervalMs
	}
}

// On returns whether reconnect is enabled (nil Enabled → true).
func (p ReconnectPolicy) On() bool {
	return p.Enabled == nil || *p.Enabled
}

// Backoff returns the wait before retry attempt (1-indexed).
// attempt=1 → IntervalMs; then doubles up to MaxIntervalMs.
func (p ReconnectPolicy) Backoff(attempt int) time.Duration {
	initial := time.Duration(p.IntervalMs) * time.Millisecond
	if initial <= 0 {
		return 0
	}
	maxBackoff := time.Duration(p.MaxIntervalMs) * time.Millisecond
	if maxBackoff > 0 && initial > maxBackoff {
		return maxBackoff
	}
	if attempt < 1 {
		attempt = 1
	}
	b := initial
	for i := 1; i < attempt; i++ {
		if maxBackoff > 0 && b >= maxBackoff {
			return maxBackoff
		}
		next := b * 2
		if next < b {
			// overflow
			if maxBackoff > 0 {
				return maxBackoff
			}
			return b
		}
		b = next
	}
	if maxBackoff > 0 && b > maxBackoff {
		return maxBackoff
	}
	return b
}
