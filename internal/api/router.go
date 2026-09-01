// Package api exposes the JSON HTTP API over the expense store. The web UI and
// any future client (CSV import, MCP/AI) consume this same surface (R15, KD2).
package api

import (
	"encoding/json"
	"net/http"

	"expensemanager/internal/store"
)

// API holds the handler dependencies.
type API struct {
	store *store.Store
}

// New builds an API over the given store.
func New(st *store.Store) *API {
	return &API{store: st}
}

// Register mounts the JSON API routes on mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/expenses", a.createExpense)
	mux.HandleFunc("GET /api/expenses", a.listExpenses)
	mux.HandleFunc("PATCH /api/expenses/{id}", a.updateExpense)
	mux.HandleFunc("DELETE /api/expenses/{id}", a.deleteExpense)
	mux.HandleFunc("GET /api/summary", a.summary)
	mux.HandleFunc("GET /api/categories", a.categories)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
