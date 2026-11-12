package avflow

import "encoding/json"

type NodeType string

const (
	NodeTypeVideoMixer    NodeType = "video_mixer"
	NodeTypeAudioMixer    NodeType = "audio_mixer"
	NodeTypeAudioResample NodeType = "audio_resample"
	NodeTypeVideoEncoder  NodeType = "video_encoder"
	NodeTypeAudioEncoder  NodeType = "audio_encoder"
	NodeTypeASR           NodeType = "asr"
	NodeTypeTranslate     NodeType = "translate"
	NodeTypeVoiceAgent    NodeType = "voice_agent"
)

// Hard caps for node configs. SetDefaults clamps values to these.
const (
	MaxCanvasLongEdge  = 1920 // 1080p long edge (landscape width / portrait height)
	MaxCanvasShortEdge = 1080 // 1080p short edge (landscape height / portrait width)
	MaxCanvasFps       = 60
	MaxGridColumns     = 10
	MaxSpeakerThumbs   = 16

	DefaultCanvasFps     = 30
	DefaultMaxThumbnails = 6
	DefaultSampleRate    = 48_000
	DefaultResampleRate  = 16_000
	MaxAudioChannels     = 2
)

// Audio-level (volume) reporting caps and defaults. Levels quantizes RMS into
// discrete tiers (tier 0 = silence); see AudioLevelConfig.
const (
	DefaultAudioLevels     = 5
	MinAudioLevels         = 2
	MaxAudioLevels         = 10
	DefaultLevelIntervalMs = 1000
	MinLevelIntervalMs     = 200
	MaxLevelIntervalMs     = 2_000
	DefaultLevelFloorDb    = -50
)

// AllowedSampleRates lists canonical raw-audio output rates for mixer and
// resample nodes. Values outside this set are snapped to the nearest entry.
var AllowedSampleRates = [...]int{8_000, 16_000, 24_000, 32_000, 44_100, 48_000}

func snapSampleRate(sr int) int {
	best := AllowedSampleRates[0]
	bestDiff := sr - best
	if bestDiff < 0 {
		bestDiff = -bestDiff
	}
	for _, v := range AllowedSampleRates[1:] {
		d := sr - v
		if d < 0 {
			d = -d
		}
		if d < bestDiff {
			best, bestDiff = v, d
		}
	}
	return best
}

type Node struct {
	Name   string          `json:"name"`
	Type   NodeType        `json:"type"`
	Inputs InputRefs       `json:"inputs"`
	Config json.RawMessage `json:"config"`
}

func (n *Node) InputMediaTypes() []MediaType {
	switch n.Type {
	case NodeTypeVideoMixer:
		return []MediaType{MediaTypeVideo}
	case NodeTypeAudioMixer:
		return []MediaType{MediaTypeAudio}
	case NodeTypeAudioResample:
		return []MediaType{MediaTypeAudio}
	case NodeTypeVideoEncoder:
		return []MediaType{MediaTypeVideo}
	case NodeTypeAudioEncoder:
		return []MediaType{MediaTypeAudio}
	case NodeTypeASR:
		return []MediaType{MediaTypeAudio}
	case NodeTypeTranslate:
		return []MediaType{MediaTypeAudio}
	case NodeTypeVoiceAgent:
		return []MediaType{MediaTypeAudio}
	default:
		return []MediaType{MediaTypeVideo, MediaTypeAudio}
	}
}

func (n *Node) OutputMediaTypes() []MediaType {
	withData := func(mediaTypes ...MediaType) []MediaType {
		if n.emitsDataEvents() {
			return append(mediaTypes, MediaTypeData)
		}
		return mediaTypes
	}
	switch n.Type {
	case NodeTypeVideoMixer:
		return withData(MediaTypeVideo)
	case NodeTypeAudioMixer:
		return withData(MediaTypeAudio)
	case NodeTypeAudioResample:
		return []MediaType{MediaTypeAudio}
	case NodeTypeVideoEncoder:
		return []MediaType{MediaTypeVideo}
	case NodeTypeAudioEncoder:
		return []MediaType{MediaTypeAudio}
	case NodeTypeASR:
		return []MediaType{MediaTypeData}
	case NodeTypeTranslate:
		return []MediaType{MediaTypeAudio, MediaTypeData}
	case NodeTypeVoiceAgent:
		return []MediaType{MediaTypeAudio, MediaTypeData}
	default:
		return []MediaType{MediaTypeVideo, MediaTypeAudio}
	}
}

