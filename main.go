// Command expensemanager is a single-user expense tracker: one static binary
// serving a numpad-first PWA and a JSON API over a local SQLite database.
package main

import (
	"log"
	"net/http"

	"expensemanager/internal/config"
	"expensemanager/internal/server"
	"expensemanager/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	handler := server.New(cfg, st)

	log.Printf("expense manager listening on %s (db: %s)", cfg.Addr, cfg.DBPath)
	if err := http.ListenAndServe(cfg.Addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}
