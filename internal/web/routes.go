package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/login", a.loginHandler)
	mux.HandleFunc("/logout", a.logoutHandler)

	mux.Handle("/servers", a.requireAuth(http.HandlerFunc(a.serversIndexCreateHandler)))
	mux.Handle("/servers/", a.requireAuth(http.HandlerFunc(a.serversDetailHandler)))
	mux.Handle("/servers/generate-name", a.requireAuth(http.HandlerFunc(a.generateNameHandler)))
	mux.Handle("/sites", a.requireAuth(http.HandlerFunc(a.sitesIndexCreateHandler)))
	mux.Handle("/sites/", a.requireAuth(http.HandlerFunc(a.sitesDetailHandler)))

	return mux
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.renderTemplate(w, "login.html", map[string]any{"Error": ""})
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

	email := r.Form.Get("email")
	password := r.Form.Get("password")
	if !a.auth.authenticate(email, password) {
		a.renderTemplate(w, "login.html", map[string]any{"Error": "Invalid credentials."})
		return
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

func (a *App) serversIndexCreateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		serversList, err := a.serversService.ListServers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.renderTemplate(w, "servers_index.html", map[string]any{
			"Servers": serversList,
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		sshPort := 22
		if value := strings.TrimSpace(r.Form.Get("ssh_port")); value != "" {
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
				sshPort = parsed
			}
		}

		server, err := a.serversService.CreateServer(r.Context(), servers.CreateRequest{
			Name:       strings.TrimSpace(r.Form.Get("name")),
			Hostname:   strings.TrimSpace(r.Form.Get("hostname")),
			Timezone:   strings.TrimSpace(r.Form.Get("timezone")),
			SSHPort:    sshPort,
			Provider:   strings.TrimSpace(r.Form.Get("provider")),
			ServerType: strings.TrimSpace(r.Form.Get("server_type")),
			Region:     strings.TrimSpace(r.Form.Get("region")),
			Image:      strings.TrimSpace(r.Form.Get("image")),
			IPAddress:  strings.TrimSpace(r.Form.Get("ip_address")),
			APIToken:   strings.TrimSpace(r.Form.Get("api_token")),
		})
		if err != nil {
			serversList, listErr := a.serversService.ListServers(r.Context())
			if listErr != nil {
				http.Error(w, listErr.Error(), http.StatusInternalServerError)
				return
			}
			a.renderTemplate(w, "servers_index.html", map[string]any{
				"Servers": serversList,
				"Error":   err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/servers/"+fmt.Sprintf("%d", server.ID), http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) serversDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/servers/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	serverID, err := parsePositiveID(parts[0])
	if err != nil {
		http.Error(w, "invalid server id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		server, getErr := a.serversService.GetServer(r.Context(), serverID)
		if getErr != nil {
			if errors.Is(getErr, servers.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		scriptURL := "/servers/" + parts[0] + "/scripts/provision?token=" + server.ProvisionToken
		callbackURL := "/servers/" + parts[0] + "/callbacks/provision?signature=" + server.CallbackSignature
		wgetCommand := `wget -qO sword-provision.sh "` + scriptURL + `" && sudo bash sword-provision.sh 2>&1 | tee sword-provision.log`

		a.renderTemplate(w, "servers_show.html", map[string]any{
			"Server":      server,
			"ScriptURL":   scriptURL,
			"CallbackURL": callbackURL,
			"WgetCommand": wgetCommand,
		})
		return
	}

	if len(parts) == 3 && parts[1] == "scripts" && parts[2] == "provision" && r.Method == http.MethodGet {
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

		callbackURL := "/servers/" + parts[0] + "/callbacks/provision?signature=" + server.CallbackSignature
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

	if len(parts) == 3 && parts[1] == "callbacks" && parts[2] == "provision" && r.Method == http.MethodPost {
		signature := r.URL.Query().Get("signature")
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		status := r.Form.Get("status")
		step := r.Form.Get("step")

		updateErr := a.serversService.MarkProvisionProgress(r.Context(), serverID, signature, status, step)
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

func (a *App) sitesIndexCreateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sitesList, err := a.sitesService.ListSites(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		serversList, err := a.serversService.ListServers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		provisionedServers := make([]servers.Server, 0)
		for _, server := range serversList {
			if server.Status == "provisioned" {
				provisionedServers = append(provisionedServers, server)
			}
		}

		a.renderTemplate(w, "sites_index.html", map[string]any{
			"Sites":   sitesList,
			"Servers": provisionedServers,
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		serverID, err := parsePositiveID(r.Form.Get("server_id"))
		if err != nil {
			http.Error(w, "invalid server id", http.StatusBadRequest)
			return
		}

		site, err := a.sitesService.CreateSite(r.Context(), sites.CreateRequest{
			ServerID: serverID,
			Domain:   strings.TrimSpace(r.Form.Get("domain")),
			PHPVer:   strings.TrimSpace(r.Form.Get("php_version")),
			Admin: sites.InstallRequest{
				AdminUser:        strings.TrimSpace(r.Form.Get("wp_admin_user")),
				AdminPassword:    r.Form.Get("wp_admin_password"),
				AdminEmail:       strings.TrimSpace(r.Form.Get("wp_admin_email")),
				AdminDisplayName: strings.TrimSpace(r.Form.Get("wp_admin_display_name")),
			},
		})
		if err != nil {
			sitesList, _ := a.sitesService.ListSites(r.Context())
			serversList, _ := a.serversService.ListServers(r.Context())
			provisionedServers := make([]servers.Server, 0)
			for _, server := range serversList {
				if server.Status == "provisioned" {
					provisionedServers = append(provisionedServers, server)
				}
			}
			a.renderTemplate(w, "sites_index.html", map[string]any{
				"Sites":   sitesList,
				"Servers": provisionedServers,
				"Error":   err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/sites/"+fmt.Sprintf("%d", site.ID), http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) sitesDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sites/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	siteID, err := parsePositiveID(parts[0])
	if err != nil {
		http.Error(w, "invalid site id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		site, getErr := a.sitesService.GetSite(r.Context(), siteID)
		if getErr != nil {
			if errors.Is(getErr, sites.ErrNotFound) {
				http.NotFound(w, r)
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

		installScriptURL := "/sites/" + parts[0] + "/scripts/install?token=" + site.InstallToken
		deleteScriptURL := "/sites/" + parts[0] + "/scripts/delete?token=" + site.InstallToken
		callbackURL := "/sites/" + parts[0] + "/callbacks/install?signature=" + site.CallbackSignature

		a.renderTemplate(w, "sites_show.html", map[string]any{
			"Site":             site,
			"Server":           server,
			"InstallScriptURL": installScriptURL,
			"DeleteScriptURL":  deleteScriptURL,
			"CallbackURL":      callbackURL,
		})
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPost {
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		methodOverride := strings.ToUpper(strings.TrimSpace(r.Form.Get("_method")))
		if methodOverride != "DELETE" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		site, getErr := a.sitesService.GetSite(r.Context(), siteID)
		if getErr != nil {
			if errors.Is(getErr, sites.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
			return
		}

		_ = a.sitesService.TriggerDeleteRemote(r.Context(), site)
		if deleteErr := a.sitesService.DeleteSite(r.Context(), siteID); deleteErr != nil {
			http.Error(w, deleteErr.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/sites", http.StatusSeeOther)
		return
	}

	if len(parts) == 3 && parts[1] == "scripts" && parts[2] == "install" && r.Method == http.MethodGet {
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

		callbackURL := "/sites/" + parts[0] + "/callbacks/install?signature=" + site.CallbackSignature
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

	if len(parts) == 3 && parts[1] == "scripts" && parts[2] == "delete" && r.Method == http.MethodGet {
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

	if len(parts) == 3 && parts[1] == "callbacks" && parts[2] == "install" && r.Method == http.MethodPost {
		signature := r.URL.Query().Get("signature")
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		updateErr := a.sitesService.MarkInstallProgress(
			r.Context(),
			siteID,
			signature,
			r.Form.Get("status"),
			r.Form.Get("step"),
		)
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

func (a *App) generateNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name, hostname, err := servers.GenerateName()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name":     name,
		"hostname": hostname,
	})
}