func (n *Node) outputsEncodedMedia() bool {
	switch n.Type {
	case NodeTypeVideoEncoder, NodeTypeAudioEncoder:
		return true
	default:
		return false
	}
}

func (n *Node) emitsDataEvents() bool {
	switch n.Type {
	case NodeTypeTranslate, NodeTypeVoiceAgent, NodeTypeASR:
		return true
	case NodeTypeVideoMixer:
		var cfg VideoMixerConfig
		return json.Unmarshal(n.Config, &cfg) == nil &&
			cfg.Layout.Common != nil &&
			cfg.Layout.Common.EmitLayoutInfo
	case NodeTypeAudioMixer:
		var cfg AudioMixerConfig
		return json.Unmarshal(n.Config, &cfg) == nil &&
			cfg.AudioLevels != nil &&
			cfg.AudioLevels.Enabled
	default:
		return false
	}
}

// NormalizeConfig parses Config by Type, calls its SetDefaults, and writes
// the result back so downstream consumers (including the browser-side
// mixer) see clamped values.
func (n *Node) NormalizeConfig() error {
	if len(n.Config) == 0 {
		return nil
	}
	var target interface {
		SetDefaults()
	}
	switch n.Type {
	case NodeTypeVideoMixer:
		target = &VideoMixerConfig{}
	case NodeTypeAudioMixer:
		target = &AudioMixerConfig{}
	case NodeTypeAudioResample:
		target = &AudioResampleConfig{}
	case NodeTypeVideoEncoder:
		target = &VideoEncoderConfig{}
	case NodeTypeAudioEncoder:
		target = &AudioEncoderConfig{}
	case NodeTypeVoiceAgent:
		target = &VoiceAgentNodeConfig{}
	case NodeTypeTranslate:
		target = &TranslateNodeConfig{}
	case NodeTypeASR:
		target = &ASRNodeConfig{}
	default:
		return nil
	}
	if err := json.Unmarshal(n.Config, target); err != nil {
		return err
	}
	target.SetDefaults()
	out, err := json.Marshal(target)
	if err != nil {
		return err
	}
	n.Config = out
	return nil
}

// VideoEncoderConfig converts raw video frames into codec access units.
type VideoEncoderConfig struct {
	Codec               string `json:"codec,omitempty"`
	BitrateBps          int    `json:"bitrateBps,omitempty"`
	KeyframeIntervalSec int    `json:"keyframeIntervalSec,omitempty"`
	EnableBFrames       bool   `json:"enableBFrames,omitempty"`
}

func (c *VideoEncoderConfig) SetDefaults() {
	c.Codec = normalizeSinkCodec(c.Codec)
	if c.BitrateBps < 0 {
		c.BitrateBps = 0
	}
	if c.BitrateBps > MaxVideoBitrateBps {
		c.BitrateBps = MaxVideoBitrateBps
	}
	if c.KeyframeIntervalSec <= 0 {
		c.KeyframeIntervalSec = DefaultKeyframeIntervalSec
	}
	if c.KeyframeIntervalSec > MaxKeyframeIntervalSec {
		c.KeyframeIntervalSec = MaxKeyframeIntervalSec
	}
}

