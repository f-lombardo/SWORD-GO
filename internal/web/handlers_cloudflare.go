package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	cloudflareapp "sword-go/internal/app/cloudflare"
	"sword-go/internal/app/integrations"
)

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
		zones, getErr := cfService.GetZones(r.Context())
		if getErr != nil {
			http.Error(w, getErr.Error(), http.StatusInternalServerError)
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
		_, purgeErr := cfService.PurgeCache(r.Context(), zoneID)
		if purgeErr != nil {
			http.Error(w, purgeErr.Error(), http.StatusInternalServerError)
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

		_, upsertErr := cfService.UpsertDNSRecord(
			r.Context(),
			zoneID,
			strings.TrimSpace(r.Form.Get("name")),
			strings.TrimSpace(r.Form.Get("type")),
			strings.TrimSpace(r.Form.Get("content")),
			proxied,
			ttl,
			strings.TrimSpace(r.Form.Get("cname_content")),
		)
		if upsertErr != nil {
			http.Error(w, upsertErr.Error(), http.StatusBadRequest)
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
			_, updateErr := cfService.UpdateDNSRecord(
				r.Context(),
				zoneID,
				recordID,
				strings.TrimSpace(r.Form.Get("type")),
				strings.TrimSpace(r.Form.Get("name")),
				strings.TrimSpace(r.Form.Get("content")),
				proxied,
				ttl,
			)
			if updateErr != nil {
				http.Error(w, updateErr.Error(), http.StatusBadRequest)
				return
			}
		case "DELETE":
			_, deleteErr := cfService.DeleteDNSRecord(r.Context(), zoneID, recordID)
			if deleteErr != nil {
				http.Error(w, deleteErr.Error(), http.StatusBadRequest)
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
