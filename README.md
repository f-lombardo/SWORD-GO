# SWORD-GO

This project starts as a porting of the [SOWRD](https://github.com/SynioBE/SWORD) project.
It's still work in progress.

The project includes:

- command-line provisioning parity for DigitalOcean and Hetzner
- first web migration slice for **Servers** (server-rendered with **HTMX**, no Node.js runtime)

## Build

```bash
go build ./...
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
- `SWORD_GO_ADMIN_PASSWORD` (default `password`)
- `SWORD_GO_SESSION_SECRET` (default `change-me-in-env`)
