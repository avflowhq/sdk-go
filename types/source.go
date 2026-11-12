package avflow

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type SourceType string

const (
	SourceTypeAxonkit        SourceType = "axonkit" // internal; not in public docs yet
	SourceTypeLivekit        SourceType = "livekit"
	SourceTypeJitsi          SourceType = "jitsi"
	SourceTypeDaily          SourceType = "daily"
	SourceTypeAgora          SourceType = "agora"
	SourceTypeWHEP           SourceType = "whep"
	SourceTypeWebCapture     SourceType = "web_capture"
	SourceTypeVideoGenerator SourceType = "video_generator"
	SourceTypeRTMPPull       SourceType = "rtmp_pull"
	SourceTypeHTTPFLVPull    SourceType = "httpflv_pull"
	SourceTypeHLSPull        SourceType = "hls_pull"
	SourceTypeSRTPull        SourceType = "srt_pull"
	SourceTypeRTSPPull       SourceType = "rtsp_pull"
	SourceTypeFilePull       SourceType = "file_pull"
)

// Hard caps shared across source configs. SetDefaults clamps values to these.
const (
	MaxReconnectMaxRetries         = 100
	MaxReconnectIntervalMs         = 60_000  // 1m
	MaxStartTimeoutMs              = 600_000 // 10m
	MaxStallTimeoutMs              = 600_000 // 10m
	MaxWebCaptureGotoTimeoutMs     = 60_000  // gotoOptions.timeout cap (CF max ~60s)
	MaxWebCaptureWaitForTimeoutMs  = 60_000  // waitForSelector.timeout / waitForTimeout cap
	MaxWebCaptureFullPageFPS       = 10      // FPS cap for getDisplayMedia / screenshot poll
	MaxWebCaptureElementFPS        = 60      // FPS cap for element.captureStream
	MaxWebCaptureWidth             = 1920    // Width cap for getDisplayMedia / screenshot
	MaxWebCaptureHeight            = 1080    // Height cap for getDisplayMedia / screenshot
	MaxWebCaptureDeviceScaleFactor = 4       // deviceScaleFactor cap (Puppeteer-style)
)

type Source struct {
	Name   string          `json:"name"`
	Type   SourceType      `json:"type"`
	Config json.RawMessage `json:"config"`
}

func (s *Source) OutputMediaTypes() []MediaType {
	if s.Type == SourceTypeVideoGenerator {
		return []MediaType{MediaTypeVideo}
	}
	return []MediaType{MediaTypeVideo, MediaTypeAudio}
}

func (s *Source) ParseConfig(v any) error {
	return json.Unmarshal(s.Config, v)
}

func (s *Source) NormalizeConfig() error {
	if s.Type != SourceTypeVideoGenerator {
		return nil
	}
	var cfg VideoGeneratorSourceConfig
	if len(s.Config) > 0 {
		if err := json.Unmarshal(s.Config, &cfg); err != nil {
			return err
		}
	}
	cfg.SetDefaults()
	out, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	s.Config = out
	return nil
}

