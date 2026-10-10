package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sword-go/internal/app/backups"
)

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
