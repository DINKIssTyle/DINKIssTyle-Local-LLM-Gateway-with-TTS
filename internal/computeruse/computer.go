// Package computeruse controls the desktop of the gateway host, not the remote
// chat client's computer. All requests share one physical desktop.
package computeruse

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"sync"
	"time"
)

// ApplicationBundleAlias resolves known localized names to a stable identity.
func ApplicationBundleAlias(name string) string {
	switch strings.ToLower(strings.Join(strings.Fields(name), "")) {
	case "findmy", "findmy.app", "나의찾기", "com.apple.findmy":
		return "com.apple.findmy"
	default:
		return ""
	}
}

type Request struct {
	DisplayID     uint32   `json:"display_id,omitempty"`
	Display       *Display `json:"display_geometry,omitempty"`
	ObservationID string   `json:"observation_id,omitempty"`
	Application   string   `json:"application,omitempty"`
	PID           int      `json:"pid,omitempty"`
	X             *int     `json:"x,omitempty"`
	Y             *int     `json:"y,omitempty"`
	Width         *int     `json:"width,omitempty"`
	Height        *int     `json:"height,omitempty"`
	Caption       string   `json:"caption,omitempty"`
	Button        string   `json:"button,omitempty"`
	Clicks        int      `json:"clicks,omitempty"`
	Text          string   `json:"text,omitempty"`
	Key           string   `json:"key,omitempty"`
	Modifiers     []string `json:"modifiers,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	Amount        int      `json:"amount,omitempty"`
}

type Display struct {
	ID      uint32  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Primary bool    `json:"primary"`
}

type Screen struct {
	Display       Display         `json:"display"`
	Displays      []Display       `json:"displays,omitempty"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	DesktopWidth  float64         `json:"desktop_width"`
	DesktopHeight float64         `json:"desktop_height"`
	FrontmostPID  int             `json:"frontmost_pid"`
	Application   string          `json:"application"`
	Accessibility bool            `json:"accessibility"`
	Windows       json.RawMessage `json:"windows,omitempty"`
}

type Backend interface {
	Available() bool
	Observe(context.Context, uint32) (Screen, []byte, error)
	Execute(context.Context, string, Request) (json.RawMessage, error)
}

type Output struct {
	Text string
	PNG  []byte
}

type observation struct {
	id, owner string
	at        time.Time
	screen    Screen
	png       []byte
}

// Controller serializes capture and input, and rejects coordinates from an old
// screenshot or another conversation. Each input consumes its observation.
type Controller struct {
	mu      sync.Mutex
	backend Backend
	last    observation
}

func New(backend Backend) *Controller { return &Controller{backend: backend} }
func (c *Controller) Available() bool { return c.backend.Available() }

func (c *Controller) Call(ctx context.Context, owner, action string, req Request) (Output, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	if !c.Available() {
		return Output{}, fmt.Errorf("computer use is unavailable on this platform")
	}
	if err := validate(action, req); err != nil {
		return Output{}, err
	}
	if action == "observe" {
		c.last = observation{}
		screen, png, err := c.backend.Observe(ctx, req.DisplayID)
		if err != nil {
			return Output{}, err
		}
		if screen.Width <= 0 || screen.Height <= 0 || screen.DesktopWidth <= 0 || screen.DesktopHeight <= 0 || len(png) == 0 {
			return Output{}, fmt.Errorf("desktop capture returned invalid dimensions or an empty image")
		}
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return Output{}, err
		}
		c.last = observation{id: hex.EncodeToString(token[:]), owner: owner, at: time.Now(), screen: screen, png: png}
		raw, err := json.Marshal(struct {
			Screen
			ObservationID string `json:"observation_id"`
			Coordinates   string `json:"coordinates"`
		}{screen, c.last.id, "Use screenshot pixels, origin at top left. This observation is valid for one input action; observe again afterward. Coordinates refer to the selected display. Use display_id from displays to observe another display."})
		return Output{Text: string(raw), PNG: png}, err
	}
	if action == "screenshot" {
		srcPNG := c.last.png
		screen := c.last.screen
		displayMismatch := req.DisplayID != 0 && screen.Display.ID != req.DisplayID
		if len(srcPNG) == 0 || c.last.owner != owner || time.Since(c.last.at) > 5*time.Minute || displayMismatch {
			var err error
			screen, srcPNG, err = c.backend.Observe(ctx, req.DisplayID)
			if err != nil {
				return Output{}, err
			}
			if screen.Width <= 0 || screen.Height <= 0 || len(srcPNG) == 0 {
				return Output{}, fmt.Errorf("desktop capture returned invalid dimensions or an empty image")
			}
		}
		outPNG := srcPNG
		outW, outH := screen.Width, screen.Height
		cropped := false
		if req.X != nil && req.Y != nil && req.Width != nil && req.Height != nil {
			var err error
			outPNG, outW, outH, err = cropPNG(srcPNG, *req.X, *req.Y, *req.Width, *req.Height)
			if err != nil {
				return Output{}, err
			}
			cropped = true
		}
		raw, err := json.Marshal(map[string]any{
			"status":      "captured",
			"width":       outW,
			"height":      outH,
			"cropped":     cropped,
			"caption":     req.Caption,
			"application": screen.Application,
			"note":        "Screenshot captured and will be attached to the final reply.",
		})
		return Output{Text: string(raw), PNG: outPNG}, err
	}
	if action == "list_apps" {
		raw, err := c.backend.Execute(ctx, action, req)
		return Output{Text: string(raw)}, err
	}
	if action != "launch_app" && action != "switch_app" {
		last := c.last
		if last.id == "" || last.id != req.ObservationID || last.owner != owner || time.Since(last.at) > 5*time.Minute {
			return Output{}, fmt.Errorf("missing, expired or consumed observation_id; call computer_observe before input")
		}
		if action == "click" {
			if *req.X < 0 || *req.X >= last.screen.Width || *req.Y < 0 || *req.Y >= last.screen.Height {
				return Output{}, fmt.Errorf("coordinates are outside the observed screenshot")
			}
			x := int(math.Floor(last.screen.Display.X + float64(*req.X)*last.screen.DesktopWidth/float64(last.screen.Width)))
			y := int(math.Floor(last.screen.Display.Y + float64(*req.Y)*last.screen.DesktopHeight/float64(last.screen.Height)))
			req.X, req.Y = &x, &y
		}
		// The backend checks this again immediately before posting input.
		req.PID = last.screen.FrontmostPID
		req.Display = &last.screen.Display
	}
	c.last = observation{}
	raw, err := c.backend.Execute(ctx, action, req)
	return Output{Text: string(raw)}, err
}