// VideoGeneratorSourceConfig produces a constant-color raw video track.
type VideoGeneratorSourceConfig struct {
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	Fps             int    `json:"fps,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

func (c *VideoGeneratorSourceConfig) SetDefaults() {
	if c.Width < 2 {
		c.Width = 16
	}
	if c.Height < 2 {
		c.Height = 16
	}
	clampCanvasSize(&c.Width, &c.Height)
	// YUV420 encoders require even dimensions.
	c.Width -= c.Width % 2
	c.Height -= c.Height % 2
	if c.Fps <= 0 {
		c.Fps = 15
	}
	if c.Fps > MaxCanvasFps {
		c.Fps = MaxCanvasFps
	}
	if c.BackgroundColor == "" {
		c.BackgroundColor = "#000000"
	}
}

// AxonkitSourceConfig subscribes to an Axonkit room (https://axonkit.io) and
// forwards the subscribed tracks downstream. The room is determined by the
// JWT's room grant.
type AxonkitSourceConfig struct {
	ServerURL string        `json:"serverUrl"`
	Token     string        `json:"token"`
	GroupSid  string        `json:"groupSid,omitempty"` // when set, only subscribe to streams from this group
	Select    *SelectFilter `json:"select,omitempty"`
}

// LivekitSourceConfig subscribes to a standard LiveKit room and forwards the
// subscribed tracks downstream. It mirrors AxonkitSourceConfig but drops the
// axonkit-only group concept: the room is determined by the JWT.
type LivekitSourceConfig struct {
	ServerURL string        `json:"serverUrl"`
	Token     string        `json:"token"`
	Select    *SelectFilter `json:"select,omitempty"`
}

// JitsiSourceConfig subscribes to a Jitsi conference (via lib-jitsi-meet) and
// forwards remote tracks downstream. Connection/conference options are passed
// through to the browser SDK. Domain identifies the Jitsi deployment and
// derives sensible connection defaults when ConnectionOptions is omitted. The
// lib-jitsi-meet bundle is served from AVFlow's own origin.
type JitsiSourceConfig struct {
	Domain   string `json:"domain"`
	RoomName string `json:"roomName"`
	Token    string `json:"token,omitempty"`
	AppID    string `json:"appId,omitempty"`
	// XmppID is the XMPP login JID. A recorder account on the deployment's
	// hiddenDomain (e.g. recorder@recorder.meet.example.com) is hidden from
	// other jitsi-meet clients. Must be set together with XmppPassword.
	XmppID            string         `json:"xmppId,omitempty"`
	XmppPassword      string         `json:"xmppPassword,omitempty"`
	ConnectionOptions map[string]any `json:"connectionOptions,omitempty"`
	ConferenceOptions map[string]any `json:"conferenceOptions,omitempty"`
	Select            *SelectFilter  `json:"select,omitempty"`
}

// DailySourceConfig subscribes to a Daily room (via @daily-co/daily-js) and
// forwards remote tracks downstream. RoomURL is the full Daily room URL
// (https://<domain>.daily.co/<room>); Token authorizes private rooms.
type DailySourceConfig struct {
	RoomURL  string        `json:"roomUrl"`
	Token    string        `json:"token,omitempty"`
	UserName string        `json:"userName,omitempty"`
	Select   *SelectFilter `json:"select,omitempty"`
}

// AgoraSourceConfig subscribes to an Agora RTC channel (via agora-rtc-sdk-ng)
// and forwards remote tracks downstream. Agora identifies an endpoint by
// AppID + Channel; Token authorizes the join (omit for testing/no-auth
// projects). UID is the local subscriber identity (number or user account;
// omit or 0 lets Agora assign a numeric uid).
type AgoraSourceConfig struct {
	AppID   string        `json:"appId"`
	Channel string        `json:"channel"`
	Token   string        `json:"token,omitempty"`
	UID     AgoraUID      `json:"uid,omitempty"`
	Select  *SelectFilter `json:"select,omitempty"`
}

// URLPullSourceConfig is shared by the url-addressed pull sources:
// file_pull, rtmp_pull, httpflv_pull, hls_pull, srt_pull, rtsp_pull, and whep.
type URLPullSourceConfig struct {
	URL string `json:"url"`
	// Identity labels this single-stream source as a speaker/user. It becomes the
	// track identity used for active-speaker tracking and ASR attribution; empty
	// falls back to the source name.
	Identity string `json:"identity,omitempty"`
	// DisplayName is the human-readable label shown in overlays/captions; empty
	// falls back to identity, then the source name.
	DisplayName string `json:"displayName,omitempty"`
	// Headers are forwarded to libavformat HTTP (HLS playlist/segment, HTTP-FLV).
	Headers map[string]string `json:"headers,omitempty"`
	// StartTimeoutMs aborts the source if no first frame arrives in time.
	// 0 = default 30000, <0 = disabled.
	StartTimeoutMs int `json:"startTimeoutMs,omitempty"`
	// StallTimeoutMs aborts the source after N ms with no new frame
	// following the first frame. 0 = disabled.
	StallTimeoutMs int `json:"stallTimeoutMs,omitempty"`
	// Loop replays the file from the start on EOF; the internal PTS offset
	// accumulates so sink timestamps stay monotonically increasing. Default false.
	Loop bool `json:"loop,omitempty"`
}

func (c *URLPullSourceConfig) SetDefaults() {
	if c.StartTimeoutMs == 0 {
		c.StartTimeoutMs = 30_000
	}
	if c.StartTimeoutMs > MaxStartTimeoutMs {
		c.StartTimeoutMs = MaxStartTimeoutMs
	}
	if c.StallTimeoutMs < 0 {
		c.StallTimeoutMs = 0
	}
	if c.StallTimeoutMs > MaxStallTimeoutMs {
		c.StallTimeoutMs = MaxStallTimeoutMs
	}
}

// ValidateFilePullURL enforces file_pull's public contract: it must be addressed
// by a parsed URL with an explicit protocol, and local file URLs are rejected.
func (c *URLPullSourceConfig) ValidateFilePullURL() error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("file_pull url is invalid: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("file_pull url must include protocol")
	}
	if strings.EqualFold(u.Scheme, "file") {
		return fmt.Errorf("file_pull url must not use file protocol")
	}
	return nil
}

// WebCaptureViewport aligns with Cloudflare Browser Rendering / Puppeteer
// Page.setViewport (width, height, deviceScaleFactor, mobile/touch/orientation).
type WebCaptureViewport struct {
	Width             int     `json:"width,omitempty"`
	Height            int     `json:"height,omitempty"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor,omitempty"`
	HasTouch          *bool   `json:"hasTouch,omitempty"`
	IsLandscape       *bool   `json:"isLandscape,omitempty"`
	IsMobile          *bool   `json:"isMobile,omitempty"`
}

