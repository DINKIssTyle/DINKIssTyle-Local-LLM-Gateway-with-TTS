package core

import "net/http"

// handleServerLiveness checks only this HTTP server. Reconnect polling must not
// query upstream model catalogs or depend on model availability.
func handleServerLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
