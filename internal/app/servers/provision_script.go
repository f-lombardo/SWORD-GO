package servers

import (
	"bytes"
	"errors"
	"net/url"
	"regexp"
	"strings"
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

set -Eeuo pipefail
IFS=$'\n\t'

CALLBACK_URL={{ .CallbackURLQ }}
HOSTNAME={{ .HostnameQ }}
TIMEZONE={{ .TimezoneQ }}
SWORD_PUBKEY={{ .SSHPublicKeyQ }}
SUDO_PASSWORD={{ .SudoPasswordQ }}
MYSQL_ROOT_PASSWORD={{ .MySQLRootPasswordQ }}

updateProgress() {
    local step="$1"
    local status="${2:-provisioning}"
    local ts
    local nonce
    ts="$(date +%s)"
    nonce="$(date +%s%N)-$RANDOM"
    echo "[$(date '+%H:%M:%S')] step: ${step} -> status: ${status}"
    curl -fsS --retry 3 --retry-delay 2 --connect-timeout 10 \
        --insecure \
        -d "status=${status}&step=${step}&ts=${ts}&nonce=${nonce}" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

notifyFailure() {
    local step="$1"
    local ts
    local nonce
    ts="$(date +%s)"
    nonce="$(date +%s%N)-$RANDOM"
    echo "[$(date '+%H:%M:%S')] step: ${step} -> status: failed"
    curl -fsS --retry 3 --retry-delay 2 --connect-timeout 10 \
        --insecure \
        -d "status=failed&step=${step}&ts=${ts}&nonce=${nonce}" \
        -X POST "${CALLBACK_URL}" > /dev/null || true
}

ensureConfigLine() {
    local file="$1"
    local match_regex="$2"
    local new_line="$3"

    touch "$file"
    if grep -Eq "$match_regex" "$file"; then
        sed -i -E "s|$match_regex.*|$new_line|" "$file"
    else
        echo "$new_line" >> "$file"
    fi
}

waitForApt() {
    while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do sleep 2; done
    while fuser /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 2; done
    while fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do sleep 2; done
}

requireCommand() {
    local cmd="$1"
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        echo "ERROR: required command '${cmd}' is missing." >&2
        exit 1
    fi
}

trap 'notifyFailure "Unexpected error on line $LINENO"' ERR

updateProgress "started"

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root." >&2
    exit 1
fi

requireCommand curl
requireCommand apt-get

export DEBIAN_FRONTEND=noninteractive
waitForApt
apt-get update -qq
waitForApt
apt-get upgrade -y -qq
updateProgress "os_upgrade"

waitForApt
apt-get install -y -qq \
  curl wget git zip unzip rsync jq gnupg \
  software-properties-common apt-transport-https ca-certificates \
  lsb-release ufw borgbackup cron
updateProgress "install_packages"

hostnamectl set-hostname "${HOSTNAME}"
timedatectl set-timezone "${TIMEZONE}"
updateProgress "timezone_hostname"

# Docker repository + installation
install -m 0755 -d /etc/apt/keyrings
if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
fi

ensureConfigLine \
  /etc/apt/sources.list.d/docker.list \
  '^deb .+download\.docker\.com/linux/ubuntu .+ stable$' \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable"

if ! command -v docker >/dev/null 2>&1; then
    waitForApt
    apt-get update -qq
    waitForApt
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
if ! docker info >/dev/null 2>&1; then
    echo "ERROR: docker daemon is not reachable." >&2
    exit 1
fi
systemctl enable --now docker
updateProgress "docker_setup"

mkdir -p /root/.ssh
chmod 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

if ! grep -qF "${SWORD_PUBKEY}" /root/.ssh/authorized_keys; then
    echo "${SWORD_PUBKEY}" >> /root/.ssh/authorized_keys
fi

updateProgress "ssh_setup"

if ! id sword >/dev/null 2>&1; then
    useradd -m -s /bin/bash sword
fi
groupadd -f docker
usermod -aG docker,sudo sword

PASSWORD_HASH=$(openssl passwd -6 "${SUDO_PASSWORD}")
usermod --password "${PASSWORD_HASH}" sword

mkdir -p /home/sword/.ssh
chmod 700 /home/sword/.ssh
cp /root/.ssh/authorized_keys /home/sword/.ssh/authorized_keys
chown -R sword:sword /home/sword/.ssh
chmod 600 /home/sword/.ssh/authorized_keys
updateProgress "user_setup"

mkdir -p /srv/sword/sites /srv/sword/stacks /srv/sword/shared/mysql/data /srv/sword/letsencrypt
chown -R sword:sword /srv/sword

if ! docker network inspect sword_network >/dev/null 2>&1; then
    docker network create sword_network
fi

cat > /srv/sword/shared/.env <<'ENVEOF'
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
ENVEOF

cat > /srv/sword/shared/docker-compose.yml <<'COMPOSEEOF'
services:
  traefik:
    image: traefik:v3
    restart: unless-stopped
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge=true"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /srv/sword/letsencrypt:/letsencrypt
    networks:
      - sword_network

  mysql:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
    volumes:
      - /srv/sword/shared/mysql/data:/var/lib/mysql
    networks:
      - sword_network

  ofelia:
    image: mcuadros/ofelia:latest
    restart: unless-stopped
    command: "daemon --docker"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - sword_network

networks:
  sword_network:
    name: sword_network
    external: true
COMPOSEEOF

docker compose -f /srv/sword/shared/docker-compose.yml up -d
updateProgress "shared_services"
updateProgress "provisioned" "provisioned"

echo "Provisioning complete for server {{ .Server.Name }}"
`))

func RenderProvisionScript(input ProvisionScriptInput) (string, error) {
	if err := validateProvisionScriptInput(input); err != nil {
		return "", err
	}

	var buffer bytes.Buffer

	err := provisionScriptTemplate.Execute(&buffer, map[string]any{
		"Server":             input.Server,
		"GeneratedAt":        time.Now().UTC().Format(time.RFC3339),
		"CallbackURLQ":       shellQuote(input.CallbackURL),
		"HostnameQ":          shellQuote(input.Server.Hostname),
		"TimezoneQ":          shellQuote(input.Server.Timezone),
		"SSHPublicKeyQ":      shellQuote(input.Server.SSHPublicKey),
		"SudoPasswordQ":      shellQuote(input.Server.SudoPassword),
		"MySQLRootPasswordQ": shellQuote(input.Server.MySQLRootPassword),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func validateProvisionScriptInput(input ProvisionScriptInput) error {
	if strings.TrimSpace(input.Server.Hostname) == "" {
		return errors.New("hostname is required")
	}
	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`).MatchString(strings.TrimSpace(input.Server.Hostname)) {
		return errors.New("hostname contains unsafe characters")
	}
	if _, err := time.LoadLocation(strings.TrimSpace(input.Server.Timezone)); err != nil {
		return errors.New("invalid timezone")
	}
	if strings.TrimSpace(input.Server.SSHPublicKey) == "" || strings.ContainsAny(input.Server.SSHPublicKey, "\r\n") {
		return errors.New("invalid ssh public key")
	}
	if strings.TrimSpace(input.Server.SudoPassword) == "" {
		return errors.New("sudo password is required")
	}
	if strings.TrimSpace(input.Server.MySQLRootPassword) == "" {
		return errors.New("mysql root password is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(input.CallbackURL))
	if err != nil {
		return errors.New("invalid callback URL")
	}
	if !parsedURL.IsAbs() || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("callback URL must be absolute")
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
