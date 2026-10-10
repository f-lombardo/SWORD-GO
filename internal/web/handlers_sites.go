package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

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

		installScriptURL := absoluteURL(r, "/public/sites/"+parts[0]+"/scripts/install?token="+site.InstallToken)
		deleteScriptURL := absoluteURL(r, "/public/sites/"+parts[0]+"/scripts/delete?token="+site.InstallToken)
		callbackURL := absoluteURL(r, "/public/sites/"+parts[0]+"/callbacks/install?signature="+site.CallbackSignature)

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
