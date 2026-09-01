// Package server wires configuration, the store, and HTTP routing together.
package server

import (
	"net/http"

	"expensemanager/internal/config"
	"expensemanager/internal/store"
)

// New builds the HTTP handler for the application. Later units register the
// JSON API, auth, and static PWA assets on this mux.
func New(cfg config.Config, st *store.Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})

	return mux
}