// WebCaptureGotoOptions aligns with Cloudflare/Puppeteer page.goto options.
type WebCaptureGotoOptions struct {
	// WaitUntil lists the navigation lifecycle events to wait for before
	// considering the page loaded. Supported values (case-insensitive):
	// "load", "domcontentloaded", "networkidle0" (no in-flight requests for
	// 500ms) and "networkidle2" (<=2 in-flight requests for 500ms). Empty
	// defaults to "load"; any other value is rejected.
	WaitUntil      []string `json:"waitUntil,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	Referer        string   `json:"referer,omitempty"`
	ReferrerPolicy string   `json:"referrerPolicy,omitempty"`
}

func (g *WebCaptureGotoOptions) UnmarshalJSON(data []byte) error {
	type plain WebCaptureGotoOptions
	aux := struct {
		plain
		WaitUntil json.RawMessage `json:"waitUntil,omitempty"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*g = WebCaptureGotoOptions(aux.plain)
	if len(aux.WaitUntil) == 0 || string(aux.WaitUntil) == "null" {
		return nil
	}
	var one string
	if err := json.Unmarshal(aux.WaitUntil, &one); err == nil {
		g.WaitUntil = []string{one}
		return nil
	}
	return json.Unmarshal(aux.WaitUntil, &g.WaitUntil)
}

// WebCaptureAuthenticate provides HTTP Basic credentials.
type WebCaptureAuthenticate struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Configured reports whether both username and password are set.
func (a *WebCaptureAuthenticate) Configured() bool {
	return a != nil && a.Username != "" && a.Password != ""
}

// WebCaptureCookie aligns with Puppeteer page.setCookie / CF cookies[].
type WebCaptureCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain,omitempty"`
	Path     string  `json:"path,omitempty"`
	URL      string  `json:"url,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly *bool   `json:"httpOnly,omitempty"`
	Secure   *bool   `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"` // Strict | Lax | None
}

// WebCaptureWaitForSelector aligns with CF waitForSelector.
type WebCaptureWaitForSelector struct {
	Selector string `json:"selector"`
	Timeout  int    `json:"timeout,omitempty"`
	Visible  *bool  `json:"visible,omitempty"`
	Hidden   *bool  `json:"hidden,omitempty"`
}

// WebCaptureSourceConfig captures a web page or a single DOM element.
//
// Viewport width / height behave differently per capture mode:
//   - getDisplayMedia path (CaptureElement empty): controls Linux Xvfb size,
//     chromedp.EmulateViewport, and getDisplayMedia ideal constraints.
//     SetDefaults fills 0 / clamps >1920x1080 to 1920x1080.
//   - element.captureStream path (CaptureElement non-empty): 0 keeps the
//     element's native size; non-zero scales via OffscreenCanvas + VideoFrame
//     (pass-through when the source frame already matches the target size).
//   - screencast path (remote CDP screenshot): EmulateViewport before capture;
//     JPEG pixels ≈ width×deviceScaleFactor by height×deviceScaleFactor.
//
// FPS is interpreted as:
//   - getDisplayMedia path: frameRate.ideal.
//   - element.captureStream path: sampling frame rate.
//   - screencast path: derived from EveryNthFrame.
//
// SetDefaults caps FPS at MaxWebCaptureFullPageFPS (full-page paths are CPU
// heavy) or MaxWebCaptureElementFPS (element path is GPU-compose light).
//
// CaptureElement is a CSS selector enabling element-level capture. When set:
//   - video uses el.captureStream(FPS); viewport width/height become target scale.
//   - getDisplayMedia is skipped (local Chrome stays headless, Linux skips Xvfb).
//   - remote headless browsers (Cloudflare Browser Run, ...) without this
//     field fall back to CDP Page.captureScreenshot poll (remote full-page,
//     no audio).
type WebCaptureSourceConfig struct {
	URL                 string                     `json:"url"`
	Viewport            *WebCaptureViewport        `json:"viewport,omitempty"`
	GotoOptions         *WebCaptureGotoOptions     `json:"gotoOptions,omitempty"`
	Authenticate        *WebCaptureAuthenticate    `json:"authenticate,omitempty"`
	Cookies             []WebCaptureCookie         `json:"cookies,omitempty"`
	SetExtraHTTPHeaders map[string]string          `json:"setExtraHTTPHeaders,omitempty"`
	WaitForSelector     *WebCaptureWaitForSelector `json:"waitForSelector,omitempty"`
	WaitForTimeout      int                        `json:"waitForTimeout,omitempty"`
	// BestAttempt soft-fails awaited events (goto waitUntil, waitForSelector;
	// default true when omitted).
	BestAttempt    *bool  `json:"bestAttempt,omitempty"`
	FPS            int    `json:"fps,omitempty"`
	CaptureAudio   bool   `json:"captureAudio,omitempty"`
	CaptureElement string `json:"captureElement,omitempty"`
}

