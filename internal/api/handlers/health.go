package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/store"
)

// Healthz is a liveness probe indicating the HTTP server is running
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Readyz is a readiness probe indicating the application is ready to receive traffic (e.g., DB is connected)
func Readyz(w http.ResponseWriter, r *http.Request) {
	if err := store.Pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable: Database disconnected\n"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
