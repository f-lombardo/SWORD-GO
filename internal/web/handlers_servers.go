package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sword-go/internal/app/backups"
	"sword-go/internal/app/servers"
)

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

		scriptURL := absoluteURL(r, "/public/servers/"+parts[0]+"/scripts/provision?token="+server.ProvisionToken)
		callbackURL := absoluteURL(r, "/public/servers/"+parts[0]+"/callbacks/provision?signature="+server.CallbackSignature)
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
