# SWORD-GO Operations Runbook

This document describes how to deploy, upgrade, and recover a production SWORD-GO instance.

## Goal of this packaging

The operational packaging provides:

- a reproducible container image (`Dockerfile`)
- a production compose stack (`docker-compose.prod.yml`)
- persistent SQLite storage (`sword_go_data` volume)
- health monitoring endpoint (`GET /healthz`)
- explicit environment-based configuration (`.env.production`)

## 1) First-time setup

1. Copy the environment template:

```bash
cp .env.production.example .env.production
```

2. Edit `.env.production` and set at least:

- `SWORD_GO_BASE_URL` (public URL)
- `SWORD_GO_ADMIN_PASSWORD_HASH` (bcrypt hash)
- `SWORD_GO_SESSION_SECRET` (long random secret)
- `SWORD_GO_AUTH_STRICT=true`

To generate a bcrypt hash for `SWORD_GO_ADMIN_PASSWORD_HASH`:

```bash
docker run --rm httpd:2.4-alpine \
  htpasswd -bnBC 12 "" "replace-with-password" | tr -d ':\n'
```

3. Start the stack:

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

4. Verify health:

```bash
curl -fsS http://127.0.0.1:${SWORD_GO_HTTP_PORT:-8088}/healthz
```

Expected response:

```json
{"status":"ok","time":"..."}
```

## 2) Day-2 operations

### Check service/container state

```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs --tail=200 sword-go
```

### Restart service

```bash
docker compose -f docker-compose.prod.yml restart sword-go
```

### Validate app responsiveness

```bash
curl -i http://127.0.0.1:${SWORD_GO_HTTP_PORT:-8088}/healthz
```

## 3) Upgrade procedure

SWORD-GO currently runs automatic schema initialization at startup, so no manual DB migration command is required.

1. Pull latest code (or update your deployment artifact).
2. Rebuild and recreate container:

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

3. Confirm:

- container healthy (`docker compose ... ps`)
- `/healthz` returns 200
- web login works

## 4) Rollback procedure

If a new build fails:

1. Checkout the previous known-good commit/tag.
2. Rebuild and redeploy:

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

3. Re-check `/healthz` and UI login.

Because the SQLite database is in a named volume, data is preserved across container recreations.

## 5) Backup and restore (SQLite)

### Backup DB file

```bash
docker run --rm \
  -v sword-go_sword_go_data:/data \
  -v "$(pwd)/backups:/backups" \
  alpine:3.22 \
  sh -c 'cp /data/sword-go.db /backups/sword-go-$(date +%Y%m%d-%H%M%S).db'
```

### Restore DB file

Stop SWORD-GO before restore:

```bash
docker compose -f docker-compose.prod.yml stop sword-go
```

Restore:

```bash
docker run --rm \
  -v sword-go_sword_go_data:/data \
  -v "$(pwd)/backups:/backups" \
  alpine:3.22 \
  sh -c 'cp /backups/<backup-file>.db /data/sword-go.db'
```

Start again:

```bash
docker compose -f docker-compose.prod.yml start sword-go
```

## 6) Security and reverse proxy notes

- Use HTTPS in front of SWORD-GO in production.
- Keep `SWORD_GO_SECURE_COOKIES=true` when HTTPS is enabled.
- Keep `SWORD_GO_AUTH_STRICT=true` in production.
- Use `SWORD_GO_ADMIN_PASSWORD_HASH` instead of plaintext password env values.
- Restrict access to the web UI (VPN, IP allow-list, or SSO proxy) when possible.
- Rotate `SWORD_GO_SESSION_SECRET` and admin credentials periodically.
- Login attempts are rate-limited (`SWORD_GO_LOGIN_MAX_ATTEMPTS` within `SWORD_GO_LOGIN_WINDOW_SECONDS`).
