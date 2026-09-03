package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"expensemanager/internal/config"
	"expensemanager/internal/store"
)

const (
	testPassword = "swordfish"
	testToken    = "secret-token"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(config.Config{
		Password:      testPassword,
		BearerToken:   testToken,
		SessionSecret: "signing-key",
	}, st)
}

func get(t *testing.T, h http.Handler, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie logs in through the real handler, so the test exercises the
// wired-up login route rather than minting a cookie behind its back.
func sessionCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	form := url.Values{"password": {testPassword}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "em_session" && c.Value != "" {
			return c
		}
	}
	t.Fatal("login set no session cookie")
	return nil
}

// TestBrowserRoutesAreGated is the test that would catch someone dropping the
// GateBrowser wrapper: auth_test proves the gate works, this proves it is on.
func TestBrowserRoutesAreGated(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{"/", "/review"} {
		rec := get(t, h, path, nil)
		if rec.Code != http.StatusFound {
			t.Errorf("GET %s anonymous = %d, want 302", path, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Errorf("GET %s redirected to %q, want /login", path, loc)
		}
	}
}

// TestAPIRoutesAreGated is the same guarantee for the JSON API, which must fail
// closed with a 401 rather than a redirect so non-browser clients see the error.
func TestAPIRoutesAreGated(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{"/api/expenses", "/api/summary", "/api/categories"} {
		rec := get(t, h, path, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s anonymous = %d, want 401", path, rec.Code)
		}
	}
}

func TestPublicRoutesNeedNoSession(t *testing.T) {
	h := testServer(t)
	tests := []struct {
		path        string
		wantBody    string
		wantCTParts string
	}{
		{"/healthz", "ok", "text/plain"},
		{"/login", "<form", "text/html"},
		{"/assets/style.css", "", "text/css"},
		{"/assets/icon.svg", "", "image/svg+xml"},
	}
	for _, tc := range tests {
		rec := get(t, h, tc.path, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, rec.Code)
			continue
		}
		if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("GET %s body missing %q", tc.path, tc.wantBody)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tc.wantCTParts) {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, ct, tc.wantCTParts)
		}
	}
}

// TestServiceWorkerServedFromRoot guards the PWA: a service worker may only
// control paths at or below its own URL, so serving it from /assets/ would
// silently leave the app uncontrolled and offline support dead.
func TestServiceWorkerServedFromRoot(t *testing.T) {
	h := testServer(t)
	rec := get(t, h, "/sw.js", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /sw.js = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want a javascript type", ct)
	}
	if !strings.Contains(rec.Body.String(), "addEventListener") {
		t.Error("body does not look like the service worker")
	}
}

// TestManifestServedWithItsMIMEType covers the init() registration; browsers
// ignore a manifest served as text/plain and the install prompt never appears.
func TestManifestServedWithItsMIMEType(t *testing.T) {
	h := testServer(t)
	rec := get(t, h, "/assets/manifest.webmanifest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET manifest = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/manifest+json") {
		t.Errorf("Content-Type = %q, want application/manifest+json", ct)
	}
}

func TestSessionUnlocksBrowserPages(t *testing.T) {
	h := testServer(t)
	c := sessionCookie(t, h)
	for _, path := range []string{"/", "/review"} {
		rec := get(t, h, path, c)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s with session = %d, want 200", path, rec.Code)
			continue
		}
		// Both templates render the fixed category set into the initial HTML.
		if !strings.Contains(rec.Body.String(), "Groceries") {
			t.Errorf("GET %s did not render the categories", path)
		}
	}
}

func TestBearerTokenUnlocksAPI(t *testing.T) {
	h := testServer(t)
	req := httptest.NewRequest("GET", "/api/categories", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/categories with bearer = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "groceries") {
		t.Error("response did not contain the category set")
	}
}

// TestUnknownPathReturns404 reaches entryPage's fallback, which exists because
// "/" is a catch-all pattern: without it every stray URL would render the app.
func TestUnknownPathReturns404(t *testing.T) {
	h := testServer(t)
	rec := get(t, h, "/no-such-page", sessionCookie(t, h))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /no-such-page = %d, want 404", rec.Code)
	}
}

// TestUnknownPathStillGated checks the 404 above does not leak path existence
// to anonymous callers: they are redirected before the handler runs.
func TestUnknownPathStillGated(t *testing.T) {
	h := testServer(t)
	rec := get(t, h, "/no-such-page", nil)
	if rec.Code != http.StatusFound {
		t.Errorf("GET /no-such-page anonymous = %d, want 302", rec.Code)
	}
}
