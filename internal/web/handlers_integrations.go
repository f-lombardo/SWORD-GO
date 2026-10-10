package web

import (
	"net/http"
	"strings"

	"sword-go/internal/app/integrations"
)

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
