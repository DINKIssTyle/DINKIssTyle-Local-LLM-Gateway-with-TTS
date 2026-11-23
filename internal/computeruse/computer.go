// Package computeruse controls the desktop of the gateway host, not the remote
// chat client's computer. All requests share one physical desktop.
package computeruse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Request struct {
	ObservationID string   `json:"observation_id,omitempty"`
	Application   string   `json:"application,omitempty"`
	PID           int      `json:"pid,omitempty"`
	X             *int     `json:"x,omitempty"`
	Y             *int     `json:"y,omitempty"`
	Button        string   `json:"button,omitempty"`
	Clicks        int      `json:"clicks,omitempty"`
	Text          string   `json:"text,omitempty"`
	Key           string   `json:"key,omitempty"`
	Modifiers     []string `json:"modifiers,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	Amount        int      `json:"amount,omitempty"`
}

type Screen struct {
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
	Observe(context.Context) (Screen, []byte, error)
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
		screen, png, err := c.backend.Observe(ctx)
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
		c.last = observation{id: hex.EncodeToString(token[:]), owner: owner, at: time.Now(), screen: screen}
		raw, err := json.Marshal(struct {
			Screen
			ObservationID string `json:"observation_id"`
			Coordinates   string `json:"coordinates"`
		}{screen, c.last.id, "Use screenshot pixels, origin at top left. This observation is valid for one input action; observe again afterward. Only the primary display is captured."})
		return Output{Text: string(raw), PNG: png}, err
	}
	if action == "list_apps" {
		raw, err := c.backend.Execute(ctx, action, req)
		return Output{Text: string(raw)}, err
	}
	if action != "launch_app" && action != "switch_app" {
		last := c.last
		if last.id == "" || last.id != req.ObservationID || last.owner != owner || time.Since(last.at) > time.Minute {
			return Output{}, fmt.Errorf("missing, expired or consumed observation_id; call computer_observe before input")
		}
		if action == "click" {
			if *req.X < 0 || *req.X >= last.screen.Width || *req.Y < 0 || *req.Y >= last.screen.Height {
				return Output{}, fmt.Errorf("coordinates are outside the observed screenshot")
			}
			x := int(float64(*req.X) * last.screen.DesktopWidth / float64(last.screen.Width))
			y := int(float64(*req.Y) * last.screen.DesktopHeight / float64(last.screen.Height))
			req.X, req.Y = &x, &y
		}
		// The backend checks this again immediately before posting input.
		req.PID = last.screen.FrontmostPID
	}
	c.last = observation{}
	raw, err := c.backend.Execute(ctx, action, req)
	return Output{Text: string(raw)}, err
}

func validate(action string, r Request) error {
	switch action {
	case "observe", "list_apps":
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
