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
		if _, err := runSTTInference(make([]byte, 3200), "pcm", "ko", "strict"); err != nil {
			http.Error(w, "STT warmup failed: "+err.Error(), 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ready": true, "modelId": selectedSTTModelID(), "preview": true})
	}
}
