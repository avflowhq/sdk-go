package avflow

import (
	"math"
	"testing"
)

func TestSpeakerLayoutDefaults(t *testing.T) {
	s := &SpeakerLayout{}
	s.SetDefaults()
	if s.MainRatio != 0.75 {
		t.Fatalf("mainRatio = %v, want 0.75", s.MainRatio)
	}
	if s.ThumbnailPosition != "bottom" {
		t.Fatalf("thumbnailPosition = %q, want bottom", s.ThumbnailPosition)
	}
	if s.MaxThumbnails == nil || *s.MaxThumbnails != DefaultMaxThumbnails {
		t.Fatalf("maxThumbnails = %v, want %d", s.MaxThumbnails, DefaultMaxThumbnails)
	}
	if math.Abs(s.ThumbnailRatio-16.0/9.0) > 1e-9 {
		t.Fatalf("thumbnailRatio = %v, want 16/9", s.ThumbnailRatio)
	}
	if s.MainFit != "auto" {
		t.Fatalf("mainFit = %q, want auto", s.MainFit)
	}
	if s.ThumbnailFit != "cover" {
		t.Fatalf("thumbnailFit = %q, want cover", s.ThumbnailFit)
	}
	if s.ThumbnailAlign != "left" {
		t.Fatalf("thumbnailAlign = %q, want left (default for horizontal strip)", s.ThumbnailAlign)
	}
}

func TestSpeakerLayoutThumbnailAlignNormalization(t *testing.T) {
	cases := []struct {
		position string
		align    string
		want     string
	}{
		// Horizontal strips accept left/center/right; others fall back to left.
		{"bottom", "", "left"},
		{"bottom", "right", "right"},
		{"bottom", "center", "center"},
		{"bottom", "top", "left"},   // cross-axis value rejected
		{"top", "bottom", "left"},   // cross-axis value rejected
		{"bottom", "bogus", "left"}, // unknown value rejected
		// Vertical strips accept top/center/bottom; others fall back to top.
		{"left", "", "top"},
		{"right", "bottom", "bottom"},
		{"right", "center", "center"},
		{"left", "left", "top"},   // cross-axis value rejected
		{"right", "right", "top"}, // cross-axis value rejected
		{"left", "bogus", "top"},  // unknown value rejected
	}
	for _, c := range cases {
		s := &SpeakerLayout{ThumbnailPosition: c.position, ThumbnailAlign: c.align}
		s.SetDefaults()
		if s.ThumbnailAlign != c.want {
			t.Errorf("position=%q align=%q: got %q, want %q", c.position, c.align, s.ThumbnailAlign, c.want)
		}
	}
}

func TestLayoutCommonOverlayDefaults(t *testing.T) {
	c := &LayoutCommon{
		NameLabel: &NameLabel{Enabled: true, Position: "sideways", MaxWidthRatio: 5, FontSize: -1},
		Avatar:    &AvatarStyle{SizeRatio: -2},
	}
	c.SetDefaults()
	if c.NameLabel.Position != "bottom-left" {
		t.Fatalf("name label position = %q, want bottom-left", c.NameLabel.Position)
	}
	if c.NameLabel.MaxWidthRatio != 0.9 {
		t.Fatalf("name label maxWidthRatio = %v, want 0.9", c.NameLabel.MaxWidthRatio)
	}
	if c.NameLabel.FontSize != 14 {
		t.Fatalf("name label fontSize = %v, want 14", c.NameLabel.FontSize)
	}
	if c.Avatar.SizeRatio != 0.4 {
		t.Fatalf("avatar sizeRatio = %v, want 0.4", c.Avatar.SizeRatio)
	}

	// A valid corner is preserved.
	c2 := &LayoutCommon{NameLabel: &NameLabel{Position: "top-right", MaxWidthRatio: 0.5}}
	c2.SetDefaults()
	if c2.NameLabel.Position != "top-right" || c2.NameLabel.MaxWidthRatio != 0.5 {
		t.Fatalf("valid name label values not preserved: %+v", c2.NameLabel)
	}

	c3 := &LayoutCommon{ShowTiles: "with_video"}
	c3.SetDefaults()
	if c3.ShowTiles != ShowTilesWithVideo {
		t.Fatalf("showTiles = %q, want with_video", c3.ShowTiles)
	}
	cLegacy := &LayoutCommon{SlotVisibility: "has_video"}
	cLegacy.SetDefaults()
	if cLegacy.ShowTiles != ShowTilesWithVideo || cLegacy.SlotVisibility != "" {
		t.Fatalf("legacy slotVisibility/has_video → showTiles=%q slotVisibility=%q",
			cLegacy.ShowTiles, cLegacy.SlotVisibility)
	}
	c4 := &LayoutCommon{ShowTiles: "bogus"}
	c4.SetDefaults()
	if c4.ShowTiles != ShowTilesWithVideo {
		t.Fatalf("invalid showTiles = %q, want with_video", c4.ShowTiles)
	}
}

