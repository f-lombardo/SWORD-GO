package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func mustAuthService(t *testing.T) *authService {
	t.Helper()
	auth, err := newAuthService(
		"admin@example.com",
		"password",
		"",
		"1234567890123456",
		false,
		false,
	)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return auth
}

func TestRequireCSRFIgnoresSafeMethods(t *testing.T) {
	app := &App{auth: mustAuthService(t)}
	called := false
	handler := app.requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/servers", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !called {
		t.Fatalf("expected wrapped handler to be called")
	}
}

func TestRequireCSRFBocksMissingToken(t *testing.T) {
	app := &App{auth: mustAuthService(t)}
	handler := app.requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/servers", strings.NewReader("name=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestRequireCSRFAllowsMatchingFormToken(t *testing.T) {
	app := &App{auth: mustAuthService(t)}
	handler := app.requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	form := url.Values{}
	form.Set("csrf_token", "abc123")
	req := httptest.NewRequest(http.MethodPost, "/servers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "abc123"})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestAuthenticateSupportsBcryptHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("my-strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate bcrypt hash: %v", err)
	}

	auth, err := newAuthService(
		"admin@example.com",
		"",
		string(hash),
		"12345678901234567890123456789012",
		true,
		true,
	)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	if !auth.authenticate("admin@example.com", "my-strong-password") {
		t.Fatalf("expected bcrypt auth to pass")
	}
	if auth.authenticate("admin@example.com", "wrong-password") {
		t.Fatalf("expected bcrypt auth to fail on wrong password")
	}
}

func TestStrictModeRejectsPlaintextPassword(t *testing.T) {
	_, err := newAuthService(
		"admin@example.com",
		"plaintext",
		"",
		"12345678901234567890123456789012",
		true,
		true,
	)
	if err == nil {
		t.Fatalf("expected strict mode to reject plaintext password")
	}
}
