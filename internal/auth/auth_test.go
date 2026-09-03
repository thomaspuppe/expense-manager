package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"expensemanager/internal/config"
)

func testAuth() *Auth {
	a := New(config.Config{Password: "swordfish", BearerToken: "secret-token", SessionSecret: "signing-key"})
	a.throttle.step, a.throttle.max = 0, 0 // don't make the suite wait out the login backoff
	return a
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

// TestLoginThrottleBacksOff covers the brute-force guard on the single account:
// each consecutive failure waits longer, up to a cap, and a success clears it.
func TestLoginThrottleBacksOff(t *testing.T) {
	tr := &loginThrottle{step: time.Millisecond, max: 3 * time.Millisecond, quiet: time.Hour}

	want := []time.Duration{1, 2, 3, 3}
	for i, w := range want {
		if got := tr.penalize(); got != w*time.Millisecond {
			t.Errorf("failure %d delayed %v, want %v", i+1, got, w*time.Millisecond)
		}
	}

	tr.clear()
	if got := tr.penalize(); got != time.Millisecond {
		t.Errorf("after a successful login the backoff restarted at %v, want %v", got, time.Millisecond)
	}
}

// A long quiet gap is the owner mistyping months later, not a run of guesses.
func TestLoginThrottleForgetsAfterQuietPeriod(t *testing.T) {
	tr := &loginThrottle{step: time.Millisecond, max: 3 * time.Millisecond, quiet: time.Hour}
	tr.penalize()
	tr.penalize()
	tr.last = time.Now().Add(-2 * time.Hour)

	if got := tr.penalize(); got != time.Millisecond {
		t.Errorf("delay after quiet period = %v, want %v", got, time.Millisecond)
	}
}

func TestLoginSucceedsAfterFailures(t *testing.T) {
	a := testAuth()
	postLogin(a, "wrong")
	postLogin(a, "wrong")
	rec := postLogin(a, "swordfish")
	if sessionCookie(t, rec) == nil {
		t.Error("correct password must still log in after failed attempts (no lockout)")
	}
}