// AudioEncoderConfig converts raw PCM frames into codec access units.
type AudioEncoderConfig struct {
	Codec      string `json:"codec,omitempty"`
	Profile    string `json:"profile,omitempty"`
	BitrateBps int    `json:"bitrateBps,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

func (c *AudioEncoderConfig) SetDefaults() {
	c.Codec = normalizeSinkCodec(c.Codec)
	if c.BitrateBps < 0 {
		c.BitrateBps = 0
	}
	if c.BitrateBps > MaxAudioBitrateBps {
		c.BitrateBps = MaxAudioBitrateBps
	}
	c.SampleRate = SnapEncodingAudioSampleRate(c.SampleRate)
	if c.Channels <= 0 {
		c.Channels = 2
	}
	if c.Channels > MaxAudioChannels {
		c.Channels = MaxAudioChannels
	}
}

// Video mixer

type VideoMixerConfig struct {
	Canvas Canvas `json:"canvas"`
	Layout Layout `json:"layout"`
}

func (c *VideoMixerConfig) SetDefaults() {
	c.Canvas.SetDefaults()
	c.Layout.SetDefaults()
}

type Canvas struct {
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	Fps              int    `json:"fps,omitempty"`
	BackgroundColor  string `json:"backgroundColor,omitempty"`
	CoordinateSystem string `json:"coordinateSystem,omitempty"`
}

func (c *Canvas) SetDefaults() {
	if c.Width < 0 {
		c.Width = 0
	}
	if c.Height < 0 {
		c.Height = 0
	}
	clampCanvasSize(&c.Width, &c.Height)
	if c.Fps <= 0 {
		c.Fps = DefaultCanvasFps
	}
	if c.Fps > MaxCanvasFps {
		c.Fps = MaxCanvasFps
	}
	if c.BackgroundColor == "" {
		c.BackgroundColor = "#000000"
	}
	switch c.CoordinateSystem {
	case "pixel", "ratio":
	default:
		c.CoordinateSystem = "pixel"
	}
}

// clampCanvasSize enforces the 1080p canvas cap for landscape and portrait.
func clampCanvasSize(w, h *int) {
	if *w <= 0 && *h <= 0 {
		return
	}
	if *w <= 0 {
		if *h > MaxCanvasLongEdge {
			*h = MaxCanvasLongEdge
		}
		return
	}
	if *h <= 0 {
		if *w > MaxCanvasLongEdge {
			*w = MaxCanvasLongEdge
		}
		return
	}
	long, short := w, h
	if *short > *long {
		long, short = short, long
	}
	if *long > MaxCanvasLongEdge {
		*long = MaxCanvasLongEdge
	}
	if *short > MaxCanvasShortEdge {
		*short = MaxCanvasShortEdge
	}
}

// LayoutMode selects how video_mixer places inputs on the canvas. The pipeline
// mixer (web/src/worker/mixer.ts) implements these modes; unknown or empty
// values fall back to grid at render time.
type LayoutMode string

const (
	// LayoutModeGrid tiles every visible input in an equal-size grid (GridLayout).
	// Default when mode is omitted.
	LayoutModeGrid LayoutMode = "grid"
	// LayoutModeSpeaker shows one large main tile plus a thumbnail strip; the
	// main slot follows speaker.mainPriority (e.g. active_speaker, screen_share).
	LayoutModeSpeaker LayoutMode = "speaker"
	// LayoutModeCustom places inputs into explicit regions (ratio or pixel
	// coords). Use for PiP, overlays, and branded scenes (CustomLayout.regions).
	LayoutModeCustom LayoutMode = "custom"
)

// Layout selects how video_mixer tiles inputs. Set mode to one of grid,
// speaker, or custom; the matching sub-config (grid, speaker, …) holds
// mode-specific geometry. Unknown mode values fall back to grid at render time.
// For a single full-canvas tile, use speaker with maxThumbnails: 0.
type Layout struct {
	// Mode is the layout algorithm: grid | speaker | custom.
	Mode LayoutMode `json:"mode"`
	// Common holds visual options shared across modes (border radius, avatar, …).
	Common *LayoutCommon `json:"common,omitempty"`
	// Grid configures equal-size tiling when mode is grid.
	Grid *GridLayout `json:"grid,omitempty"`
	// Speaker configures main + thumbnail strip when mode is speaker.
	Speaker *SpeakerLayout `json:"speaker,omitempty"`
	// Custom configures explicit regions when mode is custom.
	Custom *CustomLayout `json:"custom,omitempty"`
}

func (l *Layout) SetDefaults() {
	switch l.Mode {
	case LayoutModeGrid, LayoutModeSpeaker, LayoutModeCustom:
	default:
		l.Mode = LayoutModeGrid
	}
	if l.Common != nil {
		l.Common.SetDefaults()
	}
	if l.Grid != nil {
		l.Grid.SetDefaults()
	}
	if l.Speaker != nil {
		l.Speaker.SetDefaults()
	}
	if l.Custom != nil {
		l.Custom.SetDefaults()
	}
}

// ShowTiles controls which input slots participate in layout (grid / speaker).
type ShowTiles string

const (
	// ShowTilesAll: every slot including join placeholders; inactive / no-video
	// tiles show avatar. Any camera or screen-share track for an identity
	// suppresses that identity's join placeholder.
	ShowTilesAll ShowTiles = "all"
	// ShowTilesWithVideo (default): only slots with an active video frame
	// (join placeholders are filtered out).
	ShowTilesWithVideo ShowTiles = "with_video"
	// ShowTilesActive: slots with active video or current audio activity
	// (a speaking join placeholder can still appear).
	ShowTilesActive ShowTiles = "active"
)

// LayoutCommon holds visual options shared across layout modes (grid, speaker,
// custom). Geometry (gap, fit, …) lives on each mode's own config.
//
// Custom-mode usage: ShowTiles is ignored (each region always resolves
// its source). BorderRadius is only the fallback when a region omits
// style.borderRadius. Avatar applies only when a region has no placeholder.
// NameLabel still draws on every region.
type LayoutCommon struct {
	// EmitLayoutInfo enables avflow.videoMixerLayout metadata reporting from the video mixer.
	// Streaming sinks re-send the last layout on every video keyframe.
	EmitLayoutInfo bool `json:"emitLayoutInfo,omitempty"`
	// BorderRadius rounds each slot's video corners (custom: fallback for
	// region.style.borderRadius when unset/0). CSS-like: (0,1] → horizontal
	// ratio×width, vertical ratio×height (ellipse on non-square; 0.5 = full
	// round; 1 clamps the same); >1 = circular pixel radius.
	BorderRadius float64 `json:"borderRadius,omitempty"`
	// ShowTiles filters which slots are laid out (see ShowTiles*).
	// Ignored in custom mode — regions always resolve; no-video slots still
	// occupy the region (avatar / placeholder).
	ShowTiles ShowTiles `json:"showTiles,omitempty"`
	// Deprecated: use ShowTiles. Accepted on input for older jobs.
	SlotVisibility ShowTiles `json:"slotVisibility,omitempty"`
	// Avatar styles the per-participant no-video fill (image URL from slot meta,
	// full name fallback). Omitted fields use defaults.
	// Custom: only when the region has no placeholder.
	Avatar *AvatarStyle `json:"avatar,omitempty"`
	// NameLabel draws an always-on participant name pill over each tile
	// (including custom regions).
	NameLabel *NameLabel `json:"nameLabel,omitempty"`
}

func (c *LayoutCommon) SetDefaults() {
	if c.BorderRadius < 0 {
		c.BorderRadius = 0
	}
	if c.ShowTiles == "" && c.SlotVisibility != "" {
		c.ShowTiles = c.SlotVisibility
	}
	c.SlotVisibility = ""
	switch c.ShowTiles {
	case "has_video": // legacy alias
		c.ShowTiles = ShowTilesWithVideo
	case ShowTilesAll, ShowTilesWithVideo, ShowTilesActive:
	default:
		c.ShowTiles = ShowTilesWithVideo
	}
	if c.NameLabel != nil {
		c.NameLabel.SetDefaults()
	}
	if c.Avatar != nil {
		c.Avatar.SetDefaults()
	}
}

// NameLabel configures the on-canvas participant name pill.
type NameLabel struct {
	Enabled bool `json:"enabled,omitempty"`
	// Position is the tile corner: bottom-left (default) / bottom-right /
	// top-left / top-right.
	Position string `json:"position,omitempty"`
	// FontSize in px. Defaults to 14; max 48.
	FontSize        float64 `json:"fontSize,omitempty"`
	FontFamily      string  `json:"fontFamily,omitempty"`
	Color           string  `json:"color,omitempty"`
	BackgroundColor string  `json:"backgroundColor,omitempty"`
	// Opacity is the pill background alpha (0..1). Defaults to 0.
	Opacity float64 `json:"opacity,omitempty"`
	// Padding is the pill vertical inner padding in px. Defaults to 5; horizontal
	// padding is derived as 1.5× in the renderer.
	Padding float64 `json:"padding,omitempty"`
	// BorderRadius of the name pill. CSS-like: (0,1] = ratio×pill size axes; >1 = px.
	BorderRadius float64 `json:"borderRadius,omitempty"`
	// MaxWidthRatio caps the label width as a fraction of the tile width (0..1).
	MaxWidthRatio float64 `json:"maxWidthRatio,omitempty"`
}

func (n *NameLabel) SetDefaults() {
	switch n.Position {
	case "bottom-left", "bottom-right", "top-left", "top-right":
	default:
		n.Position = "bottom-left"
	}
	if n.FontSize <= 0 {
		n.FontSize = 14
	}
	if n.FontSize > 48 {
		n.FontSize = 48
	}
	if n.MaxWidthRatio <= 0 || n.MaxWidthRatio > 1 {
		n.MaxWidthRatio = 0.9
	}
	if n.Padding <= 0 {
		n.Padding = 5
	}
	if n.BorderRadius < 0 {
		n.BorderRadius = 0
	}
	if n.Opacity < 0 {
		n.Opacity = 0
	}
	if n.Opacity > 1 {
		n.Opacity = 1
	}
}

// AvatarStyle configures the per-participant no-video avatar circle.
type AvatarStyle struct {
	// SizeRatio is the circle diameter as a fraction of min(tileW, tileH).
	SizeRatio float64 `json:"sizeRatio,omitempty"`
	TextColor string  `json:"textColor,omitempty"`
	// BackgroundColor fixes the circle color; empty derives a color per identity.
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

func (a *AvatarStyle) SetDefaults() {
	if a.SizeRatio <= 0 || a.SizeRatio > 1 {
		a.SizeRatio = 0.4
	}
}

// Placeholder fills a custom region when no matching video slot is active.
type Placeholder struct {
	Image *PlaceholderImage `json:"image,omitempty"`
	Text  *PlaceholderText  `json:"text,omitempty"`
}

type PlaceholderImage struct {
	URL string `json:"url,omitempty"`
	// Fit scales the image: cover | contain | fill. Empty leaves renderer default.
	Fit string `json:"fit,omitempty"`
	// BorderRadius of the image. CSS-like: (0,1] = ratio×image axes; >1 = px.
	BorderRadius float64 `json:"borderRadius,omitempty"`
	Width        float64 `json:"width,omitempty"`
	Height       float64 `json:"height,omitempty"`
}

func (p *PlaceholderImage) SetDefaults() {
	if p.BorderRadius < 0 {
		p.BorderRadius = 0
	}
	if p.Width < 0 {
		p.Width = 0
	}
	if p.Height < 0 {
		p.Height = 0
	}
	p.Fit = normalizeVideoFit(p.Fit, "")
}

type PlaceholderText struct {
	Content string `json:"content,omitempty"`
	// Position within the region: top | bottom | center (default center).
	Position   string `json:"position,omitempty"`
	FontSize   int    `json:"fontSize,omitempty"`
	FontFamily string `json:"fontFamily,omitempty"`
	Color      string `json:"color,omitempty"`
}

func (t *PlaceholderText) SetDefaults() {
	if t.FontSize < 0 {
		t.FontSize = 0
	}
	switch t.Position {
	case "", "top", "bottom", "center":
	default:
		t.Position = "center"
	}
}

// normalizeVideoFit clamps layout fit strings used by grid/custom/placeholder.
// Empty stays empty when fallback is "" (optional fields); unknown values fall
// back to fallback (typically "cover").
func normalizeVideoFit(fit, fallback string) string {
	switch fit {
	case "cover", "contain", "fill":
		return fit
	case "stretch":
		return "fill"
	case "":
		return fallback
	default:
		return fallback
	}
}

// normalizeSpeakerFit is like normalizeVideoFit but also allows "auto"
// (per-track cover/contain choice for screen share vs camera).
func normalizeSpeakerFit(fit, fallback string) string {
	switch fit {
	case "auto", "cover", "contain", "fill":
		return fit
	case "stretch":
		return "fill"
	case "":
		return fallback
	default:
		return fallback
	}
}

type GridLayout struct {
	// MaxColumns caps grid columns (1..MaxGridColumns). 0 = auto (√n rounded up).
	MaxColumns int `json:"maxColumns,omitempty"`
	// Gap is the pixel spacing between grid cells (≥0, default 0).
	Gap float64 `json:"gap,omitempty"`
	// Fit scales each tile's video: cover | contain | fill (default cover).
	Fit string `json:"fit,omitempty"`
}

func (g *GridLayout) SetDefaults() {
	if g.MaxColumns < 0 {
		g.MaxColumns = 0
	}
	if g.MaxColumns > MaxGridColumns {
		g.MaxColumns = MaxGridColumns
	}
	if g.Gap < 0 {
		g.Gap = 0
	}
	g.Fit = normalizeVideoFit(g.Fit, "cover")
}

// SpeakerLayout configures the main tile plus thumbnail strip.
type SpeakerLayout struct {
	// MainPriority selects the main tile. Each entry is a kind string
	// (screen_share | active_speaker) or a MatchSource object; first match wins.
	MainPriority []MainPriorityRule `json:"mainPriority,omitempty"`
	// MainRatio is the main tile's share of the canvas along the strip axis
	// (0..1, default 0.75). The thumbnail strip gets the remainder.
	MainRatio float64 `json:"mainRatio,omitempty"`
	// MainGap is the pixel gap between the main tile and the thumbnail strip
	// (≥0, default 0).
	MainGap float64 `json:"mainGap,omitempty"`
	// ThumbnailGap is the pixel gap between adjacent thumbnails in the strip
	// (≥0, default 0).
	ThumbnailGap float64 `json:"thumbnailGap,omitempty"`
	// MainFit scales the main tile: auto | cover | contain | fill (default auto).
	// auto picks per track — screen_share uses contain, others cover.
	MainFit string `json:"mainFit,omitempty"`
	// MaxThumbnails caps the thumbnail strip. A pointer so an explicit 0 (main
	// speaker only, no thumbnails) is distinguishable from "unset" (defaults to
	// DefaultMaxThumbnails). nil → default; 0 → no thumbnails; max MaxSpeakerThumbs.
	MaxThumbnails *int `json:"maxThumbnails,omitempty"`
	// ThumbnailPosition places the strip: top | bottom (default) | left | right.
	ThumbnailPosition string `json:"thumbnailPosition,omitempty"`
	// ThumbnailRatio is each thumbnail's width/height aspect (default 16/9).
	ThumbnailRatio float64 `json:"thumbnailRatio,omitempty"`
	// ThumbnailAlign packs the thumbnail group within the strip. Horizontal strips
	// (top/bottom): left | center | right (default left). Vertical strips
	// (left/right): top | center | bottom (default top).
	ThumbnailAlign string `json:"thumbnailAlign,omitempty"`
	// ThumbnailFit scales each thumbnail: auto | cover | contain | fill
	// (default cover).
	ThumbnailFit string `json:"thumbnailFit,omitempty"`
}

func (s *SpeakerLayout) SetDefaults() {
	if s.MainRatio == 0 {
		s.MainRatio = 0.75
	}
	if s.MainRatio < 0 {
		s.MainRatio = 0
	}
	if s.MainRatio > 1 {
		s.MainRatio = 1
	}
	if s.MainGap < 0 {
		s.MainGap = 0
	}
	if s.ThumbnailGap < 0 {
		s.ThumbnailGap = 0
	}
	switch s.ThumbnailPosition {
	case "top", "bottom", "left", "right":
	default:
		s.ThumbnailPosition = "bottom"
	}
	switch {
	case s.MaxThumbnails == nil:
		n := DefaultMaxThumbnails
		s.MaxThumbnails = &n
	case *s.MaxThumbnails < 0:
		*s.MaxThumbnails = 0
	case *s.MaxThumbnails > MaxSpeakerThumbs:
		*s.MaxThumbnails = MaxSpeakerThumbs
	}
	if s.ThumbnailRatio <= 0 {
		s.ThumbnailRatio = 16.0 / 9.0
	}
	s.ThumbnailAlign = normalizeThumbAlign(s.ThumbnailAlign, s.ThumbnailPosition)
	s.MainFit = normalizeSpeakerFit(s.MainFit, "auto")
	s.ThumbnailFit = normalizeSpeakerFit(s.ThumbnailFit, "cover")
}

// normalizeThumbAlign clamps the thumbnail alignment to the values valid for
// the strip orientation. Horizontal strips (top/bottom) accept left/center/
// right (default left); vertical strips (left/right) accept top/center/bottom
// (default top). Cross-axis or unknown values fall back to the default.
func normalizeThumbAlign(align, position string) string {
	if position == "left" || position == "right" {
		switch align {
		case "top", "center", "bottom":
			return align
		default:
			return "top"
		}
	}
	switch align {
	case "left", "center", "right":
		return align
	default:
		return "left"
	}
}

// CustomLayout places inputs into explicit canvas regions.
type CustomLayout struct {
	Regions []Region `json:"regions"`
}

func (c *CustomLayout) SetDefaults() {
	for i := range c.Regions {
		r := &c.Regions[i]
		if r.Width < 0 {
			r.Width = 0
		}
		if r.Height < 0 {
			r.Height = 0
		}
		r.Fit = normalizeVideoFit(r.Fit, "")
		if r.Style != nil {
			r.Style.SetDefaults()
		}
		if r.Placeholder != nil {
			if r.Placeholder.Image != nil {
				r.Placeholder.Image.SetDefaults()
			}
			if r.Placeholder.Text != nil {
				r.Placeholder.Text.SetDefaults()
			}
		}
	}
}

// Region is one composited area in custom layout. Width/height/offset use
// canvas pixels when coordinateSystem is pixel, or fractions of canvas size
// when coordinateSystem is ratio.
type Region struct {
	Name string `json:"name"`
	// Source selects which input slot fills the region (MatchSource rules).
	Source  *MatchSource `json:"source,omitempty"`
	Width   float64      `json:"width"`
	Height  float64      `json:"height"`
	OffsetX float64      `json:"offsetX,omitempty"`
	OffsetY float64      `json:"offsetY,omitempty"`
	// ZIndex controls draw order (higher on top; default 0).
	ZIndex int `json:"zIndex,omitempty"`
	// Fit scales the region video: cover | contain | fill. Empty leaves default.
	Fit         string       `json:"fit,omitempty"`
	Placeholder *Placeholder `json:"placeholder,omitempty"`
	Style       *RegionStyle `json:"style,omitempty"`
}

// MatchSource picks an input slot. All set fields must match; first matching
// slot in sort order wins. Index is 0-based within the filtered slot list.
type MatchSource struct {
	SourceName  string `json:"sourceName,omitempty"`
	Identity    string `json:"identity,omitempty"`
	TrackSid    string `json:"trackSid,omitempty"`
	TrackSource string `json:"trackSource,omitempty"`
	TrackName   string `json:"trackName,omitempty"`
	Index       *int   `json:"index,omitempty"`
}

type RegionStyle struct {
	BackgroundColor string `json:"backgroundColor,omitempty"`
	BorderColor     string `json:"borderColor,omitempty"`
	// BorderWidth is the region border thickness in px (≥0).
	BorderWidth float64 `json:"borderWidth,omitempty"`
	// BorderRadius: CSS-like (0,1] = ratio×region width/height; >1 = pixels.
	BorderRadius float64 `json:"borderRadius,omitempty"`
	// Opacity is the region background alpha (0..1).
	Opacity float64 `json:"opacity,omitempty"`
}

func (s *RegionStyle) SetDefaults() {
	if s.BorderRadius < 0 {
		s.BorderRadius = 0
	}
	if s.BorderWidth < 0 {
		s.BorderWidth = 0
	}
	if s.Opacity < 0 {
		s.Opacity = 0
	}
	if s.Opacity > 1 {
		s.Opacity = 1
	}
}

// Audio resample

// AudioResampleConfig controls per-track sample-rate and channel conversion.
// The node is n:n: every input audio track produces one output track with the
// same metadata. Channels 0 preserves the input channel count.
type AudioResampleConfig struct {
	SampleRate int `json:"sampleRate,omitempty"`
	Channels   int `json:"channels,omitempty"`
}

func (c *AudioResampleConfig) SetDefaults() {
	if c.SampleRate <= 0 {
		c.SampleRate = DefaultResampleRate
	} else {
		c.SampleRate = snapSampleRate(c.SampleRate)
	}
	if c.Channels < 0 {
		c.Channels = 1
	}
	if c.Channels > MaxAudioChannels {
		c.Channels = MaxAudioChannels
	}
}

// Audio mixer

type AudioMixerConfig struct {
	SampleRate  int               `json:"sampleRate,omitempty"`
	Channels    int               `json:"channels,omitempty"`
	AudioLevels *AudioLevelConfig `json:"audioLevels,omitempty"`
}

func (c *AudioMixerConfig) SetDefaults() {
	if c.SampleRate <= 0 {
		c.SampleRate = DefaultSampleRate
	} else {
		c.SampleRate = snapSampleRate(c.SampleRate)
	}
	if c.Channels == 0 {
		c.Channels = 2
	}
	if c.Channels < 1 {
		c.Channels = 1
	}
	if c.Channels > MaxAudioChannels {
		c.Channels = MaxAudioChannels
	}
	if c.AudioLevels != nil {
		c.AudioLevels.SetDefaults()
	}
}

// AudioLevelConfig controls optional per-identity volume reporting from the
// audio mixer. Disabled by default. When enabled the mixer emits a quantized
// integer tier per speaking identity, which sinks forward to clients over
// their transport's metadata path (RTC data channel or in-band video metadata).
type AudioLevelConfig struct {
	Enabled bool `json:"enabled"`
	// IntervalMs is the time interval in ms between audio-level updates.
	// Defaults to 1000ms.
	IntervalMs int `json:"intervalMs,omitempty"`
	// Levels is the number of discrete tiers; output is 0..Levels-1 (0=silence).
	Levels int `json:"levels,omitempty"`
	// FloorDb is the dBFS threshold below which audio is treated as silence
	// (tier 0). Values are usually negative; positive inputs are normalized to
	// negative. Less-negative values (e.g. -30) require louder audio than
	// more-negative values (e.g. -70).
	FloorDb int `json:"floorDb,omitempty"`
}

func (c *AudioLevelConfig) SetDefaults() {
	if c.Levels == 0 {
		c.Levels = DefaultAudioLevels
	}
	if c.Levels < MinAudioLevels {
		c.Levels = MinAudioLevels
	}
	if c.Levels > MaxAudioLevels {
		c.Levels = MaxAudioLevels
	}
	if c.IntervalMs == 0 {
		c.IntervalMs = DefaultLevelIntervalMs
	}
	if c.IntervalMs < MinLevelIntervalMs {
		c.IntervalMs = MinLevelIntervalMs
	}
	if c.IntervalMs > MaxLevelIntervalMs {
		c.IntervalMs = MaxLevelIntervalMs
	}
	if c.FloorDb == 0 {
		c.FloorDb = DefaultLevelFloorDb
	}
	if c.FloorDb > 0 {
		c.FloorDb = -c.FloorDb
	}
}
