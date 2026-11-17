package core

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFinalSTTStreamsRecognitionBeforeDurableSave(t *testing.T) {
	w := httptest.NewRecorder()
	resp := STTTranscribeResponse{Text: "synthetic speech", Stage: "final", Timings: map[string]float64{"engine_ms": 5}}
	finishSTTResponse(w, &resp, true, time.Now(), func() error {
		if !w.Flushed || !strings.Contains(w.Body.String(), `"type":"stt.recognized"`) || strings.Contains(w.Body.String(), `"type":"stt.saved"`) {
			t.Fatal("recognition must be available while save is still pending")
		}
		if resp.Saved {
			t.Fatal("save was acknowledged early")
		}
		resp.Saved = true
		return nil
	})
	body := w.Body.String()
	if !strings.Contains(body, `"type":"stt.saved"`) || strings.Index(body, "stt.saved") < strings.Index(body, "stt.recognized") {
		t.Fatal(body)
	}
	if strings.Contains(strings.Split(body, "\n\n")[0], `"saved":true`) {
		t.Fatal("recognition falsely claimed durability")
	}
}
func TestFinalSTTSaveFailureNeverAcknowledgesDurability(t *testing.T) {
	for _, stream := range []bool{false, true} {
		w := httptest.NewRecorder()
		resp := STTTranscribeResponse{Text: "synthetic", Timings: map[string]float64{}}
		finishSTTResponse(w, &resp, stream, time.Now(), func() error { return errors.New("synthetic save failure") })
		if strings.Contains(w.Body.String(), `"type":"stt.saved"`) || resp.Saved {
			t.Fatal("failed archive acknowledged")
		}
		if stream {
			if !strings.Contains(w.Body.String(), "stt.error") || !strings.Contains(w.Body.String(), "stt.recognized") {
				t.Fatal(w.Body.String())
			}
		} else if w.Code != 500 {
			t.Fatal(w.Code)
		}
	}
}
func TestFinalSTTJSONRemainsSaveConfirmed(t *testing.T) {
	w := httptest.NewRecorder()
	resp := STTTranscribeResponse{Text: "synthetic", Stage: "final", Timings: map[string]float64{}}
	finishSTTResponse(w, &resp, false, time.Now(), func() error {
		if w.Body.Len() != 0 {
			t.Fatal("JSON sent before save")
		}
		resp.Saved = true
		return nil
	})
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" || !strings.Contains(w.Body.String(), `"saved":true`) {
		t.Fatal(w.Body.String())
	}
}
