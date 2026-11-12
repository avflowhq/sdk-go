package avflow

import "testing"

func TestReconnectPolicyBackoff(t *testing.T) {
	p := ReconnectPolicy{IntervalMs: 200, MaxIntervalMs: 3000}
	want := []int{200, 400, 800, 1600, 3000, 3000}
	for i, ms := range want {
		got := p.Backoff(i + 1)
		if got.Milliseconds() != int64(ms) {
			t.Fatalf("attempt %d: got %v, want %dms", i+1, got, ms)
		}
	}
}

func TestReconnectPolicySetDefaults(t *testing.T) {
	var p ReconnectPolicy
	p.SetDefaults()
	if !p.On() || p.MaxRetries != DefaultReconnectMaxRetries {
		t.Fatalf("defaults: %+v", p)
	}
	if p.IntervalMs != DefaultReconnectIntervalMs || p.MaxIntervalMs != DefaultReconnectMaxIntervalMs {
		t.Fatalf("interval defaults: %+v", p)
	}
}
