package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"expensemanager/internal/config"
)

func testAuth() *Auth {
	return New(config.Config{Password: "swordfish", BearerToken: "secret-token", SessionSecret: "signing-key"})
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("served"))
	})
}

func postLogin(a *Auth, password string) *httptest.ResponseRecorder {
	form := url.Values{"password": {password}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Login(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestLoginSetsCookieOnCorrectPassword(t *testing.T) {
	a := testAuth()
	rec := postLogin(a, "swordfish")
	if c := sessionCookie(t, rec); c == nil {
		t.Fatal("expected a session cookie on correct password")
	} else if !c.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	a := testAuth()
	rec := postLogin(a, "wrong")
	if c := sessionCookie(t, rec); c != nil {
		t.Error("wrong password must not set a session cookie")
	}
}

func TestGateBrowserRedirectsWithoutSession(t *testing.T) {
	a := testAuth()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	a.GateBrowser(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 redirect", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestGateBrowserServesWithValidSession(t *testing.T) {
	a := testAuth()
	cookie := sessionCookie(t, postLogin(a, "swordfish"))
	if cookie == nil {
		t.Fatal("setup: no session cookie")
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	a.GateBrowser(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGateAPIRejectsWithoutCredentials(t *testing.T) {
	a := testAuth()
	req := httptest.NewRequest("GET", "/api/expenses", nil)
	rec := httptest.NewRecorder()
	a.GateAPI(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestGateAPIAcceptsBearerToken(t *testing.T) {
	a := testAuth()
	req := httptest.NewRequest("GET", "/api/expenses", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	a.GateAPI(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with valid bearer", rec.Code)
	}
}

func TestGateAPIRejectsWrongBearer(t *testing.T) {
	a := testAuth()
	req := httptest.NewRequest("GET", "/api/expenses", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	a.GateAPI(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with wrong bearer", rec.Code)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	a := testAuth()
	cookie := sessionCookie(t, postLogin(a, "swordfish"))

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	a.Logout(rec, req)

	// The logout response must expire the cookie.
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout should expire the session cookie")
	}
}
