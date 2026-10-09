package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "sword_go_session"

type authService struct {
	adminEmail    string
	adminPassword string
	secret        []byte
}

func newAuthService(adminEmail string, adminPassword string, secret string) *authService {
	return &authService{
		adminEmail:    strings.TrimSpace(adminEmail),
		adminPassword: adminPassword,
		secret:        []byte(secret),
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
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
}

func (a *authService) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
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
