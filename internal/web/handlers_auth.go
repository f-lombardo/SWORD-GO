package web

import (
	"net/http"
	"time"
)

func (a *App) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.auth != nil && a.auth.isAuthenticated(r) {
		http.Redirect(w, r, "/servers", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.renderTemplate(w, r, "login.html", map[string]any{"Error": ""})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	clientID := clientIdentifier(r)
	if a.loginLimiter != nil && !a.loginLimiter.Allow(clientID, time.Now()) {
		w.WriteHeader(http.StatusTooManyRequests)
		a.renderTemplate(w, r, "login.html", map[string]any{"Error": "Too many login attempts. Please retry later."})
		return
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")
	if !a.auth.authenticate(email, password) {
		if a.loginLimiter != nil {
			a.loginLimiter.RegisterFailure(clientID, time.Now())
		}
		a.renderTemplate(w, r, "login.html", map[string]any{"Error": "Invalid credentials."})
		return
	}
	if a.loginLimiter != nil {
		a.loginLimiter.Reset(clientID)
	}

	a.auth.setSession(w)
	http.Redirect(w, r, "/servers", http.StatusSeeOther)
}

func (a *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.auth.clearSession(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
