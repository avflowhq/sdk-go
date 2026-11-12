package avflow

import "testing"

func TestBitrateFromResolution_MatchesBridgeSink(t *testing.T) {
	tests := []struct {
		w, h    int
		codec   string
		wantBps int
	}{
		{1920, 1080, "h264", 5_000_000},
		{1280, 720, "h264", 2_500_000},
		{640, 480, "h264", 1_000_000},
		{320, 240, "h264", 500_000},
		{1280, 720, "h265", 1_500_000},
		{1280, 720, "av1", 1_500_000},
		{1280, 720, "vp9", 1_875_000},
		{0, 0, "h264", 2_500_000},
	}
	for _, tc := range tests {
		if got := BitrateFromResolution(tc.w, tc.h, tc.codec); got != tc.wantBps {
			t.Fatalf("%dx%d %s: got %d want %d", tc.w, tc.h, tc.codec, got, tc.wantBps)
		}
	}
}

func TestEffectiveVideoBitrateBps(t *testing.T) {
	explicit := &EncodingConfig{VideoBitrateBps: 3_000_000}
	if got := EffectiveVideoBitrateBps(explicit, 1920, 1080, "h264"); got != 3_000_000 {
		t.Fatalf("explicit = %d", got)
	}
	auto := &EncodingConfig{VideoCodec: "h264"}
	if got := EffectiveVideoBitrateBps(auto, 1280, 720, "h264"); got != 2_500_000 {
		t.Fatalf("auto = %d", got)
	}
}
