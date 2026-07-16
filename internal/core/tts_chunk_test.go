package core

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTTSSafetyChunkLimit(t *testing.T) {
	tests := []struct {
		lang string
		want int
	}{
		{lang: "ko", want: 120},
		{lang: "ja", want: 120},
		{lang: "en", want: 300},
		{lang: "de", want: 300},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := ttsSafetyChunkLimit(tt.lang); got != tt.want {
				t.Fatalf("ttsSafetyChunkLimit(%q) = %d, want %d", tt.lang, got, tt.want)
			}
		})
	}
}

func TestChunkTextHonorsJapaneseSafetyLimit(t *testing.T) {
	text := ""
	for i := 0; i < 121; i++ {
		text += "あ"
	}

	chunks := chunkText(text, ttsSafetyChunkLimit("ja"))
	if len(chunks) != 2 {
		t.Fatalf("chunkText returned %d chunks, want 2", len(chunks))
	}
	if got := len([]rune(chunks[0])); got > 120 {
		t.Fatalf("first chunk has %d runes, want at most 120", got)
	}
}

type ttsAcquireResult struct {
	ctx     context.Context
	release func()
	err     error
}

func waitForTTSPending(t *testing.T, scheduler *ttsRequestScheduler, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scheduler.mu.Lock()
		actual := len(scheduler.pending)
		scheduler.mu.Unlock()
		if actual >= count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("scheduler did not reach %d pending requests", count)
}

func TestTTSSchedulerNewSessionSupersedesSameOwner(t *testing.T) {
	scheduler := newTTSRequestScheduler()
	ctx1, release1, err := scheduler.acquire(context.Background(), ttsScheduleMeta{
		Owner: "alice/client-a", SessionID: 1, RequestID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	pendingOld := make(chan ttsAcquireResult, 1)
	go func() {
		ctx, release, err := scheduler.acquire(context.Background(), ttsScheduleMeta{
			Owner: "alice/client-a", SessionID: 1, RequestID: 2,
		})
		pendingOld <- ttsAcquireResult{ctx: ctx, release: release, err: err}
	}()
	waitForTTSPending(t, scheduler, 1)

	newSession := make(chan ttsAcquireResult, 1)
	go func() {
		ctx, release, err := scheduler.acquire(context.Background(), ttsScheduleMeta{
			Owner: "alice/client-a", SessionID: 2, RequestID: 3,
		})
		newSession <- ttsAcquireResult{ctx: ctx, release: release, err: err}
	}()

	select {
	case <-ctx1.Done():
		if !errors.Is(context.Cause(ctx1), errTTSSuperseded) {
			t.Fatalf("unexpected active cancellation cause: %v", context.Cause(ctx1))
		}
	case <-time.After(time.Second):
		t.Fatal("active stale session was not cancelled")
	}

	select {
	case result := <-pendingOld:
		if !errors.Is(result.err, errTTSSuperseded) {
			t.Fatalf("pending stale request error = %v", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending stale request was not rejected")
	}

	release1()
	select {
	case result := <-newSession:
		if result.err != nil {
			t.Fatal(result.err)
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("new playback session was not granted")
	}

	_, _, err = scheduler.acquire(context.Background(), ttsScheduleMeta{
		Owner: "alice/client-a", SessionID: 1, RequestID: 4,
	})
	if !errors.Is(err, errTTSSuperseded) {
		t.Fatalf("late stale session error = %v", err)
	}
}

func TestTTSSchedulerDoesNotCancelAnotherOwner(t *testing.T) {
	scheduler := newTTSRequestScheduler()
	ctx1, release1, err := scheduler.acquire(context.Background(), ttsScheduleMeta{
		Owner: "alice/client-a", SessionID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	second := make(chan ttsAcquireResult, 1)
	go func() {
		ctx, release, err := scheduler.acquire(context.Background(), ttsScheduleMeta{
			Owner: "bob/client-b", SessionID: 99,
		})
		second <- ttsAcquireResult{ctx: ctx, release: release, err: err}
	}()
	waitForTTSPending(t, scheduler, 1)

	select {
	case <-ctx1.Done():
		t.Fatalf("another owner's request cancelled the active session: %v", context.Cause(ctx1))
	default:
	}
	release1()
	select {
	case result := <-second:
		if result.err != nil {
			t.Fatal(result.err)
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("second owner was not granted after release")
	}
}

func TestTTSScheduleMetaFromAuthenticatedRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/tts", nil)
	req.Header.Set("X-User-ID", "alice")
	req.Header.Set("X-TTS-Client-ID", "tab-1")
	req.Header.Set("X-TTS-Session-ID", "12")
	req.Header.Set("X-TTS-Request-ID", "34")
	meta := ttsScheduleMetaFromRequest(req)
	if meta.Owner != "alice\x00tab-1" || meta.SessionID != 12 || meta.RequestID != 34 {
		t.Fatalf("unexpected schedule metadata: %+v", meta)
	}
}
