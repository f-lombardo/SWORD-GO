package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sword-go/internal/app/servers"
)

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/login", a.loginHandler)
	mux.HandleFunc("/logout", a.logoutHandler)

	mux.Handle("/servers", a.requireAuth(http.HandlerFunc(a.serversIndexCreateHandler)))
	mux.Handle("/servers/", a.requireAuth(http.HandlerFunc(a.serversDetailHandler)))
	mux.Handle("/servers/generate-name", a.requireAuth(http.HandlerFunc(a.generateNameHandler)))

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
	serverID, err := a.parseServerID(parts[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
