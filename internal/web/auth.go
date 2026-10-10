package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "sword_go_session"
const csrfCookieName = "sword_go_csrf"
const sessionTTL = 24 * time.Hour

type authService struct {
	adminEmail        string
	adminPassword     string
	adminPasswordHash string
	secret            []byte
	secureCookies     bool
	strictMode        bool
}

func newAuthService(adminEmail string, adminPassword string, adminPasswordHash string, secret string, secureCookies bool, strictMode bool) (*authService, error) {
	service := &authService{
		adminEmail:        strings.TrimSpace(adminEmail),
		adminPassword:     adminPassword,
		adminPasswordHash: strings.TrimSpace(adminPasswordHash),
		secret:            []byte(secret),
		secureCookies:     secureCookies,
		strictMode:        strictMode,
	}
	if err := service.validateConfiguration(); err != nil {
		return nil, err
	}
	return service, nil
}

func (a *authService) authenticate(email string, password string) bool {
	if !strings.EqualFold(strings.TrimSpace(email), a.adminEmail) {
		return false
	}
	if a.adminPasswordHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(a.adminPasswordHash), []byte(password)) == nil
	}
	return password == a.adminPassword
}

func (a *authService) validateConfiguration() error {
	if a.adminEmail == "" || !strings.Contains(a.adminEmail, "@") {
		return errors.New("SWORD_GO_ADMIN_EMAIL must be a valid email address")
	}
	if len(a.secret) < 16 {
		return errors.New("SWORD_GO_SESSION_SECRET must be at least 16 characters")
	}
	if a.strictMode && len(a.secret) < 32 {
		return errors.New("SWORD_GO_SESSION_SECRET must be at least 32 characters in strict mode")
	}
	if a.strictMode && string(a.secret) == "change-me-in-env" {
		return errors.New("SWORD_GO_SESSION_SECRET uses the default insecure value")
	}

	if a.adminPasswordHash != "" {
		if _, err := bcrypt.Cost([]byte(a.adminPasswordHash)); err != nil {
			return errors.New("SWORD_GO_ADMIN_PASSWORD_HASH is not a valid bcrypt hash")
		}
		return nil
	}

	if a.adminPassword == "" {
		return errors.New("set SWORD_GO_ADMIN_PASSWORD_HASH (preferred) or SWORD_GO_ADMIN_PASSWORD")
	}
	if a.strictMode {
		return errors.New("SWORD_GO_ADMIN_PASSWORD_HASH is required in strict mode")
	}
	return nil
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
