package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "sword_go_session"
const csrfCookieName = "sword_go_csrf"
const sessionTTL = 24 * time.Hour

type authService struct {
	adminEmail    string
	adminPassword string
	secret        []byte
	secureCookies bool
}

func newAuthService(adminEmail string, adminPassword string, secret string, secureCookies bool) *authService {
	return &authService{
		adminEmail:    strings.TrimSpace(adminEmail),
		adminPassword: adminPassword,
		secret:        []byte(secret),
		secureCookies: secureCookies,
	}
}

func (a *authService) authenticate(email string, password string) bool {
	return strings.EqualFold(strings.TrimSpace(email), a.adminEmail) && password == a.adminPassword
}

func (a *authService) setSession(w http.ResponseWriter) {
	issuedAt := time.Now().UTC().Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(a.adminEmail + "|" + strconv.FormatInt(issuedAt, 10)))
	signature := a.sign(payload)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    payload + "." + signature,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func (a *authService) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (a *authService) isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}

	payload := parts[0]
	signature := parts[1]
	if !hmac.Equal([]byte(signature), []byte(a.sign(payload))) {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return false
	}

	values := strings.Split(string(decoded), "|")
	if len(values) != 2 {
		return false
	}

	issuedAt, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return false
	}
	issuedAtTime := time.Unix(issuedAt, 0).UTC()
	now := time.Now().UTC()
	if issuedAtTime.After(now.Add(5 * time.Minute)) {
		return false
	}
	if now.Sub(issuedAtTime) > sessionTTL {
		return false
	}

	return strings.EqualFold(values[0], a.adminEmail)
}

func (a *authService) sign(payload string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.auth.isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *authService) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return cookie.Value
	}

	token, err := randomToken()
	if err != nil {
		token = a.sign(strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	return token
}

func (a *authService) verifyCSRFToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false
	}

	submitted := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if submitted == "" {
		_ = r.ParseForm()
		submitted = strings.TrimSpace(r.FormValue("csrf_token"))
	}

	if submitted == "" {
		return false
	}

	if len(submitted) != len(cookie.Value) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(submitted), []byte(cookie.Value)) == 1
}

func (a *App) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if !a.auth.verifyCSRFToken(r) {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func randomToken() (string, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	extra := make([]byte, 8)
	binary.BigEndian.PutUint64(extra, uint64(time.Now().UTC().UnixNano()))
	token := append(seed, extra...)
	return base64.RawURLEncoding.EncodeToString(token), nil
}
