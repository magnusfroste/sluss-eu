package server

import (
	"encoding/json"
	"net/http"
)

type ReadyzChecker interface {
	Name() string
	Ready() error
}

func HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func ReadyzHandler(checkers ...ReadyzChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		failures := make(map[string]string)
		for _, c := range checkers {
			if err := c.Ready(); err != nil {
				failures[c.Name()] = err.Error()
			}
		}
		if len(failures) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  "not_ready",
				"reasons": failures,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}
