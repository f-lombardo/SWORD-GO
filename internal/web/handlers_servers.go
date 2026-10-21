package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"sword-go/internal/app/backups"
	"sword-go/internal/app/servers"
	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"
	"sword-go/internal/cloud/hostinger"
)

type serverOption struct {
	Value string
	Label string
}

func (a *App) serverOptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	provider := r.Form.Get("provider")
	apiKey := strings.TrimSpace(r.Form.Get("api_token"))
	if apiKey == "" {
		var err error
		apiKey, err = a.providerAPIToken(r, provider)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	region := strings.TrimSpace(r.Form.Get("region"))
	var regions, serverTypes, images []serverOption
	if apiKey != "" {
		switch provider {
		case "digital_ocean":
			provider := digitalocean.NewProvider(http.DefaultClient)
			items, err := provider.ListAvailableRegions(r.Context(), apiKey)
			if err == nil {
				for _, item := range items {
					regions = append(regions, serverOption{Value: item.Slug, Label: item.Name})
				}
			}
			if region != "" {
				items, err := provider.ListSizes(r.Context(), apiKey)
				if err == nil {
					selectedRegion := digitalocean.Region{Slug: region}
					for _, item := range selectedRegion.ListAvailable(items) {
						serverTypes = append(serverTypes, serverOption{
							Value: item.Slug,
							Label: fmt.Sprintf("%d MB RAM, %d vCPUs, %d GB disk, %s", item.Memory, item.VCPUs, item.Disk, item.ProcessorType()),
						})
					}
				}
			}
			imageItems, err := provider.ListAvailableImages(r.Context(), apiKey)
			if err == nil {
				for _, item := range imageItems {
					images = append(images, serverOption{Value: cmpOr(item.Slug, fmt.Sprintf("%d", item.ID))})
				}
			}
		case "hetzner":
			provider := hetzner.NewProvider(http.DefaultClient)
			locations, err := provider.ListAvailableLocations(r.Context(), apiKey)
			if err == nil {
				for _, item := range locations {
					regions = append(regions, serverOption{Value: item.Name, Label: item.Description})
				}
			}
			if region != "" {
				types, typeErr := provider.ListServerTypes(r.Context(), apiKey)
				if typeErr == nil {
					for _, location := range locations {
						if location.Name == region {
							for _, item := range location.ListAvailable(types) {
								serverTypes = append(serverTypes, serverOption{
									Value: item.Name,
									Label: fmt.Sprintf("%s; %d vCPUs, %g GB RAM, %d GB disk", item.Description, item.Cores, item.Memory, item.Disk),
								})
							}
							break
						}
					}
				}
			}
			imageItems, err := provider.ListAvailableImages(r.Context(), apiKey)
			if err == nil {
				for _, item := range imageItems {
					if item.Name != nil && *item.Name != "" {
						images = append(images, serverOption{Value: *item.Name})
					} else {
						images = append(images, serverOption{Value: fmt.Sprintf("%d", item.ID)})
					}
				}
			}
		case "hostinger":
			provider := hostinger.NewProvider(http.DefaultClient)
			dataCenters, err := provider.ListDataCenters(r.Context(), apiKey)
			if err == nil {
				for _, item := range dataCenters {
					regions = append(regions, serverOption{Value: strconv.Itoa(item.ID), Label: fmt.Sprintf("%s, %s (%s)", item.City, item.Continent, item.Name)})
				}
			}
			catalog, err := provider.ListVPSCatalog(r.Context(), apiKey)
			if err == nil {
				for _, item := range catalog {
					for _, price := range item.Prices {
						serverTypes = append(serverTypes, serverOption{Value: price.ID, Label: fmt.Sprintf("%s — %s %d.%02d / %d %s", item.Name, price.Currency, price.Price/100, price.Price%100, price.Period, price.PeriodUnit)})
					}
				}
			}
			templates, err := provider.ListTemplates(r.Context(), apiKey)
			if err == nil {
				for _, item := range templates {
					images = append(images, serverOption{Value: strconv.Itoa(item.ID), Label: cmpOr(item.Description, item.Name)})
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<div id="server-options"><datalist id="region-options">`)
	for _, option := range regions {
		writeServerOption(w, option)
	}
	_, _ = fmt.Fprint(w, `</datalist><datalist id="server-type-options">`)
	for _, option := range serverTypes {
		writeServerOption(w, option)
	}
	_, _ = fmt.Fprint(w, `</datalist><datalist id="image-options">`)
	for _, option := range images {
		writeServerOption(w, option)
	}
	_, _ = fmt.Fprint(w, `</datalist></div>`)
}

func writeServerOption(w http.ResponseWriter, option serverOption) {
	if option.Label == "" {
		_, _ = fmt.Fprintf(w, `<option value="%s"></option>`, template.HTMLEscapeString(option.Value))
		return
	}
	_, _ = fmt.Fprintf(w, `<option value="%s" label="%s"></option>`, template.HTMLEscapeString(option.Value), template.HTMLEscapeString(option.Label))
}

func (a *App) providerAPIToken(r *http.Request, provider string) (string, error) {
	if provider == "" {
		return "", nil
	}

	integrationsList, err := a.integrationsService.ListByProvider(r.Context(), provider)
	if err != nil {
		return "", err
	}
	if len(integrationsList) == 0 {
		return "", nil
	}

	integration := integrationsList[0]
	if value := strings.TrimSpace(r.Form.Get("integration_id")); value != "" {
		id, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || id < 1 {
			return "", errors.New("invalid integration")
		}
		integration, err = a.integrationsService.GetByID(r.Context(), id)
		if err != nil {
			return "", err
		}
		if integration.Provider != provider {
			return "", errors.New("the selected integration does not match the provider")
		}
	}
	if integration.Credentials.Type != "api_token" || strings.TrimSpace(integration.Credentials.Token) == "" {
		return "", errors.New("the selected integration does not have an API token")
	}
	return integration.Credentials.Token, nil
}

func (a *App) serversIndexCreateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		serversList, err := a.serversService.ListServers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		integrationsList, err := a.integrationsService.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.renderTemplate(w, r, "servers_index.html", map[string]any{"Servers": serversList, "Integrations": integrationsList})
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

		apiToken := strings.TrimSpace(r.Form.Get("api_token"))
		if apiToken == "" {
			resolvedToken, tokenErr := a.providerAPIToken(r, strings.TrimSpace(r.Form.Get("provider")))
			if tokenErr != nil {
				http.Error(w, tokenErr.Error(), http.StatusBadRequest)
				return
			}
			apiToken = resolvedToken
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
			APIToken:   apiToken,
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

		scriptURL := a.absoluteURL(r, "/public/servers/"+parts[0]+"/scripts/provision?token="+server.ProvisionToken)
		callbackURL := a.absoluteURL(r, "/public/servers/"+parts[0]+"/callbacks/provision?signature="+server.CallbackSignature)
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
