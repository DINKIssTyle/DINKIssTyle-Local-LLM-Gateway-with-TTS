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
		{lang: "ko", want: 1000},
		{lang: "ja", want: 1000},
		{lang: "en", want: 1000},
		{lang: "de", want: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := ttsSafetyChunkLimit(tt.lang); got != tt.want {
				t.Fatalf("ttsSafetyChunkLimit(%q) = %d, want %d", tt.lang, got, tt.want)
			}
		})
	}
}

func TestNormalizeTTSThreads(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: -1, want: 4},
		{input: 0, want: 4},
		{input: 1, want: 1},
		{input: 2, want: 2},
		{input: 4, want: 4},
		{input: 5, want: 4},
		{input: 16, want: 4},
	}

	for _, tt := range tests {
		if got := normalizeTTSThreads(tt.input); got != tt.want {
			t.Fatalf("normalizeTTSThreads(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeTTSSpeed(t *testing.T) {
	tests := []struct {
		input float32
		want  float32
	}{
		{input: -1, want: 0.9},
		{input: 0, want: 0.9},
		{input: 0.5, want: 0.7},
		{input: 0.7, want: 0.7},
		{input: 0.9, want: 0.9},
		{input: 1.05, want: 1.05},
		{input: 2.0, want: 2.0},
		{input: 3.0, want: 2.0},
	}

	for _, tt := range tests {
		if got := normalizeTTSSpeed(tt.input); got != tt.want {
			t.Fatalf("normalizeTTSSpeed(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestChunkTextHonorsJapaneseSafetyLimit(t *testing.T) {
	text := ""
	for i := 0; i < 1001; i++ {
		text += "あ"
	}

	chunks := chunkText(text, ttsSafetyChunkLimit("ja"))
	if len(chunks) != 2 {
		t.Fatalf("chunkText returned %d chunks, want 2", len(chunks))
	}
	if got := len([]rune(chunks[0])); got > 1000 {
		t.Fatalf("first chunk has %d runes, want at most 1000", got)
	}
}

func TestChunkTextKeepsPunctuationInsideParagraph(t *testing.T) {
	text := "첫 문장입니다. 둘째 문장도 이어집니다, 쉼표 뒤도 같은 문단입니다. 마지막 문장입니다!"
	chunks := chunkText(text, 300)
	if len(chunks) != 1 {
		t.Fatalf("chunkText returned %d chunks, want one paragraph chunk: %#v", len(chunks), chunks)
	}
	if chunks[0] != text {
		t.Fatalf("chunkText changed paragraph: %q", chunks[0])
	}
}

func TestChunkTextSplitsOnlyAtParagraphBoundaryWithinLimit(t *testing.T) {
	text := "첫 문단입니다. 문장이 여러 개여도 유지합니다.\n\n둘째 문단입니다, 쉼표도 유지합니다."
	chunks := chunkText(text, 300)
	if len(chunks) != 2 {
		t.Fatalf("chunkText returned %d chunks, want 2 paragraphs: %#v", len(chunks), chunks)
	}
	if chunks[0] != "첫 문단입니다. 문장이 여러 개여도 유지합니다." {
		t.Fatalf("unexpected first paragraph: %q", chunks[0])
	}
	if chunks[1] != "둘째 문단입니다, 쉼표도 유지합니다." {
		t.Fatalf("unexpected second paragraph: %q", chunks[1])
	}
}

func TestChunkTextHardWrapsLongParagraphAtWhitespace(t *testing.T) {
	text := "하나 둘 셋 넷 다섯 여섯 일곱 여덟"
	chunks := chunkText(text, 10)
	if len(chunks) < 2 {
		t.Fatalf("chunkText returned %d chunks, want a hard wrap: %#v", len(chunks), chunks)
	}
	for _, chunk := range chunks {
		if got := len([]rune(chunk)); got > 10 {
			t.Fatalf("chunk %q has %d runes, want at most 10", chunk, got)
		}
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
