package core

import (
	"encoding/json"
	"net/http"
)

func handleSTTWarmup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var err error
		modelID := selectedSTTModelID()
		switch r.URL.Query().Get("engine") {
		case "", "whisper":
			_, err = runSTTInference(make([]byte, 3200), "pcm", "ko", "strict")
		case "qwen3-asr":
			modelID = qwenSTTModelID
			_, err = runQwenSTTInference(make([]byte, 3200), "pcm", "auto", "strict", false)
		default:
			http.Error(w, "unknown local STT engine", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "STT warmup failed: "+err.Error(), 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ready": true, "modelId": modelID, "preview": true})
	}
}
