package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// The recognizer has returned before this function runs. Publish its result
// immediately, but do not acknowledge durability until the synchronous save
// succeeds. A disconnected client does not cancel the requested archive save.
func finishSTTResponse(w http.ResponseWriter, resp *STTTranscribeResponse, stream bool, started time.Time, save func() error) {
	if resp.Timings == nil {
		resp.Timings = map[string]float64{}
	}
	emit := func(kind string, result any) {
		data, _ := json.Marshal(map[string]any{"type": kind, "result": result})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		recognized := *resp
		recognized.Stage = "recognized"
		recognized.Saved = false
		emit("stt.recognized", recognized)
	}
	if err := save(); err != nil {
		if stream {
			data, _ := json.Marshal(map[string]any{"type": "stt.error", "message": "Transcript could not be saved: " + err.Error(), "retryable": true})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		} else {
			http.Error(w, "Transcript could not be saved: "+err.Error(), 500)
		}
		return
	}
	resp.Timings["server_ms"] = float64(time.Since(started).Microseconds()) / 1000
	if stream {
		emit("stt.saved", resp)
	} else {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
