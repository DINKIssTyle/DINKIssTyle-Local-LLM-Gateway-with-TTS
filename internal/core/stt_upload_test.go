package core

import (
	"bytes"
	"testing"
	"time"
)

func TestSTTUploadDeltaRecoveryAndIsolation(t *testing.T) {
	b := sttUploadBuffer{}
	req := STTTranscribeRequest{SessionID: "s", SegmentID: "x", Engine: "apple", Language: "en"}
	full := []byte{1, 2, 3, 4}
	got, err := b.merge("u", req, full)
	if err != nil || !bytes.Equal(got, full) {
		t.Fatal(got, err)
	}
	req.AudioDelta = true
	req.AudioOffset = 4
	got, err = b.merge("u", req, []byte{5, 6})
	if err != nil || !bytes.Equal(got, []byte{1, 2, 3, 4, 5, 6}) {
		t.Fatal(got, err)
	}
	got, err = b.merge("u", req, []byte{5, 6})
	if err != nil || len(got) != 6 {
		t.Fatal("retry", got, err)
	}
	if _, err = b.merge("another-user", req, []byte{5, 6}); err == nil {
		t.Fatal("owner isolation")
	}
	if _, err = b.merge("u", req, []byte{7, 8}); err == nil {
		t.Fatal("overlap mismatch must resync")
	}
	req.AudioOffset = 8
	if _, err = b.merge("u", req, []byte{7, 8}); err == nil {
		t.Fatal("missing offset must resync")
	}
	req.AudioOffset = 0
	got, err = b.merge("u", req, full)
	if err != nil || !bytes.Equal(got, full) {
		t.Fatal("resync", err)
	}
	for k, e := range b.entries {
		e.touched = time.Now().Add(-time.Minute)
		b.entries[k] = e
	}
	req.AudioOffset = 4
	if _, err = b.merge("u", req, []byte{5, 6}); err == nil {
		t.Fatal("expired snapshot accepted")
	}
	req.AudioOffset = 960128
	if _, err = b.merge("u", req, []byte{1, 2}); err == nil {
		t.Fatal("oversize accepted")
	}
}
func TestSTTUploadBounds(t *testing.T) {
	b := sttUploadBuffer{}
	req := STTTranscribeRequest{SessionID: "s", Engine: "apple"}
	for i := 0; i < 50; i++ {
		req.SegmentID = string(rune('A' + i))
		if _, err := b.merge("u", req, make([]byte, 900000)); err != nil {
			t.Fatal(err)
		}
	}
	size := 0
	for _, e := range b.entries {
		size += len(e.audio)
	}
	if len(b.entries) > 32 || size > 16<<20 {
		t.Fatalf("unbounded %d/%d", len(b.entries), size)
	}
}
