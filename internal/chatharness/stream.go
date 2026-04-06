package chatharness

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type SSEEmitter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	active  bool
}

func NewSSEEmitter(w http.ResponseWriter) (*SSEEmitter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	return &SSEEmitter{
		w:       w,
		flusher: flusher,
		active:  true,
	}, nil
}

func (e *SSEEmitter) SetupHeaders() {
	e.w.Header().Set("Content-Type", "text/event-stream")
	e.w.Header().Set("Cache-Control", "no-cache")
	e.w.Header().Set("Connection", "keep-alive")
	e.w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (e *SSEEmitter) Active() bool {
	return e != nil && e.active
}

func (e *SSEEmitter) EmitRaw(payload string) error {
	if e == nil || !e.active {
		return nil
	}
	if _, err := fmt.Fprintf(e.w, "%s\n\n", payload); err != nil {
		e.active = false
		return err
	}
	e.flusher.Flush()
	return nil
}

func (e *SSEEmitter) EmitDataJSON(payload interface{}) error {
	if payload == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return e.EmitRaw(fmt.Sprintf("data: %s", string(data)))
}

func (e *SSEEmitter) SendError(msg string) {
	payload := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]string{
					"content": "\n\n❌ **Error:** " + msg + "\n",
				},
			},
		},
	}
	_ = e.EmitDataJSON(payload)
	if e != nil && e.active {
		_, _ = fmt.Fprintf(e.w, "event: error\ndata: %s\n\n", msg)
		e.flusher.Flush()
	}
}
