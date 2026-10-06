# SWORD-GO

This project starts as a porting of the [SOWRD](https://github.com/SynioBE/SWORD) project.
It's still work in progress.

## Build

```bash
go build ./...
```

## Usage

```bash
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