func TestSpeakerLayoutThumbnailRatioClamp(t *testing.T) {
	s := &SpeakerLayout{ThumbnailRatio: -1}
	s.SetDefaults()
	if math.Abs(s.ThumbnailRatio-16.0/9.0) > 1e-9 {
		t.Fatalf("non-positive ratio not defaulted: %v", s.ThumbnailRatio)
	}
}

func TestSpeakerLayoutMaxThumbnails(t *testing.T) {
	// Explicit 0 means "main speaker only, no thumbnails" and must survive
	// SetDefaults (the whole point of the pointer field).
	zero := 0
	s := &SpeakerLayout{MaxThumbnails: &zero}
	s.SetDefaults()
	if s.MaxThumbnails == nil || *s.MaxThumbnails != 0 {
		t.Fatalf("explicit 0 not preserved: %v", s.MaxThumbnails)
	}

	// Negative clamps up to 0.
	neg := -3
	s2 := &SpeakerLayout{MaxThumbnails: &neg}
	s2.SetDefaults()
	if s2.MaxThumbnails == nil || *s2.MaxThumbnails != 0 {
		t.Fatalf("negative not clamped to 0: %v", s2.MaxThumbnails)
	}

	// Over the cap clamps down.
	big := 999
	s3 := &SpeakerLayout{MaxThumbnails: &big}
	s3.SetDefaults()
	if s3.MaxThumbnails == nil || *s3.MaxThumbnails != MaxSpeakerThumbs {
		t.Fatalf("over-cap not clamped: %v", s3.MaxThumbnails)
	}
}

func TestGridLayoutFitDefaults(t *testing.T) {
	g := &GridLayout{}
	g.SetDefaults()
	if g.Fit != "cover" {
		t.Fatalf("grid fit = %q, want cover", g.Fit)
	}
	g2 := &GridLayout{Fit: "contain"}
	g2.SetDefaults()
	if g2.Fit != "contain" {
		t.Fatalf("grid fit = %q, want contain", g2.Fit)
	}
}

func TestNameLabelPaddingDefaults(t *testing.T) {
	n := &NameLabel{}
	n.SetDefaults()
	if n.Padding != 5 {
		t.Fatalf("padding = %v, want 5", n.Padding)
	}
}

func TestNameLabelFontSizeDefaults(t *testing.T) {
	n := &NameLabel{FontSize: 99}
	n.SetDefaults()
	if n.FontSize != 48 {
		t.Fatalf("fontSize = %v, want 48", n.FontSize)
	}
	n2 := &NameLabel{}
	n2.SetDefaults()
	if n2.FontSize != 14 {
		t.Fatalf("fontSize = %v, want 14", n2.FontSize)
	}
}

func TestNameLabelOpacityDefaults(t *testing.T) {
	n := &NameLabel{Opacity: -0.2}
	n.SetDefaults()
	if n.Opacity != 0 {
		t.Fatalf("opacity = %v, want 0", n.Opacity)
	}
	n2 := &NameLabel{Opacity: 1.5}
	n2.SetDefaults()
	if n2.Opacity != 1 {
		t.Fatalf("opacity = %v, want 1", n2.Opacity)
	}
}

func TestGridSpeakerLayoutGapDefaults(t *testing.T) {
	g := &GridLayout{Gap: -2}
	g.SetDefaults()
	if g.Gap != 0 {
		t.Fatalf("grid gap = %v, want 0", g.Gap)
	}

	sp := &SpeakerLayout{MainGap: -4, ThumbnailGap: -3}
	sp.SetDefaults()
	if sp.MainGap != 0 {
		t.Fatalf("speaker mainGap = %v, want 0", sp.MainGap)
	}
	if sp.ThumbnailGap != 0 {
		t.Fatalf("speaker thumbnailGap = %v, want 0", sp.ThumbnailGap)
	}
}