func validate(action string, r Request) error {
	switch action {
	case "observe", "list_apps":
	case "screenshot":
		hasAny := r.X != nil || r.Y != nil || r.Width != nil || r.Height != nil
		hasAll := r.X != nil && r.Y != nil && r.Width != nil && r.Height != nil
		if hasAny && !hasAll {
			return fmt.Errorf("x, y, width, and height must all be provided when cropping a screenshot")
		}
		if hasAll && (*r.Width < 16 || *r.Height < 16) {
			return fmt.Errorf("crop width and height must be at least 16 pixels")
		}
		if len([]rune(r.Caption)) > 200 {
			return fmt.Errorf("caption must be at most 200 characters")
		}
	case "launch_app":
		if len(r.Application) == 0 || len(r.Application) > 1024 || r.Application[0] == '-' {
			return fmt.Errorf("application must be an app name, bundle ID or absolute .app path")
		}
	case "switch_app":
		if r.PID <= 0 || r.PID > 2147483647 {
			return fmt.Errorf("pid must identify a running application from computer_list_apps")
		}
	case "click":
		if r.X == nil || r.Y == nil {
			return fmt.Errorf("x and y are required")
		}
		if r.Button != "" && r.Button != "left" && r.Button != "right" {
			return fmt.Errorf("button must be left or right")
		}
		if r.Clicks < 0 || r.Clicks > 2 {
			return fmt.Errorf("clicks must be 1 or 2")
		}
	case "type":
		if len([]rune(r.Text)) == 0 || len([]rune(r.Text)) > 2000 {
			return fmt.Errorf("text must contain 1 to 2000 characters")
		}
	case "key":
		if _, ok := KeyCodes[r.Key]; !ok {
			return fmt.Errorf("unsupported key %q; use a named key or lowercase ASCII letter/digit", r.Key)
		}
		if len(r.Modifiers) > 4 {
			return fmt.Errorf("at most four modifiers are supported")
		}
		for _, m := range r.Modifiers {
			if m != "command" && m != "control" && m != "option" && m != "shift" {
				return fmt.Errorf("unsupported modifier %q", m)
			}
		}
	case "scroll":
		if r.Direction != "up" && r.Direction != "down" && r.Direction != "left" && r.Direction != "right" {
			return fmt.Errorf("direction must be up, down, left or right")
		}
		if r.Amount < 1 || r.Amount > 20 {
			return fmt.Errorf("amount must be 1 to 20 scroll lines")
		}
	default:
		return fmt.Errorf("unknown computer action %q", action)
	}
	return nil
}

// macOS virtual key codes. Text input uses Unicode events instead of this map.
var KeyCodes = map[string]int{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9, "b": 11,
	"q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23,
	"9": 25, "7": 26, "8": 28, "0": 29, "o": 31, "u": 32, "i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
	"enter": 36, "tab": 48, "space": 49, "backspace": 51, "escape": 53, "home": 115, "pageup": 116, "delete": 117, "end": 119, "pagedown": 121,
	"left": 123, "right": 124, "down": 125, "up": 126,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97, "f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
}

var Default = New(newNativeBackend())

func cropPNG(src []byte, x, y, width, height int) ([]byte, int, int, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode screenshot for crop: %w", err)
	}
	b := img.Bounds()
	x0 := max(b.Min.X, b.Min.X+x)
	y0 := max(b.Min.Y, b.Min.Y+y)
	x1 := min(b.Max.X, b.Min.X+x+width)
	y1 := min(b.Max.Y, b.Min.Y+y+height)
	if x1-x0 < 16 || y1-y0 < 16 {
		return nil, 0, 0, fmt.Errorf("crop region (%d,%d,%d,%d) is outside the screenshot (%dx%d)", x, y, width, height, b.Dx(), b.Dy())
	}
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(x0, y0), draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), x1 - x0, y1 - y0, nil
}
