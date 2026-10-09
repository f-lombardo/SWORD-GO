package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sword-go/internal/app/backups"
	cloudflareapp "sword-go/internal/app/cloudflare"
	"sword-go/internal/app/integrations"
	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/login", a.requireCSRF(http.HandlerFunc(a.loginHandler)))
	mux.Handle("/logout", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.logoutHandler))))

	mux.Handle("/public/servers/", http.HandlerFunc(a.publicServersHandler))
	mux.Handle("/public/sites/", http.HandlerFunc(a.publicSitesHandler))

	protected := func(handler http.HandlerFunc) http.Handler {
		return a.requireAuth(a.requireCSRF(handler))
	}

	mux.Handle("/servers", protected(a.serversIndexCreateHandler))
	mux.Handle("/servers/", protected(a.serversDetailHandler))
	mux.Handle("/servers/generate-name", protected(a.generateNameHandler))
	mux.Handle("/sites", protected(a.sitesIndexCreateHandler))
	mux.Handle("/sites/", protected(a.sitesDetailHandler))
	mux.Handle("/backup-destinations", protected(a.backupDestinationsIndexCreateHandler))
	mux.Handle("/backup-destinations/", protected(a.backupDestinationsDetailHandler))
	mux.Handle("/backup-schedules", protected(a.backupSchedulesIndexHandler))
	mux.Handle("/settings/integrations", protected(a.integrationsIndexCreateHandler))
	mux.Handle("/settings/integrations/", protected(a.integrationsDetailHandler))
	mux.Handle("/cloudflare", protected(a.cloudflareIndexHandler))
	mux.Handle("/cloudflare/", protected(a.cloudflareDetailHandler))

	return mux
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

	email := r.Form.Get("email")
	password := r.Form.Get("password")
	if !a.auth.authenticate(email, password) {
		a.renderTemplate(w, r, "login.html", map[string]any{"Error": "Invalid credentials."})
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

		a.renderTemplate(w, r, "servers_index.html", map[string]any{
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
			a.renderTemplate(w, r, "servers_index.html", map[string]any{
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

		scriptURL := "/public/servers/" + parts[0] + "/scripts/provision?token=" + server.ProvisionToken
		callbackURL := "/public/servers/" + parts[0] + "/callbacks/provision?signature=" + server.CallbackSignature
		wgetCommand := `wget -qO sword-provision.sh "` + scriptURL + `" && sudo bash sword-provision.sh 2>&1 | tee sword-provision.log`
		backupSchedules, schedulesErr := a.backupsService.ListSchedulesByServer(r.Context(), server.ID)
		if schedulesErr != nil {
			http.Error(w, schedulesErr.Error(), http.StatusInternalServerError)
			return
		}
		backupRuns, runsErr := a.backupsService.ListRunsByServer(r.Context(), server.ID, 10)
		if runsErr != nil {
			http.Error(w, runsErr.Error(), http.StatusInternalServerError)
			return
		}
		backupDestinations, destinationsErr := a.backupsService.ListDestinations(r.Context())
		if destinationsErr != nil {
			http.Error(w, destinationsErr.Error(), http.StatusInternalServerError)
			return
		}

		a.renderTemplate(w, r, "servers_show.html", map[string]any{
			"Server":             server,
			"ScriptURL":          scriptURL,
			"CallbackURL":        callbackURL,
			"WgetCommand":        wgetCommand,
			"BackupSchedules":    backupSchedules,
			"BackupRuns":         backupRuns,
			"BackupDestinations": backupDestinations,
		})
		return
	}

	if len(parts) == 2 && parts[1] == "backup-schedules" && r.Method == http.MethodPost {
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		backupDestinationID, parseErr := parsePositiveID(r.Form.Get("backup_destination_id"))
		if parseErr != nil {
			http.Error(w, "invalid backup destination id", http.StatusBadRequest)
			return
		}

		frequency := strings.TrimSpace(r.Form.Get("frequency"))
		dayOfWeek := parseOptionalInt(r.Form.Get("day_of_week"))
		dayOfMonth := parseOptionalInt(r.Form.Get("day_of_month"))
		retention := 7
		if value := strings.TrimSpace(r.Form.Get("retention_count")); value != "" {
			if parsed, conversionErr := strconv.Atoi(value); conversionErr == nil {
				retention = parsed
			}
		}

		_, createErr := a.backupsService.CreateSchedule(r.Context(), backups.CreateScheduleInput{
			ServerID:            serverID,
			BackupDestinationID: backupDestinationID,
			Frequency:           frequency,
			Time:                strings.TrimSpace(r.Form.Get("time")),
			DayOfWeek:           dayOfWeek,
			DayOfMonth:          dayOfMonth,
			RetentionCount:      retention,
		})
		if createErr != nil {
			http.Error(w, createErr.Error(), http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/servers/"+parts[0], http.StatusSeeOther)
		return
	}

	if len(parts) == 4 && parts[1] == "backup-schedules" && parts[3] == "run" && r.Method == http.MethodPost {
		scheduleID, parseErr := parsePositiveID(parts[2])
		if parseErr != nil {
			http.Error(w, "invalid backup schedule id", http.StatusBadRequest)
			return
		}
		if runErr := a.backupsService.RunScheduleNow(r.Context(), serverID, scheduleID); runErr != nil {
			if errors.Is(runErr, backups.ErrScheduleNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, runErr.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/servers/"+parts[0], http.StatusSeeOther)
		return
	}

	if len(parts) == 3 && parts[1] == "backup-schedules" && r.Method == http.MethodPost {
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if strings.ToUpper(strings.TrimSpace(r.Form.Get("_method"))) != "DELETE" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		scheduleID, parseErr := parsePositiveID(parts[2])
		if parseErr != nil {
			http.Error(w, "invalid backup schedule id", http.StatusBadRequest)
			return
		}
		deleteErr := a.backupsService.DeleteSchedule(r.Context(), serverID, scheduleID)
		if deleteErr != nil {
			if errors.Is(deleteErr, backups.ErrScheduleNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, deleteErr.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/servers/"+parts[0], http.StatusSeeOther)
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

		a.renderTemplate(w, r, "sites_index.html", map[string]any{
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
			a.renderTemplate(w, r, "sites_index.html", map[string]any{
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

		installScriptURL := "/public/sites/" + parts[0] + "/scripts/install?token=" + site.InstallToken
		deleteScriptURL := "/public/sites/" + parts[0] + "/scripts/delete?token=" + site.InstallToken
		callbackURL := "/public/sites/" + parts[0] + "/callbacks/install?signature=" + site.CallbackSignature

		a.renderTemplate(w, r, "sites_show.html", map[string]any{
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

	http.NotFound(w, r)
}

func (a *App) backupDestinationsIndexCreateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		destinations, err := a.backupsService.ListDestinations(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.renderTemplate(w, r, "backup_destinations_index.html", map[string]any{
			"Destinations": destinations,
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		port := 22
		if value := strings.TrimSpace(r.Form.Get("port")); value != "" {
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
				port = parsed
			}
		}
		created, err := a.backupsService.CreateDestination(r.Context(), backups.CreateDestinationInput{
			Name:          strings.TrimSpace(r.Form.Get("name")),
			Type:          cmpOr(strings.TrimSpace(r.Form.Get("type")), "borg"),
			Host:          strings.TrimSpace(r.Form.Get("host")),
			Port:          port,
			Username:      strings.TrimSpace(r.Form.Get("username")),
			AuthMethod:    strings.TrimSpace(r.Form.Get("auth_method")),
			Password:      r.Form.Get("password"),
			SSHPrivateKey: r.Form.Get("ssh_private_key"),
			StoragePath:   strings.TrimSpace(r.Form.Get("storage_path")),
		})
		if err != nil {
			destinations, _ := a.backupsService.ListDestinations(r.Context())
			a.renderTemplate(w, r, "backup_destinations_index.html", map[string]any{
				"Destinations": destinations,
				"Error":        err.Error(),
			})
			return
		}
		http.Redirect(w, r, "/backup-destinations/"+fmt.Sprintf("%d", created.ID), http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) backupDestinationsDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/backup-destinations/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	destinationID, err := parsePositiveID(path)
	if err != nil {
		http.Error(w, "invalid backup destination id", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		destination, err := a.backupsService.GetDestination(r.Context(), destinationID)
		if err != nil {
			if errors.Is(err, backups.ErrDestinationNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		schedules, _ := a.backupsService.ListSchedules(r.Context())
		a.renderTemplate(w, r, "backup_destinations_show.html", map[string]any{
			"Destination": destination,
			"Schedules":   schedules,
		})
		return
	}

	if r.Method == http.MethodPost {
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		switch strings.ToUpper(strings.TrimSpace(r.Form.Get("_method"))) {
		case "PATCH":
			port := 22
			if value := strings.TrimSpace(r.Form.Get("port")); value != "" {
				if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
					port = parsed
				}
			}
			_, updateErr := a.backupsService.UpdateDestination(r.Context(), destinationID, backups.UpdateDestinationInput{
				Name:          strings.TrimSpace(r.Form.Get("name")),
				Type:          cmpOr(strings.TrimSpace(r.Form.Get("type")), "borg"),
				Host:          strings.TrimSpace(r.Form.Get("host")),
				Port:          port,
				Username:      strings.TrimSpace(r.Form.Get("username")),
				AuthMethod:    strings.TrimSpace(r.Form.Get("auth_method")),
				Password:      r.Form.Get("password"),
				SSHPrivateKey: r.Form.Get("ssh_private_key"),
				StoragePath:   strings.TrimSpace(r.Form.Get("storage_path")),
			})
			if updateErr != nil {
				http.Error(w, updateErr.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/backup-destinations/"+path, http.StatusSeeOther)
			return
		case "DELETE":
			deleteErr := a.backupsService.DeleteDestination(r.Context(), destinationID)
			if deleteErr != nil {
				http.Error(w, deleteErr.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/backup-destinations", http.StatusSeeOther)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (a *App) backupSchedulesIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	schedules, err := a.backupsService.ListSchedules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderTemplate(w, r, "backup_schedules_index.html", map[string]any{
		"Schedules": schedules,
	})
}

func parseOptionalInt(value string) *int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}

func (a *App) integrationsIndexCreateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		integrationsList, err := a.integrationsService.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		masked := make([]integrations.Integration, 0, len(integrationsList))
		for _, integration := range integrationsList {
			integration.Credentials = integrations.MaskCredentials(integration.Credentials)
			masked = append(masked, integration)
		}

		a.renderTemplate(w, r, "integrations_index.html", map[string]any{
			"Integrations": masked,
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		_, err := a.integrationsService.Create(r.Context(), integrations.CreateInput{
			Name:     strings.TrimSpace(r.Form.Get("name")),
			Provider: strings.TrimSpace(r.Form.Get("provider")),
			Credentials: integrations.Credentials{
				Type:  strings.TrimSpace(r.Form.Get("type")),
				Token: r.Form.Get("token"),
				Email: strings.TrimSpace(r.Form.Get("email")),
				Key:   r.Form.Get("key"),
			},
		})
		if err != nil {
			integrationsList, _ := a.integrationsService.List(r.Context())
			a.renderTemplate(w, r, "integrations_index.html", map[string]any{
				"Integrations": integrationsList,
				"Error":        err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/settings/integrations", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) integrationsDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/settings/integrations/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	integrationID, err := parsePositiveID(path)
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err = r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	switch strings.ToUpper(strings.TrimSpace(r.Form.Get("_method"))) {
	case "PATCH":
		_, err = a.integrationsService.Update(r.Context(), integrationID, integrations.UpdateInput{
			Name: strings.TrimSpace(r.Form.Get("name")),
			Credentials: integrations.Credentials{
				Type:  strings.TrimSpace(r.Form.Get("type")),
				Token: r.Form.Get("token"),
				Email: strings.TrimSpace(r.Form.Get("email")),
				Key:   r.Form.Get("key"),
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/settings/integrations", http.StatusSeeOther)
	case "DELETE":
		if err = a.integrationsService.Delete(r.Context(), integrationID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/settings/integrations", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) cloudflareIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	integrationsList, err := a.integrationsService.ListByProvider(r.Context(), "cloudflare")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.renderTemplate(w, r, "cloudflare_index.html", map[string]any{
		"Integrations": integrationsList,
	})
}

func (a *App) cloudflareDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/cloudflare/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	integrationID, err := parsePositiveID(parts[0])
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	integration, err := a.integrationsService.GetByID(r.Context(), integrationID)
	if err != nil {
		if errors.Is(err, integrations.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cfService := cloudflareapp.NewService(&http.Client{Timeout: 30 * time.Second}, integration.Credentials)

	if len(parts) == 1 && r.Method == http.MethodGet {
		zones, err := cfService.GetZones(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.renderTemplate(w, r, "cloudflare_zones.html", map[string]any{
			"Integration": integration,
			"Zones":       zones,
		})
		return
	}

	zoneID := parts[1]

	if len(parts) == 2 && r.Method == http.MethodGet {
		zone, zoneErr := cfService.GetZone(r.Context(), zoneID)
		if zoneErr != nil {
			http.Error(w, zoneErr.Error(), http.StatusInternalServerError)
			return
		}
		records, recordsErr := cfService.GetDNSRecords(r.Context(), zoneID)
		if recordsErr != nil {
			http.Error(w, recordsErr.Error(), http.StatusInternalServerError)
			return
		}
		analytics, analyticsErr := cfService.GetZoneAnalytics(r.Context(), zoneID)
		if analyticsErr != nil {
			http.Error(w, analyticsErr.Error(), http.StatusInternalServerError)
			return
		}
		sslSettings, sslErr := cfService.GetSSLSettings(r.Context(), zoneID)
		if sslErr != nil {
			http.Error(w, sslErr.Error(), http.StatusInternalServerError)
			return
		}

		a.renderTemplate(w, r, "cloudflare_show.html", map[string]any{
			"Integration": integration,
			"ZoneID":      zoneID,
			"ZoneName":    zone["name"],
			"DNSRecords":  records,
			"Analytics":   analytics,
			"SSLSettings": sslSettings,
		})
		return
	}

	if len(parts) == 3 && parts[2] == "purge-cache" && r.Method == http.MethodPost {
		_, err := cfService.PurgeCache(r.Context(), zoneID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/cloudflare/"+parts[0]+"/"+zoneID, http.StatusSeeOther)
		return
	}

	if len(parts) == 3 && parts[2] == "dns-records" && r.Method == http.MethodPost {
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		proxied := strings.TrimSpace(r.Form.Get("proxied")) == "on" || strings.TrimSpace(r.Form.Get("proxied")) == "1"
		ttl := 1
		if value := strings.TrimSpace(r.Form.Get("ttl")); value != "" {
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
				ttl = parsed
			}
		}

		_, err = cfService.UpsertDNSRecord(
			r.Context(),
			zoneID,
			strings.TrimSpace(r.Form.Get("name")),
			strings.TrimSpace(r.Form.Get("type")),
			strings.TrimSpace(r.Form.Get("content")),
			proxied,
			ttl,
			strings.TrimSpace(r.Form.Get("cname_content")),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/cloudflare/"+parts[0]+"/"+zoneID, http.StatusSeeOther)
		return
	}

	if len(parts) == 4 && parts[2] == "dns-records" && r.Method == http.MethodPost {
		recordID := parts[3]
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		methodOverride := strings.ToUpper(strings.TrimSpace(r.Form.Get("_method")))
		switch methodOverride {
		case "PATCH":
			proxied := strings.TrimSpace(r.Form.Get("proxied")) == "on" || strings.TrimSpace(r.Form.Get("proxied")) == "1"
			ttl := 1
			if value := strings.TrimSpace(r.Form.Get("ttl")); value != "" {
				if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
					ttl = parsed
				}
			}
			_, err := cfService.UpdateDNSRecord(
				r.Context(),
				zoneID,
				recordID,
				strings.TrimSpace(r.Form.Get("type")),
				strings.TrimSpace(r.Form.Get("name")),
				strings.TrimSpace(r.Form.Get("content")),
				proxied,
				ttl,
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "DELETE":
			_, err := cfService.DeleteDNSRecord(r.Context(), zoneID, recordID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		http.Redirect(w, r, "/cloudflare/"+parts[0]+"/"+zoneID, http.StatusSeeOther)
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
