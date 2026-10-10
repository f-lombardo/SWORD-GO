package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

func (a *App) publicServersHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/public/servers/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	serverID, err := parsePositiveID(parts[0])
	if err != nil {
		http.Error(w, "invalid server id", http.StatusBadRequest)
		return
	}

	if parts[1] == "scripts" && parts[2] == "provision" && r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		server, getErr := a.serversService.GetServerByProvisionToken(r.Context(), serverID, token)
		if getErr != nil {
			if errors.Is(getErr, servers.ErrNotFound) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		callbackURL := "/public/servers/" + parts[0] + "/callbacks/provision?signature=" + server.CallbackSignature
		callbackURL = absoluteURL(r, callbackURL)
		script, renderErr := servers.RenderProvisionScript(servers.ProvisionScriptInput{
			Server:      server,
			CallbackURL: callbackURL,
		})
		if renderErr != nil {
			http.Error(w, renderErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/x-shellscript")
		_, _ = w.Write([]byte(script))
		return
	}

	if parts[1] == "callbacks" && parts[2] == "provision" && r.Method == http.MethodPost {
		signature := r.URL.Query().Get("signature")
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if a.callbackGuard == nil {
			http.Error(w, "callback guard unavailable", http.StatusServiceUnavailable)
			return
		}
		if guardErr := a.callbackGuard.Validate("server_provision", serverID, r.Form.Get("ts"), r.Form.Get("nonce"), time.Now()); guardErr != nil {
			http.Error(w, "invalid callback replay token", http.StatusForbidden)
			return
		}

		updateErr := a.serversService.MarkProvisionProgress(r.Context(), serverID, signature, r.Form.Get("status"), r.Form.Get("step"))
		if updateErr != nil {
			if errors.Is(updateErr, servers.ErrNotFound) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, updateErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	http.NotFound(w, r)
}

func (a *App) publicSitesHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/public/sites/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	siteID, err := parsePositiveID(parts[0])
	if err != nil {
		http.Error(w, "invalid site id", http.StatusBadRequest)
		return
	}

	if parts[1] == "scripts" && parts[2] == "install" && r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		site, getErr := a.sitesService.GetSiteByInstallToken(r.Context(), siteID, token)
		if getErr != nil {
			if errors.Is(getErr, sites.ErrNotFound) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		server, getErr := a.serversService.GetServer(r.Context(), site.ServerID)
		if getErr != nil {
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		callbackURL := "/public/sites/" + parts[0] + "/callbacks/install?signature=" + site.CallbackSignature
		callbackURL = absoluteURL(r, callbackURL)
		script, renderErr := sites.RenderInstallScript(sites.InstallScriptInput{
			Site:             site,
			Server:           server,
			CallbackURL:      callbackURL,
			AdminUser:        cmpOr(r.URL.Query().Get("wp_admin_user"), "sword_admin"),
			AdminPassword:    cmpOr(r.URL.Query().Get("wp_admin_password"), randomLowerAlphaNumeric(20)),
			AdminEmail:       cmpOr(r.URL.Query().Get("wp_admin_email"), "admin@example.com"),
			AdminDisplayName: cmpOr(r.URL.Query().Get("wp_admin_display_name"), "SWORD Admin"),
		})
		if renderErr != nil {
			http.Error(w, renderErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/x-shellscript")
		_, _ = w.Write([]byte(script))
		return
	}

	if parts[1] == "scripts" && parts[2] == "delete" && r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		site, getErr := a.sitesService.GetSiteByInstallToken(r.Context(), siteID, token)
		if getErr != nil {
			if errors.Is(getErr, sites.ErrNotFound) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		server, getErr := a.serversService.GetServer(r.Context(), site.ServerID)
		if getErr != nil {
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		script, renderErr := sites.RenderDeleteScript(sites.DeleteScriptInput{
			Site:   site,
			Server: server,
		})
		if renderErr != nil {
			http.Error(w, renderErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/x-shellscript")
		_, _ = w.Write([]byte(script))
		return
	}

	if parts[1] == "callbacks" && parts[2] == "install" && r.Method == http.MethodPost {
		signature := r.URL.Query().Get("signature")
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if a.callbackGuard == nil {
			http.Error(w, "callback guard unavailable", http.StatusServiceUnavailable)
			return
		}
		if guardErr := a.callbackGuard.Validate("site_install", siteID, r.Form.Get("ts"), r.Form.Get("nonce"), time.Now()); guardErr != nil {
			http.Error(w, "invalid callback replay token", http.StatusForbidden)
			return
		}

		updateErr := a.sitesService.MarkInstallProgress(r.Context(), siteID, signature, r.Form.Get("status"), r.Form.Get("step"))
		if updateErr != nil {
			if errors.Is(updateErr, sites.ErrNotFound) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, updateErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	http.NotFound(w, r)
}

func absoluteURL(r *http.Request, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	scheme := "http"
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	base := fmt.Sprintf("%s://%s", scheme, host)
	parsed, err := url.Parse(path)
	if err != nil {
		return base + path
	}
	return base + parsed.String()
}
