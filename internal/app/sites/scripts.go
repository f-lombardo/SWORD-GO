package sites

import (
	"bytes"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
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

set -Eeuo pipefail
IFS=$'\n\t'

CALLBACK_URL={{ .CallbackURLQ }}
DOMAIN={{ .DomainQ }}
PHP_VERSION={{ .PHPVersionQ }}
DB_NAME={{ .DBNameQ }}
DB_USER={{ .DBUserQ }}
DB_PASS={{ .DBPassQ }}
MYSQL_ROOT_PASSWORD={{ .MySQLRootPasswordQ }}
SITE_DIR="/srv/sword/sites/${DOMAIN}"
STACK_DIR="/srv/sword/stacks/${DOMAIN}"
WP_DIR="${SITE_DIR}/wordpress"
CACHE_DIR="${STACK_DIR}/nginx-cache"

updateProgress() {
    local step="$1"
    local status="${2:-installing}"
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
requireCommand docker

if ! docker info >/dev/null 2>&1; then
    echo "ERROR: docker daemon is not reachable." >&2
    exit 1
fi

mkdir -p "${SITE_DIR}" "${STACK_DIR}" "${WP_DIR}" "${CACHE_DIR}"
chown -R sword:sword "${SITE_DIR}" "${STACK_DIR}"

if ! docker ps --format '{{ "{{" }} .Names {{ "}}" }}' | grep -q '^sword_mysql$'; then
    echo "ERROR: sword_mysql container not found. Provision shared services first." >&2
    exit 1
fi

docker exec sword_mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" \
    -e "CREATE DATABASE IF NOT EXISTS ${DB_NAME} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
docker exec sword_mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" \
    -e "CREATE USER IF NOT EXISTS '${DB_USER}'@'%' IDENTIFIED BY '${DB_PASS}';"
docker exec sword_mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" \
    -e "GRANT ALL PRIVILEGES ON ${DB_NAME}.* TO '${DB_USER}'@'%'; FLUSH PRIVILEGES;"
updateProgress "create_database"

cat > "${STACK_DIR}/Dockerfile" <<DOCKERFILEEOF
FROM php:${PHP_VERSION}-fpm-alpine

RUN apk add --no-cache \
    curl git unzip libzip-dev oniguruma-dev icu-dev \
    && docker-php-ext-install -j\$(nproc) mysqli pdo pdo_mysql zip intl mbstring opcache

RUN curl -sO https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar \
    && chmod +x wp-cli.phar \
    && mv wp-cli.phar /usr/local/bin/wp
DOCKERFILEEOF

cat > "${STACK_DIR}/docker-compose.yml" <<COMPOSEEOF
services:
  php:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: sword_{{ .Site.ID }}_php
    restart: unless-stopped
    volumes:
      - ${WP_DIR}:/var/www/html
    networks:
      - sword_network

  nginx:
    image: nginx:alpine
    container_name: sword_{{ .Site.ID }}_nginx
    restart: unless-stopped
    volumes:
      - ${WP_DIR}:/var/www/html:ro
      - ${STACK_DIR}/nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - ${CACHE_DIR}:/var/cache/nginx/fastcgi:rw
    labels:
      - traefik.enable=true
      - traefik.http.routers.{{ .Site.ID }}.rule=Host(\"{{ .Site.Domain }}\")
      - traefik.http.routers.{{ .Site.ID }}.entrypoints=websecure
      - traefik.http.routers.{{ .Site.ID }}.tls.certresolver=letsencrypt
    networks:
      - sword_network

  redis:
    image: redis:7-alpine
    container_name: sword_{{ .Site.ID }}_redis
    restart: unless-stopped
    networks:
      - sword_network

networks:
  sword_network:
    name: sword_network
    external: true
COMPOSEEOF

cat > "${STACK_DIR}/nginx.conf" <<NGINXEOF
server {
    listen 80;
    server_name ${DOMAIN};
    root /var/www/html;
    index index.php;

    location / {
        try_files \$uri \$uri/ /index.php?\$args;
    }

    location ~ \.php\$ {
        fastcgi_pass php:9000;
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME \$document_root\$fastcgi_script_name;
    }
}
NGINXEOF

docker compose -f "${STACK_DIR}/docker-compose.yml" up -d --build
updateProgress "docker_setup"

sleep 3

docker exec "sword_{{ .Site.ID }}_php" wp core download --path=/var/www/html --allow-root --force

docker exec -i "sword_{{ .Site.ID }}_php" wp config create \
    --path=/var/www/html \
    --dbname="${DB_NAME}" \
    --dbuser="${DB_USER}" \
    --dbpass="${DB_PASS}" \
    --dbhost="sword_mysql" \
    --allow-root \
    --force

docker exec "sword_{{ .Site.ID }}_php" wp core install \
    --path=/var/www/html \
    --url="https://${DOMAIN}" \
    --title="${DOMAIN}" \
    --admin_user={{ .AdminUserQ }} \
    --admin_password={{ .AdminPasswordQ }} \
    --admin_email={{ .AdminEmailQ }} \
    --skip-email \
    --allow-root

docker exec "sword_{{ .Site.ID }}_php" wp user update {{ .AdminUserQ }} \
    --path=/var/www/html \
    --display_name={{ .AdminDisplayNameQ }} \
    --allow-root

chown -R sword:sword "${SITE_DIR}"
docker exec "sword_{{ .Site.ID }}_php" chown -R www-data:www-data /var/www/html
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

set -Eeuo pipefail
IFS=$'\n\t'

DOMAIN={{ .DomainQ }}
DB_NAME={{ .DBNameQ }}
DB_USER={{ .DBUserQ }}
MYSQL_ROOT_PASSWORD={{ .MySQLRootPasswordQ }}
SITE_DIR="/srv/sword/sites/${DOMAIN}"
STACK_DIR="/srv/sword/stacks/${DOMAIN}"

echo "Stopping and removing containers..."
docker compose -f "${STACK_DIR}/docker-compose.yml" down --remove-orphans 2>/dev/null || true
docker rm -f "sword_{{ .Site.ID }}_php" 2>/dev/null || true
docker rm -f "sword_{{ .Site.ID }}_nginx" 2>/dev/null || true
docker rm -f "sword_{{ .Site.ID }}_redis" 2>/dev/null || true

echo "Dropping database and user..."
docker exec sword_mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" \
    -e "DROP DATABASE IF EXISTS ${DB_NAME};" 2>/dev/null || true
docker exec sword_mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" \
    -e "DROP USER IF EXISTS '${DB_USER}'@'%'; FLUSH PRIVILEGES;" 2>/dev/null || true

echo "Removing site files..."
rm -rf "${SITE_DIR}" "${STACK_DIR}"

docker restart sword_ofelia 2>/dev/null || true
echo "Site {{ .Site.Domain }} deleted."
`))

func RenderInstallScript(input InstallScriptInput) (string, error) {
	if err := validateInstallScriptInput(input); err != nil {
		return "", err
	}

	var buffer bytes.Buffer

	err := installScriptTemplate.Execute(&buffer, map[string]any{
		"Site":               input.Site,
		"Server":             input.Server,
		"GeneratedAt":        time.Now().UTC().Format(time.RFC3339),
		"CallbackURLQ":       shellQuote(input.CallbackURL),
		"DomainQ":            shellQuote(input.Site.Domain),
		"PHPVersionQ":        shellQuote(input.Site.PHPVersion),
		"DBNameQ":            shellQuote(input.Site.DBName),
		"DBUserQ":            shellQuote(input.Site.DBUser),
		"DBPassQ":            shellQuote(input.Site.DBPassword),
		"MySQLRootPasswordQ": shellQuote(input.Server.MySQLRootPassword),
		"AdminUserQ":         shellQuote(input.AdminUser),
		"AdminPasswordQ":     shellQuote(input.AdminPassword),
		"AdminEmailQ":        shellQuote(input.AdminEmail),
		"AdminDisplayNameQ":  shellQuote(input.AdminDisplayName),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func RenderDeleteScript(input DeleteScriptInput) (string, error) {
	if err := validateDeleteScriptInput(input); err != nil {
		return "", err
	}

	var buffer bytes.Buffer

	err := deleteScriptTemplate.Execute(&buffer, map[string]any{
		"Site":               input.Site,
		"Server":             input.Server,
		"GeneratedAt":        time.Now().UTC().Format(time.RFC3339),
		"DomainQ":            shellQuote(input.Site.Domain),
		"DBNameQ":            shellQuote(input.Site.DBName),
		"DBUserQ":            shellQuote(input.Site.DBUser),
		"MySQLRootPasswordQ": shellQuote(input.Server.MySQLRootPassword),
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

var (
	domainPattern     = regexp.MustCompile(`^[a-zA-Z0-9.-]{1,255}$`)
	dbIdentPattern    = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
	phpVersionPattern = regexp.MustCompile(`^8\.[1-4]$`)
)

func validateInstallScriptInput(input InstallScriptInput) error {
	parsedCallbackURL, err := url.Parse(strings.TrimSpace(input.CallbackURL))
	if err != nil || !parsedCallbackURL.IsAbs() || parsedCallbackURL.Host == "" {
		return errors.New("invalid callback URL")
	}
	if !domainPattern.MatchString(strings.TrimSpace(input.Site.Domain)) {
		return errors.New("invalid domain")
	}
	if !dbIdentPattern.MatchString(strings.TrimSpace(input.Site.DBName)) || !dbIdentPattern.MatchString(strings.TrimSpace(input.Site.DBUser)) {
		return errors.New("invalid database identifier")
	}
	if strings.TrimSpace(input.Site.DBPassword) == "" || strings.ContainsAny(input.Site.DBPassword, "\r\n") {
		return errors.New("invalid database password")
	}
	if !phpVersionPattern.MatchString(strings.TrimSpace(input.Site.PHPVersion)) {
		return errors.New("invalid PHP version")
	}
	if strings.TrimSpace(input.Server.MySQLRootPassword) == "" || strings.ContainsAny(input.Server.MySQLRootPassword, "\r\n") {
		return errors.New("invalid mysql root password")
	}
	if strings.TrimSpace(input.AdminUser) == "" || strings.ContainsAny(input.AdminUser, "\r\n") {
		return errors.New("invalid admin user")
	}
	if strings.TrimSpace(input.AdminPassword) == "" || strings.ContainsAny(input.AdminPassword, "\r\n") {
		return errors.New("invalid admin password")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(input.AdminEmail)); err != nil {
		return errors.New("invalid admin email")
	}
	if strings.TrimSpace(input.AdminDisplayName) == "" || strings.ContainsAny(input.AdminDisplayName, "\r\n") {
		return errors.New("invalid admin display name")
	}
	return nil
}

func validateDeleteScriptInput(input DeleteScriptInput) error {
	if !domainPattern.MatchString(strings.TrimSpace(input.Site.Domain)) {
		return errors.New("invalid domain")
	}
	if !dbIdentPattern.MatchString(strings.TrimSpace(input.Site.DBName)) || !dbIdentPattern.MatchString(strings.TrimSpace(input.Site.DBUser)) {
		return errors.New("invalid database identifier")
	}
	if strings.TrimSpace(input.Server.MySQLRootPassword) == "" || strings.ContainsAny(input.Server.MySQLRootPassword, "\r\n") {
		return errors.New("invalid mysql root password")
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
