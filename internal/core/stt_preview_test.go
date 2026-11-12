package core

import "testing"

func TestPreviewDoesNotQueueBehindEngine(t *testing.T) {
	sttLock.Lock()
	defer sttLock.Unlock()
	if _, err := runSTTInference(nil, "pcm", "ko", "strict", true); err != errSTTBusy {
		t.Fatalf("busy preview should return immediately: %v", err)
	}
}