func (c *WebCaptureSourceConfig) ensureViewport() *WebCaptureViewport {
	if c.Viewport == nil {
		c.Viewport = &WebCaptureViewport{}
	}
	return c.Viewport
}

// CaptureWidth returns viewport width (0 when unset).
func (c *WebCaptureSourceConfig) CaptureWidth() int {
	if c.Viewport == nil {
		return 0
	}
	return c.Viewport.Width
}

// CaptureHeight returns viewport height (0 when unset).
func (c *WebCaptureSourceConfig) CaptureHeight() int {
	if c.Viewport == nil {
		return 0
	}
	return c.Viewport.Height
}

// BestAttemptEnabled reports whether awaited wait failures are soft (default true).
func (c *WebCaptureSourceConfig) BestAttemptEnabled() bool {
	return webCaptureBestAttempt(c.BestAttempt)
}

func webCaptureBestAttempt(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

func (c *WebCaptureSourceConfig) SetDefaults() {
	v := c.ensureViewport()
	if c.CaptureElement == "" {
		if v.Width == 0 || v.Width > MaxWebCaptureWidth {
			v.Width = MaxWebCaptureWidth
		}
		if v.Height == 0 || v.Height > MaxWebCaptureHeight {
			v.Height = MaxWebCaptureHeight
		}
	} else {
		if v.Width < 0 {
			v.Width = 0
		}
		if v.Height < 0 {
			v.Height = 0
		}
		if v.Width > MaxWebCaptureWidth {
			v.Width = MaxWebCaptureWidth
		}
		if v.Height > MaxWebCaptureHeight {
			v.Height = MaxWebCaptureHeight
		}
	}
	if v.DeviceScaleFactor <= 0 {
		v.DeviceScaleFactor = 1
	}
	if v.DeviceScaleFactor > MaxWebCaptureDeviceScaleFactor {
		v.DeviceScaleFactor = MaxWebCaptureDeviceScaleFactor
	}
	if c.FPS == 0 {
		c.FPS = 5
	}
	maxFPS := MaxWebCaptureElementFPS
	if c.CaptureElement == "" {
		maxFPS = MaxWebCaptureFullPageFPS
	}
	if c.FPS < 0 {
		c.FPS = 0
	}
	if c.FPS > maxFPS {
		c.FPS = maxFPS
	}
	if c.GotoOptions != nil {
		c.GotoOptions.SetDefaults()
	}
	if c.WaitForSelector != nil {
		c.WaitForSelector.SetDefaults()
	}
	if c.WaitForTimeout < 0 {
		c.WaitForTimeout = 0
	}
	if c.WaitForTimeout > MaxWebCaptureWaitForTimeoutMs {
		c.WaitForTimeout = MaxWebCaptureWaitForTimeoutMs
	}
}

func (g *WebCaptureGotoOptions) SetDefaults() {
	if g.Timeout < 0 {
		g.Timeout = 0
	}
	if g.Timeout > MaxWebCaptureGotoTimeoutMs {
		g.Timeout = MaxWebCaptureGotoTimeoutMs
	}
}

func (w *WebCaptureWaitForSelector) SetDefaults() {
	if w.Timeout < 0 {
		w.Timeout = 0
	}
	if w.Timeout > MaxWebCaptureWaitForTimeoutMs {
		w.Timeout = MaxWebCaptureWaitForTimeoutMs
	}
}
