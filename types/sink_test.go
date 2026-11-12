package avflow

import (
	"encoding/json"
	"testing"
)

func TestSink_NormalizeConfig_ClampsEncodingCaps(t *testing.T) {
	cases := []struct {
		name string
		typ  SinkType
		raw  string
	}{
		{
			name: "axonkit",
			typ:  SinkTypeAxonkit,
			raw: `{
				"serverUrl": "wss://x", "token": "t", "roomName": "r",
				"encoding": {
					"videoBitrateBps": 999000000000,
					"audioBitrateBps": 10000000,
					"keyframeIntervalSec": 9999
				}
			}`,
		},
		{
			name: "rtmp_push",
			typ:  SinkTypeRTMPPush,
			raw: `{
				"urls": ["rtmp://x/y"],
				"encoding": {
					"videoBitrateBps": -1,
					"audioBitrateBps": 999999999,
					"keyframeIntervalSec": 60
				}
			}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sk := Sink{Type: tc.typ, Config: json.RawMessage(tc.raw)}
			if err := sk.NormalizeConfig(); err != nil {
				t.Fatalf("NormalizeConfig: %v", err)
			}
			var probe struct {
				Encoding EncodingConfig `json:"encoding"`
			}
			if err := json.Unmarshal(sk.Config, &probe); err != nil {
				t.Fatalf("unmarshal normalized: %v", err)
			}
			enc := probe.Encoding
			if enc.VideoBitrateBps < 0 || enc.VideoBitrateBps > MaxVideoBitrateBps {
				t.Errorf("videoBitrateBps=%d out of [0, %d]", enc.VideoBitrateBps, MaxVideoBitrateBps)
			}
			if enc.AudioBitrateBps < 0 || enc.AudioBitrateBps > MaxAudioBitrateBps {
				t.Errorf("audioBitrateBps=%d out of [0, %d]", enc.AudioBitrateBps, MaxAudioBitrateBps)
			}
			if enc.KeyframeIntervalSec < 0 || enc.KeyframeIntervalSec > MaxKeyframeIntervalSec {
				t.Errorf("keyframeIntervalSec=%d out of [0, %d]", enc.KeyframeIntervalSec, MaxKeyframeIntervalSec)
			}
		})
	}
}

func TestSink_NormalizeConfig_ImageClamps(t *testing.T) {
	sk := Sink{
		Type:   SinkTypeImage,
		Config: json.RawMessage(`{"format":"jpeg","intervalSec":99999,"quality":200,"width":99999,"height":99999}`),
	}
	if err := sk.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got ImageSinkConfig
	if err := json.Unmarshal(sk.Config, &got); err != nil {
		t.Fatalf("unmarshal normalized: %v", err)
	}
	if got.IntervalSec != MaxImageIntervalSec {
		t.Errorf("IntervalSec=%d, want %d", got.IntervalSec, MaxImageIntervalSec)
	}
	if got.Quality != 100 {
		t.Errorf("Quality=%d, want 100", got.Quality)
	}
	if got.Width != MaxImageWidth || got.Height != MaxImageHeight {
		t.Errorf("Width=%d Height=%d, want %d %d", got.Width, got.Height, MaxImageWidth, MaxImageHeight)
	}
}

func TestSink_NormalizeConfig_SegmentCaptionTextSplitDefaults(t *testing.T) {
	sk := Sink{
		Type:   SinkTypeSegment,
		Config: json.RawMessage(`{"storageType":"local","caption":{"textSplit":{"enabled":true}}}`),
	}
	if err := sk.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got SegmentSinkConfig
	if err := json.Unmarshal(sk.Config, &got); err != nil {
		t.Fatalf("unmarshal normalized: %v", err)
	}
	if got.Caption == nil {
		t.Fatal("caption missing")
	}
	if got.Caption.TextSplit == nil {
		t.Fatal("caption.textSplit missing")
	}
	if got.Caption.TextSplit.Length != DefaultCaptionTextSplitLength ||
		got.Caption.TextSplit.Lines != DefaultCaptionTextSplitLines {
		t.Fatalf("got length=%d lines=%d", got.Caption.TextSplit.Length, got.Caption.TextSplit.Lines)
	}

	got.Caption.TextSplit.Length = 80
	got.Caption.TextSplit.Lines = 3
	got.SetDefaults()

	if got.Caption.TextSplit.Length != 80 || got.Caption.TextSplit.Lines != 3 {
		t.Fatalf("explicit values overwritten: length=%d lines=%d", got.Caption.TextSplit.Length, got.Caption.TextSplit.Lines)
	}
}

func TestSink_NormalizeConfig_EmptyConfigIsNoop(t *testing.T) {
	sk := Sink{Type: SinkTypeAxonkit}
	if err := sk.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	if len(sk.Config) != 0 {
		t.Errorf("empty config got mutated: %s", sk.Config)
	}
}

func TestSink_NormalizeConfig_EncodingCodecDefaults(t *testing.T) {
	t.Run("mux fills h264+aac when encoding is omitted", func(t *testing.T) {
		sk := Sink{Type: SinkTypeRTMPPush, Config: json.RawMessage(`{"urls":["rtmp://x"]}`)}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got RTMPPushConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding == nil || got.Encoding.VideoCodec != "h264" || got.Encoding.AudioCodec != "aac" {
			t.Fatalf("encoding = %+v, want h264+aac", got.Encoding)
		}
	})
	t.Run("mux fills omitted codecs even when encoding is present", func(t *testing.T) {
		sk := Sink{
			Type:   SinkTypeSegment,
			Config: json.RawMessage(`{"storageType":"local","encoding":{"videoCodec":"h264"}}`),
		}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got SegmentSinkConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding == nil || got.Encoding.VideoCodec != "h264" || got.Encoding.AudioCodec != "aac" {
			t.Fatalf("encoding = %+v, want h264+aac", got.Encoding)
		}
	})
	t.Run("whip defaults empty codecs to h264+opus", func(t *testing.T) {
		sk := Sink{Type: SinkTypeWHIP, Config: json.RawMessage(`{"url":"https://whip.example/whip"}`)}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got WHIPPushConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding == nil || got.Encoding.VideoCodec != "h264" || got.Encoding.AudioCodec != "opus" {
			t.Fatalf("encoding = %+v, want h264+opus", got.Encoding)
		}
	})
	t.Run("whip keeps an explicit video codec", func(t *testing.T) {
		sk := Sink{
			Type:   SinkTypeWHIP,
			Config: json.RawMessage(`{"url":"https://whip.example/whip","encoding":{"videoCodec":"vp8"}}`),
		}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got WHIPPushConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding.VideoCodec != "vp8" || got.Encoding.AudioCodec != "opus" {
			t.Fatalf("encoding = %+v, want vp8+opus", got.Encoding)
		}
	})
	t.Run("rtc room defaults empty codecs to vp8+opus", func(t *testing.T) {
		sk := Sink{
			Type:   SinkTypeLivekit,
			Config: json.RawMessage(`{"serverUrl":"wss://x","token":"t"}`),
		}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got LivekitSinkConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding == nil || got.Encoding.VideoCodec != "vp8" || got.Encoding.AudioCodec != "opus" {
			t.Fatalf("encoding = %+v, want vp8+opus", got.Encoding)
		}
	})
	t.Run("rtc room keeps an explicit video codec", func(t *testing.T) {
		sk := Sink{
			Type:   SinkTypeJitsi,
			Config: json.RawMessage(`{"domain":"meet.example","roomName":"r","encoding":{"videoCodec":"h264"}}`),
		}
		if err := sk.NormalizeConfig(); err != nil {
			t.Fatal(err)
		}
		var got JitsiSinkConfig
		if err := json.Unmarshal(sk.Config, &got); err != nil {
			t.Fatal(err)
		}
		if got.Encoding.VideoCodec != "h264" || got.Encoding.AudioCodec != "opus" {
			t.Fatalf("encoding = %+v, want h264+opus", got.Encoding)
		}
	})
}

func TestJob_Normalize_PropagatesSinkErrors(t *testing.T) {
	job := &Job{
		Sinks: []Sink{
			{Type: SinkTypeAxonkit, Config: json.RawMessage(`{not json`)},
		},
	}
	if err := job.Normalize(); err == nil {
		t.Fatal("expected JSON error to propagate")
	}
}

func TestNode_NormalizeConfig_VideoMixerClamps(t *testing.T) {
	n := Node{
		Type:   NodeTypeVideoMixer,
		Config: json.RawMessage(`{"canvas":{"width":99999,"height":99999,"fps":999},"layout":{"mode":"grid","grid":{"maxColumns":99}}}`),
	}
	if err := n.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got VideoMixerConfig
	if err := json.Unmarshal(n.Config, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Canvas.Width != 1920 || got.Canvas.Height != 1080 {
		t.Errorf("canvas %dx%d, want 1920x1080", got.Canvas.Width, got.Canvas.Height)
	}
	if got.Canvas.Fps != MaxCanvasFps {
		t.Errorf("canvas fps = %d, want %d", got.Canvas.Fps, MaxCanvasFps)
	}
	if got.Layout.Grid == nil {
		t.Fatalf("grid missing")
	}
	if got.Layout.Grid.MaxColumns != MaxGridColumns {
		t.Errorf("grid maxColumns = %d, want %d", got.Layout.Grid.MaxColumns, MaxGridColumns)
	}
}

func TestNode_NormalizeConfig_VideoMixerRadiusClamps(t *testing.T) {
	n := Node{
		Type: NodeTypeVideoMixer,
		Config: json.RawMessage(`{
			"canvas":{"width":1280,"height":720},
			"layout":{
				"mode":"custom",
				"common":{
					"borderRadius":-8,
					"nameLabel":{"enabled":true,"borderRadius":-4}
				},
				"custom":{"regions":[{
					"name":"r1","width":100,"height":100,
					"style":{"borderRadius":-12},
					"placeholder":{"image":{"url":"https://example.com/a.png","borderRadius":-1}}
				}]}
			}
		}`),
	}
	if err := n.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got VideoMixerConfig
	if err := json.Unmarshal(n.Config, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Layout.Common == nil {
		t.Fatalf("common missing")
	}
	if got.Layout.Common.BorderRadius != 0 {
		t.Errorf("borderRadius = %v, want 0", got.Layout.Common.BorderRadius)
	}
	if got.Layout.Common.NameLabel == nil || got.Layout.Common.NameLabel.BorderRadius != 0 {
		t.Errorf("nameLabel.borderRadius = %v, want 0", got.Layout.Common.NameLabel)
	}
	if got.Layout.Custom == nil || len(got.Layout.Custom.Regions) != 1 {
		t.Fatalf("custom regions missing")
	}
	r := got.Layout.Custom.Regions[0]
	if r.Style == nil || r.Style.BorderRadius != 0 {
		t.Errorf("region.style.borderRadius = %v, want 0", r.Style)
	}
	if r.Placeholder == nil || r.Placeholder.Image == nil || r.Placeholder.Image.BorderRadius != 0 {
		t.Errorf("placeholder.image.borderRadius = %v, want 0", r.Placeholder)
	}
}

func TestNode_NormalizeConfig_VideoMixerFieldClamps(t *testing.T) {
	n := Node{
		Type: NodeTypeVideoMixer,
		Config: json.RawMessage(`{
			"canvas":{"width":1280,"height":720,"coordinateSystem":"bogus"},
			"layout":{
				"mode":"speaker",
				"grid":{"fit":"stretch","gap":-3},
				"speaker":{
					"mainFit":"nope","thumbnailFit":"stretch",
					"thumbnailPosition":"diagonal","mainGap":-2,"thumbnailGap":-3
				},
				"custom":{"regions":[{
					"name":"r1","width":-10,"height":-20,"fit":"weird",
					"style":{"borderWidth":-5,"opacity":1.5},
					"placeholder":{
						"image":{"url":"https://example.com/a.png","width":-1,"height":-2,"fit":"stretch"},
						"text":{"content":"hi","fontSize":-3,"position":"side"}
					}
				}]}
			}
		}`),
	}
	if err := n.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got VideoMixerConfig
	if err := json.Unmarshal(n.Config, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Canvas.CoordinateSystem != "pixel" {
		t.Errorf("coordinateSystem = %q, want pixel", got.Canvas.CoordinateSystem)
	}
	if got.Layout.Grid == nil || got.Layout.Grid.Fit != "fill" || got.Layout.Grid.Gap != 0 {
		t.Errorf("grid = %+v, want fit=fill gap=0", got.Layout.Grid)
	}
	if got.Layout.Speaker == nil {
		t.Fatalf("speaker missing")
	}
	if got.Layout.Speaker.MainFit != "auto" {
		t.Errorf("mainFit = %q, want auto", got.Layout.Speaker.MainFit)
	}
	if got.Layout.Speaker.ThumbnailFit != "fill" {
		t.Errorf("thumbnailFit = %q, want fill", got.Layout.Speaker.ThumbnailFit)
	}
	if got.Layout.Speaker.ThumbnailPosition != "bottom" {
		t.Errorf("thumbnailPosition = %q, want bottom", got.Layout.Speaker.ThumbnailPosition)
	}
	if got.Layout.Speaker.MainGap != 0 {
		t.Errorf("speaker.mainGap = %v, want 0", got.Layout.Speaker.MainGap)
	}
	if got.Layout.Speaker.ThumbnailGap != 0 {
		t.Errorf("speaker.thumbnailGap = %v, want 0", got.Layout.Speaker.ThumbnailGap)
	}
	if got.Layout.Custom == nil || len(got.Layout.Custom.Regions) != 1 {
		t.Fatalf("custom regions missing")
	}
	r := got.Layout.Custom.Regions[0]
	if r.Width != 0 || r.Height != 0 {
		t.Errorf("region size = %vx%v, want 0x0", r.Width, r.Height)
	}
	if r.Fit != "" {
		t.Errorf("region.fit = %q, want empty", r.Fit)
	}
	if r.Style == nil || r.Style.BorderWidth != 0 || r.Style.Opacity != 1 {
		t.Errorf("region.style = %+v, want borderWidth=0 opacity=1", r.Style)
	}
	img := r.Placeholder.Image
	if img.Width != 0 || img.Height != 0 || img.Fit != "fill" {
		t.Errorf("placeholder.image = %+v, want w/h=0 fit=fill", img)
	}
	text := r.Placeholder.Text
	if text.FontSize != 0 || text.Position != "center" {
		t.Errorf("placeholder.text = %+v, want fontSize=0 position=center", text)
	}
}

func TestNode_NormalizeConfig_VideoMixerCanvasFpsDefault(t *testing.T) {
	n := Node{
		Type:   NodeTypeVideoMixer,
		Config: json.RawMessage(`{"canvas":{"width":1280,"height":720},"layout":{"mode":"grid"}}`),
	}
	if err := n.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got VideoMixerConfig
	if err := json.Unmarshal(n.Config, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Canvas.Fps != DefaultCanvasFps {
		t.Errorf("canvas fps = %d, want default %d", got.Canvas.Fps, DefaultCanvasFps)
	}
}

func TestCanvas_SetDefaults_1080pLandscape(t *testing.T) {
	c := Canvas{Width: 3000, Height: 2000}
	c.SetDefaults()
	if c.Width != 1920 || c.Height != 1080 {
		t.Fatalf("canvas %dx%d, want 1920x1080", c.Width, c.Height)
	}
}

func TestCanvas_SetDefaults_1080pPortrait(t *testing.T) {
	c := Canvas{Width: 2000, Height: 3000}
	c.SetDefaults()
	if c.Width != 1080 || c.Height != 1920 {
		t.Fatalf("canvas %dx%d, want 1080x1920", c.Width, c.Height)
	}
}

func TestSnapEncodingAudioSampleRate(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, DefaultEncodingAudioSampleRate},
		{8000, 8000},
		{44100, 44100},
		{48000, 48000},
		{24000, 16000},
		{96000, 48000},
	}
	for _, tc := range cases {
		if got := SnapEncodingAudioSampleRate(tc.in); got != tc.want {
			t.Errorf("SnapEncodingAudioSampleRate(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNode_NormalizeConfig_AudioMixerClamps(t *testing.T) {
	n := Node{
		Type:   NodeTypeAudioMixer,
		Config: json.RawMessage(`{"sampleRate":192000,"channels":8}`),
	}
	if err := n.NormalizeConfig(); err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	var got AudioMixerConfig
	if err := json.Unmarshal(n.Config, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SampleRate != DefaultSampleRate {
		t.Errorf("sampleRate=%d, want %d (snap to 48000)", got.SampleRate, DefaultSampleRate)
	}
	if got.Channels != MaxAudioChannels {
		t.Errorf("channels=%d, want %d", got.Channels, MaxAudioChannels)
	}
}

func TestAudioMixer_SetDefaults_SnapsToAllowedRates(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, DefaultSampleRate},
		{8000, 8000},
		{44100, 44100},
		{48000, 48000},
		{23000, 24000}, // closer to 24000 than 22050 (not allowed) / 16000
		{40000, 44100}, // closer to 44100 than 32000
		{96000, 48000}, // clamps to highest allowed
		{6000, 8000},   // clamps to lowest allowed
	}
	for _, tc := range cases {
		c := AudioMixerConfig{SampleRate: tc.in, Channels: 2}
		c.SetDefaults()
		if c.SampleRate != tc.want {
			t.Errorf("SetDefaults(%d).SampleRate = %d, want %d", tc.in, c.SampleRate, tc.want)
		}
	}
}

func TestPolicies_SetDefaults_IdleTimeout(t *testing.T) {
	var p Policies
	p.SetDefaults()
	if p.IdleTimeoutSec != DefaultIdleTimeout {
		t.Errorf("IdleTimeoutSec=%d, want %d", p.IdleTimeoutSec, DefaultIdleTimeout)
	}
}

func TestPolicies_SetDefaults_ClampsCaps(t *testing.T) {
	p := Policies{MaxDurationSec: 99 * 24 * 3600, IdleTimeoutSec: 99999}
	p.SetDefaults()
	if p.MaxDurationSec != MaxJobDurationSec {
		t.Errorf("MaxDurationSec=%d, want %d", p.MaxDurationSec, MaxJobDurationSec)
	}
	if p.IdleTimeoutSec != MaxIdleTimeoutSec {
		t.Errorf("IdleTimeoutSec=%d, want %d", p.IdleTimeoutSec, MaxIdleTimeoutSec)
	}
}

func TestWebCaptureAuthenticate_Configured(t *testing.T) {
	if (&WebCaptureAuthenticate{Username: "u", Password: "p"}).Configured() != true {
		t.Fatal("expected configured")
	}
	if (&WebCaptureAuthenticate{Username: "u"}).Configured() {
		t.Fatal("password required")
	}
	if (&WebCaptureAuthenticate{Password: "p"}).Configured() {
		t.Fatal("username required")
	}
	if (*WebCaptureAuthenticate)(nil).Configured() {
		t.Fatal("nil not configured")
	}
}

func TestWebCapture_GotoOptions_UnmarshalWaitUntil(t *testing.T) {
	var single WebCaptureGotoOptions
	if err := json.Unmarshal([]byte(`{"waitUntil":"networkidle2","timeout":45000}`), &single); err != nil {
		t.Fatal(err)
	}
	if len(single.WaitUntil) != 1 || single.WaitUntil[0] != "networkidle2" || single.Timeout != 45000 {
		t.Fatalf("single=%+v", single)
	}
	var multi WebCaptureGotoOptions
	if err := json.Unmarshal([]byte(`{"waitUntil":["load","networkidle0"]}`), &multi); err != nil {
		t.Fatal(err)
	}
	if len(multi.WaitUntil) != 2 || multi.WaitUntil[0] != "load" || multi.WaitUntil[1] != "networkidle0" {
		t.Fatalf("multi=%+v", multi)
	}
}

func TestWebCapture_BestAttempt_DefaultTrue(t *testing.T) {
	var c WebCaptureSourceConfig
	if !c.BestAttemptEnabled() {
		t.Fatal("nil bestAttempt should default to true")
	}
	falseVal := false
	c.BestAttempt = &falseVal
	if c.BestAttemptEnabled() {
		t.Fatal("explicit false should disable bestAttempt")
	}
}

func TestWebCapture_SetDefaults_ClampsCaps(t *testing.T) {
	c := WebCaptureSourceConfig{
		Viewport: &WebCaptureViewport{Width: 9999, Height: 9999, DeviceScaleFactor: 99},
		FPS:      999,
	}
	c.SetDefaults()
	if c.CaptureWidth() != MaxWebCaptureWidth || c.CaptureHeight() != MaxWebCaptureHeight {
		t.Errorf("%dx%d, want %dx%d", c.CaptureWidth(), c.CaptureHeight(), MaxWebCaptureWidth, MaxWebCaptureHeight)
	}
	if c.Viewport.DeviceScaleFactor != MaxWebCaptureDeviceScaleFactor {
		t.Errorf("deviceScaleFactor=%v, want %v", c.Viewport.DeviceScaleFactor, MaxWebCaptureDeviceScaleFactor)
	}
	if c.FPS != MaxWebCaptureFullPageFPS {
		t.Errorf("FPS=%d, want %d", c.FPS, MaxWebCaptureFullPageFPS)
	}

	elem := WebCaptureSourceConfig{CaptureElement: "#video", FPS: 999}
	elem.SetDefaults()
	if elem.FPS != MaxWebCaptureElementFPS {
		t.Errorf("element FPS=%d, want %d", elem.FPS, MaxWebCaptureElementFPS)
	}
}
