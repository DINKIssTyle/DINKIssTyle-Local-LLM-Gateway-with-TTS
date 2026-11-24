package computeruse

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeBackend struct {
	calls   int
	last    Request
	fail    bool
	screens map[uint32]Screen
}

func (*fakeBackend) Available() bool { return true }
func (b *fakeBackend) Observe(_ context.Context, displayID uint32) (Screen, []byte, error) {
	if b.screens != nil {
		screen, ok := b.screens[displayID]
		if !ok {
			return Screen{}, nil, errors.New("display not found")
		}
		return screen, []byte("png"), nil
	}
	return Screen{Width: 1000, Height: 500, DesktopWidth: 2000, DesktopHeight: 1000, FrontmostPID: 42}, []byte("png"), nil
}
func (b *fakeBackend) Execute(_ context.Context, _ string, r Request) (json.RawMessage, error) {
	b.calls++
	b.last = r
	if b.fail {
		return nil, errors.New("foreground changed")
	}
	return json.RawMessage(`{"status":"input_posted"}`), nil
}
func observeID(t *testing.T, c *Controller, owner string) string {
	t.Helper()
	out, err := c.Call(context.Background(), owner, "observe", Request{})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		ObservationID string `json:"observation_id"`
	}
	if err := json.Unmarshal([]byte(out.Text), &result); err != nil {
		t.Fatal(err)
	}
	if result.ObservationID == "" || len(out.PNG) == 0 {
		t.Fatal("missing screenshot or observation ID")
	}
	return result.ObservationID
}

func TestInputUsesScreenshotCoordinatesAndConsumesObservation(t *testing.T) {
	b := &fakeBackend{}
	c := New(b)
	id := observeID(t, c, "chat1")
	x, y := 100, 200
	r := Request{ObservationID: id, X: &x, Y: &y}
	if _, err := c.Call(context.Background(), "chat2", "click", r); err == nil {
		t.Fatal("another chat used the observation")
	}
	if b.calls != 0 {
		t.Fatal("invalid owner reached backend")
	}
	if _, err := c.Call(context.Background(), "chat1", "click", r); err != nil {
		t.Fatal(err)
	}
	if *b.last.X != 200 || *b.last.Y != 400 || b.last.PID != 42 {
		t.Fatalf("wrong coordinate conversion or foreground guard: %+v", b.last)
	}
	if _, err := c.Call(context.Background(), "chat1", "click", r); err == nil {
		t.Fatal("replayed an input")
	}
	if b.calls != 1 {
		t.Fatal("replay reached backend")
	}
}

func TestObservationExpiryAndReplacement(t *testing.T) {
	c := New(&fakeBackend{})
	old := observeID(t, c, "chat")
	id := observeID(t, c, "chat")
	if id == old {
		t.Fatal("observation ID reused")
	}
	if _, err := c.Call(context.Background(), "chat", "type", Request{ObservationID: old, Text: "한글"}); err == nil {
		t.Fatal("old capture accepted")
	}
	c.last.at = time.Now().Add(-6 * time.Minute)
	if _, err := c.Call(context.Background(), "chat", "type", Request{ObservationID: id, Text: "한글"}); err == nil {
		t.Fatal("expired capture accepted")
	}
}

func TestInvalidInputNeverReachesDesktop(t *testing.T) {
	b := &fakeBackend{}
	c := New(b)
	x, y := 1000, 1
	for _, tc := range []struct {
		action string
		req    Request
	}{
		{"click", Request{X: &x}}, {"click", Request{X: &x, Y: &y}},
		{"key", Request{Key: "invalid"}}, {"key", Request{Key: "a", Modifiers: []string{"meta"}}},
		{"type", Request{}}, {"scroll", Request{Direction: "down", Amount: 21}},
		{"scroll", Request{Direction: "diagonal", Amount: 1}}, {"switch_app", Request{PID: -1}},
		{"launch_app", Request{Application: "-a"}},
	} {
		tc.req.ObservationID = observeID(t, c, "chat")
		if _, err := c.Call(context.Background(), "chat", tc.action, tc.req); err == nil {
			t.Fatalf("invalid %s request accepted", tc.action)
		}
	}
	if b.calls != 0 {
		t.Fatal("invalid input reached backend")
	}
}

func TestAppSwitchAndBackendFailureInvalidateCoordinates(t *testing.T) {
	b := &fakeBackend{}
	c := New(b)
	id := observeID(t, c, "chat")
	if _, err := c.Call(context.Background(), "chat", "switch_app", Request{PID: 99}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Call(context.Background(), "chat", "type", Request{ObservationID: id, Text: "test"}); err == nil {
		t.Fatal("old app observation survived switch")
	}
	id = observeID(t, c, "chat")
	b.fail = true
	if _, err := c.Call(context.Background(), "chat", "type", Request{ObservationID: id, Text: "test"}); err == nil {
		t.Fatal("backend failure lost")
	}
	b.fail = false
	if _, err := c.Call(context.Background(), "chat", "type", Request{ObservationID: id, Text: "test"}); err == nil {
		t.Fatal("failed action could be replayed")
	}
}

func TestCancelledRequestDoesNotTouchDesktop(t *testing.T) {
	b := &fakeBackend{}
	c := New(b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Call(ctx, "chat", "launch_app", Request{Application: "Safari"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("wrong cancellation error: %v", err)
	}
	if b.calls != 0 {
		t.Fatal("cancelled request reached desktop")
	}
}

func TestSwitchingBackToAnEarlierApplication(t *testing.T) {
	b := &fakeBackend{}
	c := New(b)
	for _, pid := range []int{42, 99, 42} {
		if _, err := c.Call(context.Background(), "chat", "switch_app", Request{PID: pid}); err != nil {
			t.Fatal(err)
		}
	}
	if b.calls != 3 || b.last.PID != 42 {
		t.Fatal("switching back to an earlier app was lost")
	}
}

func TestMixedScaleDisplayCoordinates(t *testing.T) {
	primary := Display{ID: 2, Width: 2560, Height: 1440, Primary: true}
	left := Display{ID: 1, X: -1920, Y: 199, Width: 1920, Height: 1080}
	above := Display{ID: 3, X: 800, Y: -900, Width: 1440, Height: 900}
	for _, tc := range []struct {
		display                           Display
		width, height, x, y, wantX, wantY int
	}{
		{primary, 1440, 810, 720, 405, 1280, 720},
		{left, 1440, 810, 720, 405, -960, 739},
		{above, 1440, 900, 720, 450, 1520, -450},
		{Display{ID: 4, X: -800, Width: 800, Height: 600}, 1440, 1080, 1439, 1079, -1, 599},
	} {
		b := &fakeBackend{screens: map[uint32]Screen{tc.display.ID: {
			Display: tc.display, Width: tc.width, Height: tc.height, DesktopWidth: tc.display.Width, DesktopHeight: tc.display.Height,
		}}}
		c := New(b)
		out, err := c.Call(context.Background(), "chat", "observe", Request{DisplayID: tc.display.ID})
		if err != nil {
			t.Fatal(err)
		}
		var observation struct {
			ID string `json:"observation_id"`
		}
		if err := json.Unmarshal([]byte(out.Text), &observation); err != nil {
			t.Fatal(err)
		}
		_, err = c.Call(context.Background(), "chat", "click", Request{ObservationID: observation.ID, X: &tc.x, Y: &tc.y, DisplayID: 99})
		if err != nil || *b.last.X != tc.wantX || *b.last.Y != tc.wantY || *b.last.Display != tc.display {
			t.Fatalf("display %d: wrong click target %+v, %v", tc.display.ID, b.last, err)
		}
	}
}
