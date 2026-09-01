// Package auth gates the app for its single owner: a password-backed session
// cookie for the browser/PWA, and a static bearer token for non-browser clients
// on /api/* (KTD4).
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expensemanager/internal/config"
)

const (
	cookieName    = "em_session"
	sessionMaxAge = 30 * 24 * time.Hour
)

// Auth holds the credentials and signing key.
type Auth struct {
	password string
	token    string
	secret   []byte
}

// New builds an Auth from config.
func New(cfg config.Config) *Auth {
	return &Auth{
		password: cfg.Password,
		token:    cfg.BearerToken,
		secret:   []byte(cfg.SessionSecret),
	}
}

// --- session token: "<issuedUnix>.<hex hmac>" ---

func (a *Auth) sign(msg string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *Auth) newSessionValue() string {
	msg := strconv.FormatInt(time.Now().Unix(), 10)
	return msg + "." + a.sign(msg)
}

func (a *Auth) validSessionValue(value string) bool {
	msg, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	expected := a.sign(msg)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return false
	}
	issued, err := strconv.ParseInt(msg, 10, 64)
	if err != nil {
		return false
	}
	return time.Since(time.Unix(issued, 0)) < sessionMaxAge
}

func (a *Auth) hasValidSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return a.validSessionValue(c.Value)
}

func (a *Auth) hasValidBearer(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	tok, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(a.token)) == 1
}

// isSecure reports whether the client connection is HTTPS, accounting for a
// terminating proxy (Caddy sets X-Forwarded-Proto), so the Secure cookie flag is
// set in production but not on plain-HTTP local dev.
func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// --- handlers ---

// Login checks the posted password and, on success, sets the session cookie and
// redirects to the app root. On failure it redirects back to the login page.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    a.newSessionValue(),
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// GateBrowser requires a valid session for a browser route, redirecting to the
// login page when absent.
func (a *Auth) GateBrowser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.hasValidSession(r) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GateAPI requires either a valid session cookie or a matching bearer token,
// returning 401 otherwise.
func (a *Auth) GateAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.hasValidSession(r) || a.hasValidBearer(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	})
}
