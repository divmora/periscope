package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/store"
)

// Healthz is a liveness probe indicating the HTTP server is running
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		return
	}
}

// Readyz is a readiness probe indicating the application is ready to receive traffic (e.g., DB is connected)
func Readyz(w http.ResponseWriter, r *http.Request) {
	if err := store.Pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("Service Unavailable: Database disconnected\n")); err != nil {
			return
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		return
	}
}
