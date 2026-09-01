// Package server wires configuration, the store, auth, and HTTP routing together.
package server

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"mime"
	"net/http"

	"expensemanager/internal/api"
	"expensemanager/internal/auth"
	"expensemanager/internal/config"
	"expensemanager/internal/store"
	"expensemanager/web"
)

var templates = template.Must(template.ParseFS(web.FS, "*.html"))

// categoriesJSON is the fixed category set marshaled once for injection into the
// review page (its JS needs a key->label/icon map).
var categoriesJSON = mustCategoriesJSON()

func init() {
	// Serve the web app manifest with its correct MIME type.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func mustCategoriesJSON() template.JS {
	b, err := json.Marshal(store.Categories)
	if err != nil {
		panic(err)
	}
	return template.JS(b)
}

// New builds the HTTP handler for the application: public login/health/asset
// routes, the session-or-bearer-gated JSON API, and the session-gated browser app.
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

	// Static assets (public, so the login page can style itself too).
	assets, _ := fs.Sub(web.FS, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))

	// Service worker must be served from root scope to control the whole origin.
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		b, err := web.FS.ReadFile("assets/sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write(b)
	})

	// JSON API — valid session cookie or bearer token.
	apiMux := http.NewServeMux()
	api.New(st).Register(apiMux)
	mux.Handle("/api/", a.GateAPI(apiMux))

	// Browser app — session-gated.
	mux.Handle("GET /review", a.GateBrowser(http.HandlerFunc(reviewPage)))
	mux.Handle("/", a.GateBrowser(http.HandlerFunc(entryPage)))

	return mux
}

// reviewPage renders the month overview, breakdown, history, and edit surfaces.
func reviewPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "review.html", map[string]any{
		"Categories":     store.Categories,
		"CategoriesJSON": categoriesJSON,
	}); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// entryPage renders the numpad entry screen with the categories embedded in the
// initial HTML (KTD3, KTD5), so the grid is present on first paint with no fetch.
func entryPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "index.html", map[string]any{
		"Categories": store.Categories,
	}); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errMsg := ""
	if r.URL.Query().Get("error") != "" {
		errMsg = `<p class="err">Wrong password.</p>`
	}
	w.Write([]byte(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="/assets/style.css">
<title>Log in</title></head>
<body><form class="login" method="post" action="/login">
<strong>Expenses</strong>` + errMsg + `
<input type="password" name="password" placeholder="Password" autofocus autocomplete="current-password">
<button type="submit">Log in</button>
</form></body></html>`))
}
