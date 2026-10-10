# SWORD-GO

This project starts as a porting of the [SOWRD](https://github.com/SynioBE/SWORD) project.
It's still work in progress.

The project includes:

- command-line provisioning parity for DigitalOcean and Hetzner
- first web migration slice for **Servers** (server-rendered with **HTMX**, no Node.js runtime)
- second web migration slice for **Sites** (create/show/delete + install/delete scripts + install callback)
- third web migration slice for **Backups** (destinations, schedules, due dispatch, run history, and real SSH/Borg execution pipeline)
- fourth web migration slice for **Integrations + Cloudflare** (integrations CRUD, zone browsing, DNS CRUD/upsert, cache purge)
- security hardening updates for web flows (CSRF protection on state-changing forms, session TTL validation, optional secure cookies, public token/signed callback endpoints for automation scripts)
- hardened server/site lifecycle scripts with idempotent provisioning/install/delete steps (Docker shared services, WordPress bootstrap, cleanup)
- callback replay protection for lifecycle callbacks (`ts` + `nonce` enforced with freshness/replay checks) and stricter shell input validation/quoting for generated scripts
- production auth/session hardening (bcrypt password hash support, strict auth mode, and login rate limiting)

## Cloudflare integration documentation

See: `docs/CLOUDFLARE-INTEGRATION.md`

## Operations runbook

See: `docs/OPERATIONS.md`

## Build

```bash
go build ./...
```

## Containerized production run

```bash
cp .env.production.example .env.production
# edit .env.production with secure values
docker compose -f docker-compose.prod.yml up -d --build
```

Health endpoint:

```bash
curl -fsS http://127.0.0.1:8088/healthz
```

## Usage

```bash
# Web UI (default admin: admin@example.com / password)
go run . web

# DigitalOcean
go run . digitalocean create \
  --key=<token> \
  --public-key="ssh-ed25519 AAAA..."

# Hetzner
go run . hetzner create \
  --key=<token> \
  --public-key="ssh-ed25519 AAAA..."
```

This is the API key resolution order::

1. `--key`
2. Secret file (`/run/secrets/do_api_key` or `/run/secrets/hetzner_api_key`)
3. Environment variable (`DIGITALOCEAN_TOKEN` or `HETZNER_TOKEN`)

The `--public-key` value accepts either:

- an existing key name
- a raw SSH public key string (the key is uploaded or re-used if already present)

## Web UI environment variables

- `SWORD_GO_HTTP_PORT` (default `8088`)
- `SWORD_GO_DB_DSN` (default `file:sword-go.db?cache=shared&mode=rwc`)
- `SWORD_GO_ADMIN_EMAIL` (default `admin@example.com`)
- `SWORD_GO_ADMIN_PASSWORD` (default `password`, development fallback)
- `SWORD_GO_ADMIN_PASSWORD_HASH` (bcrypt hash; required when strict mode is enabled)
- `SWORD_GO_SESSION_SECRET` (default `change-me-in-env`)
- `SWORD_GO_BASE_URL` (default inferred as `http://localhost:$SWORD_GO_HTTP_PORT`)
- `SWORD_GO_SECURE_COOKIES` (default `false`; set to `true` behind HTTPS so session/CSRF cookies are marked `Secure`)
- `SWORD_GO_AUTH_STRICT` (default follows `SWORD_GO_SECURE_COOKIES`; when `true`, requires strong session secret and bcrypt hash auth)
- `SWORD_GO_LOGIN_MAX_ATTEMPTS` (default `5`)
- `SWORD_GO_LOGIN_WINDOW_SECONDS` (default `600`)

## Automation endpoints used by generated scripts

The provision/install/delete scripts and callback updates are exposed under unauthenticated but signed/tokenized endpoints:

- `/public/servers/:id/scripts/provision?token=...`
- `/public/servers/:id/callbacks/provision?signature=...`
- `/public/sites/:id/scripts/install?token=...`
- `/public/sites/:id/scripts/delete?token=...`
- `/public/sites/:id/callbacks/install?signature=...`

These endpoints are intentionally separate from authenticated UI routes so non-browser processes can execute lifecycle scripts and report progress securely.
