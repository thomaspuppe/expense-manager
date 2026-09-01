// Package server wires configuration, the store, auth, and HTTP routing together.
package server

import (
	"net/http"

	"expensemanager/internal/api"
	"expensemanager/internal/auth"
	"expensemanager/internal/config"
	"expensemanager/internal/store"
)

// New builds the HTTP handler for the application: public login/health routes,
// the session-or-bearer-gated JSON API, and the session-gated browser app.
func New(cfg config.Config, st *store.Store) http.Handler {
	a := auth.New(cfg)
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /login", loginPage)
	mux.HandleFunc("POST /login", a.Login)
	mux.HandleFunc("POST /logout", a.Logout)

	// JSON API — valid session cookie or bearer token.
	apiMux := http.NewServeMux()
	api.New(st).Register(apiMux)
	mux.Handle("/api/", a.GateAPI(apiMux))

	// Browser app — session-gated. U5/U8 replace the placeholder with the
	// embedded numpad entry and review surfaces.
	mux.Handle("/", a.GateBrowser(http.HandlerFunc(appPlaceholder)))

	return mux
}

func appPlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Expenses</title>
<p>Authenticated. Entry screen arrives in U5.</p>
<form method="post" action="/logout"><button>Log out</button></form>`))
}

func loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := ""
	if r.URL.Query().Get("error") != "" {
		msg = `<p style="color:#c00">Wrong password.</p>`
	}
	w.Write([]byte(`<!doctype html><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Log in</title>` + msg + `
<form method="post" action="/login">
  <input type="password" name="password" placeholder="Password" autofocus>
  <button type="submit">Log in</button>
</form>`))
}
