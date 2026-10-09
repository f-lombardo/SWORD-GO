package servers

import (
	"bytes"
	"text/template"
	"time"
)

type ProvisionScriptInput struct {
	Server      Server
	CallbackURL string
}

var provisionScriptTemplate = template.Must(template.New("provision-script").Parse(`#!/usr/bin/env bash
# ============================================================
# SWORD Server Provisioning Script
# Server: {{ .Server.Name }} (ID: {{ .Server.ID }})
# Generated: {{ .GeneratedAt }}
# ============================================================

CALLBACK_URL="{{ .CallbackURL }}"

updateProgress() {
    curl -s --insecure -d "status=${2:-provisioning}&step=$1" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

notifyFailure() {
    curl -s --insecure -d "status=failed&step=$1" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

trap 'notifyFailure "Unexpected error on line $LINENO"' ERR

updateProgress "started"

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root." >&2
    exit 1
fi

hostnamectl set-hostname "{{ .Server.Hostname }}"
timedatectl set-timezone "{{ .Server.Timezone }}"

mkdir -p /root/.ssh
chmod 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

SWORD_PUBKEY="{{ .Server.SSHPublicKey }}"
if ! grep -qF "${SWORD_PUBKEY}" /root/.ssh/authorized_keys; then
    echo "${SWORD_PUBKEY}" >> /root/.ssh/authorized_keys
fi

updateProgress "ssh_setup"
updateProgress "provisioned" "provisioned"

echo "Provisioning complete for server {{ .Server.Name }}"
`))

func RenderProvisionScript(input ProvisionScriptInput) (string, error) {
	var buffer bytes.Buffer

	err := provisionScriptTemplate.Execute(&buffer, map[string]any{
		"Server":      input.Server,
		"CallbackURL": input.CallbackURL,
		"GeneratedAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
