package sites

import (
	"bytes"
	"text/template"
	"time"

	"sword-go/internal/app/servers"
)

type InstallScriptInput struct {
	Site             Site
	Server           servers.Server
	CallbackURL      string
	AdminUser        string
	AdminPassword    string
	AdminEmail       string
	AdminDisplayName string
}

type DeleteScriptInput struct {
	Site   Site
	Server servers.Server
}

var installScriptTemplate = template.Must(template.New("site-install-script").Parse(`#!/usr/bin/env bash
# ============================================================
# SWORD Site Installation Script
# Site:   {{ .Site.Domain }} (ID: {{ .Site.ID }})
# Server: {{ .Server.Name }} (ID: {{ .Server.ID }})
# Generated: {{ .GeneratedAt }}
# ============================================================

CALLBACK_URL="{{ .CallbackURL }}"
DOMAIN="{{ .Site.Domain }}"
DB_NAME="{{ .Site.DBName }}"
DB_USER="{{ .Site.DBUser }}"
DB_PASS="{{ .Site.DBPassword }}"

updateProgress() {
    curl -s --insecure -d "status=${2:-installing}&step=$1" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

notifyFailure() {
    curl -s --insecure -d "status=failed&step=$1" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

trap 'notifyFailure "Unexpected error on line $LINENO"' ERR

updateProgress "started"
updateProgress "install_wordpress"
updateProgress "installed" "installed"
`))

var deleteScriptTemplate = template.Must(template.New("site-delete-script").Parse(`#!/usr/bin/env bash
# ============================================================
# SWORD Site Deletion Script
# Site:   {{ .Site.Domain }} (ID: {{ .Site.ID }})
# Server: {{ .Server.Name }} (ID: {{ .Server.ID }})
# Generated: {{ .GeneratedAt }}
# ============================================================

echo "Deleting site {{ .Site.Domain }} ..."
`))

func RenderInstallScript(input InstallScriptInput) (string, error) {
	var buffer bytes.Buffer

	err := installScriptTemplate.Execute(&buffer, map[string]any{
		"Site":             input.Site,
		"Server":           input.Server,
		"CallbackURL":      input.CallbackURL,
		"AdminUser":        input.AdminUser,
		"AdminPassword":    input.AdminPassword,
		"AdminEmail":       input.AdminEmail,
		"AdminDisplayName": input.AdminDisplayName,
		"GeneratedAt":      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func RenderDeleteScript(input DeleteScriptInput) (string, error) {
	var buffer bytes.Buffer

	err := deleteScriptTemplate.Execute(&buffer, map[string]any{
		"Site":        input.Site,
		"Server":      input.Server,
		"GeneratedAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
