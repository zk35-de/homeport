package api

import (
	"encoding/json"
	"net/http"
)

// HandleHealth responds with a simple liveness payload.
// GET /api/health → {"status":"ok","version":"dev"}
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "dev",
	})
}
