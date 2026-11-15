package core

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

// Bounded, owner-scoped, ephemeral preview buffer. Final requests remain full
// snapshots, preserving transcript durability and compatibility with old apps.
type sttUploadEntry struct {
	audio   []byte
	touched time.Time
}
type sttUploadBuffer struct {
	mu      sync.Mutex
	entries map[string]sttUploadEntry
	timer   *time.Timer
}

var previewUploads = sttUploadBuffer{entries: map[string]sttUploadEntry{}}

func (b *sttUploadBuffer) merge(owner string, req STTTranscribeRequest, incoming []byte) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.entries == nil {
		b.entries = map[string]sttUploadEntry{}
	}
	now := time.Now()
	for key, e := range b.entries {
		if now.Sub(e.touched) > 45*time.Second {
			delete(b.entries, key)
		}
	}
	key := owner + "\x00" + req.SessionID + "\x00" + req.SegmentID + "\x00" + req.Engine + "\x00" + req.Language
	if req.SessionID == "" || req.SegmentID == "" {
		if req.AudioDelta {
			return nil, fmt.Errorf("audio snapshot required")
		}
		return incoming, nil
	}
	if len(incoming)%2 != 0 || req.AudioOffset < 0 || req.AudioOffset%2 != 0 || req.AudioOffset > 960128 || len(incoming) > 960128-req.AudioOffset {
		return nil, fmt.Errorf("invalid audio offset")
	}
	var combined []byte
	if !req.AudioDelta || req.AudioOffset == 0 {
		combined = append([]byte(nil), incoming...)
	} else {
		previous, ok := b.entries[key]
		if !ok || req.AudioOffset > len(previous.audio) {
			return nil, fmt.Errorf("audio snapshot required")
		}
		overlap := len(previous.audio) - req.AudioOffset
		if overlap > len(incoming) {
			overlap = len(incoming)
		}
		if !bytes.Equal(previous.audio[req.AudioOffset:req.AudioOffset+overlap], incoming[:overlap]) {
			return nil, fmt.Errorf("audio snapshot required")
		}
		end := req.AudioOffset + len(incoming)
		combined = append([]byte(nil), previous.audio...)
		if end > len(combined) {
			combined = append(combined, incoming[overlap:]...)
		}
		// An older retry reads its own prefix, without discarding newer bytes.
		b.store(key, combined, now)
		return append([]byte(nil), combined[:end]...), nil
	}
	b.store(key, combined, now)
	return append([]byte(nil), combined...), nil
}

// Caller holds b.mu. Every append participates in the global memory limit.
func (b *sttUploadBuffer) store(key string, audio []byte, now time.Time) {
	delete(b.entries, key)
	total := 0
	for _, e := range b.entries {
		total += len(e.audio)
	}
	for len(b.entries) >= 32 || total+len(audio) > 16<<20 {
		oldest := ""
		var at time.Time
		for k, e := range b.entries {
			if oldest == "" || e.touched.Before(at) {
				oldest = k
				at = e.touched
			}
		}
		if oldest == "" {
			break
		}
		total -= len(b.entries[oldest].audio)
		delete(b.entries, oldest)
	}
	b.entries[key] = sttUploadEntry{audio: audio, touched: now}
	if b.timer == nil {
		b.timer = time.AfterFunc(45*time.Second, b.expire)
	}
}

func (b *sttUploadBuffer) expire() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.timer = nil
	now := time.Now()
	next := 45 * time.Second
	for key, e := range b.entries {
		remaining := 45*time.Second - now.Sub(e.touched)
		if remaining <= 0 {
			delete(b.entries, key)
		} else if remaining < next {
			next = remaining
		}
	}
	if len(b.entries) > 0 {
		b.timer = time.AfterFunc(next, b.expire)
	}
}
