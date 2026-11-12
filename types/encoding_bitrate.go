package avflow

import (
	"math"
	"strings"
)

// BitrateFromResolution mirrors web/src/sink/BridgeSink.ts when videoBitrateBps
// is unset (0).
func BitrateFromResolution(width, height int, codec string) int {
	if width <= 0 || height <= 0 {
		return 2_500_000
	}
	pixels := width * height
	var base int
	switch {
	case pixels >= 1920*1080:
		base = 5_000_000
	case pixels >= 1280*720:
		base = 2_500_000
	case pixels >= 640*480:
		base = 1_000_000
	default:
		base = 500_000
	}
	switch strings.ToLower(codec) {
	case "h265", "hevc", "av1":
		return int(math.Round(float64(base) * 0.6))
	case "vp9":
		return int(math.Round(float64(base) * 0.75))
	default:
		return base
	}
}

// EffectiveVideoBitrateBps returns encoding.videoBitrateBps when set, otherwise
// BitrateFromResolution for the given canvas and codec.
func EffectiveVideoBitrateBps(enc *EncodingConfig, width, height int, codec string) int {
	if enc != nil {
		enc.SetDefaults()
		if enc.VideoBitrateBps > 0 {
			return enc.VideoBitrateBps
		}
		if enc.VideoCodec != "" {
			codec = enc.VideoCodec
		}
	}
	return BitrateFromResolution(width, height, codec)
}
