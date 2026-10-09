# Cloudflare Integration in SWORD-GO

## Goal of this integration

The Cloudflare integration is the control-plane bridge between SWORD-GO and public DNS/edge behavior.  
Its purpose is to let the platform:

1. **discover and manage DNS zones** owned by your Cloudflare account,
2. **create/update/delete DNS records** required for server/site provisioning flows,
3. **purge cache** when operational changes require immediate edge invalidation,
4. provide a **single operational UI** (inside SWORD-GO) for infrastructure + DNS tasks.

In practical terms, this removes manual DNS steps during server/site lifecycle management and keeps DNS actions auditable from the same admin interface.

---

## Supported authentication modes

SWORD-GO supports the same two credential styles used by the Laravel app:

- **API Token** (`type=api_token`) — recommended, scoped and safer.
- **Global API Key** (`type=global_key`) — legacy-style full-account access (use only when token scope cannot satisfy your needs).

Integrations are stored in the local app database (`integrations` table), with credentials serialized in JSON.

---

## What is implemented

### Integrations management

- create integration
- list integrations
- update integration (with “keep existing secrets if left blank” behavior)
- delete integration

Web routes:

- `GET /settings/integrations`
- `POST /settings/integrations`
- `POST /settings/integrations/{id}` with `_method=PATCH|DELETE`

### Cloudflare operations

- list zones for a Cloudflare integration
- show zone details page
- list DNS records
- upsert DNS record (`A`, `CNAME`, or `both`)
- update DNS record
- delete DNS record
- purge full zone cache
- resolve zone by FQDN internally (`find zone for domain`)

Web routes:

- `GET /cloudflare`
- `GET /cloudflare/{integrationId}`
- `GET /cloudflare/{integrationId}/{zoneId}`
- `POST /cloudflare/{integrationId}/{zoneId}/purge-cache`
- `POST /cloudflare/{integrationId}/{zoneId}/dns-records`
- `POST /cloudflare/{integrationId}/{zoneId}/dns-records/{recordId}` with `_method=PATCH|DELETE`

---

## Setup guide

## 1) Start SWORD-GO

From `SWORD-GO/`:

```bash
go run . web
```

By default it serves on `http://localhost:8088`.

Optional environment variables:

- `SWORD_GO_HTTP_PORT`
- `SWORD_GO_DB_DSN`
- `SWORD_GO_ADMIN_EMAIL`
- `SWORD_GO_ADMIN_PASSWORD`
- `SWORD_GO_SESSION_SECRET`
- `SWORD_GO_BASE_URL`

---

## 2) Login

Default credentials (if unchanged):

- email: `admin@example.com`
- password: `password`

You should override these in environment variables for any non-local usage.

---

## 3) Create a Cloudflare integration

Open:

- `http://localhost:8088/settings/integrations`

Create a new integration with:

- `provider=cloudflare`
- auth `type=api_token` **or** `type=global_key`

### Token mode fields

- `token` (Cloudflare API token)

### Global key mode fields

- `email`
- `key` (global API key)

---

## 4) Verify zone visibility

Open:

- `http://localhost:8088/cloudflare`

Select your integration and verify zones are listed.

If zones are empty or request fails, verify:

- token/key correctness,
- token scopes,
- account/zone access boundaries.

---

## 5) Manage DNS records

From a zone page:

- use **Upsert DNS** to create/update records by `(type,name)` match,
- use per-record actions to update/delete specific record IDs.

### Upsert behavior notes

- `type=A` or `type=CNAME`: upsert single record.
- `type=both`: upsert `A` from `content` and `CNAME` from `cname_content`.

---

## 6) Purge cache

Use the **Purge cache** action on a zone page to trigger Cloudflare “purge everything”.

This is useful after:

- DNS cutovers,
- origin changes,
- urgent content invalidation windows.

---

## Recommended Cloudflare token scopes

For least-privilege token auth, include:

- Zone:Read
- DNS:Read
- DNS:Edit
- Cache Purge:Edit

Scope to the specific zones SWORD-GO should manage.

---

## Troubleshooting

## Authentication errors

- Ensure `type` matches provided fields:
  - `api_token` => token required
  - `global_key` => email+key required
- Regenerate token/key if revoked or expired.

## Zone list works but DNS changes fail

- Token may have read-only DNS scope.
- Add `DNS:Edit`.

## Purge fails

- Add `Cache Purge:Edit`.
- Verify target zone ownership in the same account context.

## Unexpected DNS duplicates

- Upsert matches on exact `(type,name)` only.
- Existing records with different names (e.g. trailing dot variants) are treated as different records.

---

## Security considerations

- Use **API tokens** over global keys whenever possible.
- Keep `SWORD_GO_SESSION_SECRET` non-default.
- Avoid logging raw token/key values.
- Restrict app exposure and use HTTPS/reverse proxy in shared environments.
- Rotate credentials periodically and after team changes.

---

## Operational intent in SWORD architecture

This integration is designed to be the DNS control interface for:

- provisioning servers,
- assigning/updating domain records,
- post-deploy and incident cache operations.

It keeps DNS automation colocated with the rest of SWORD-GO orchestration, minimizing context switches and reducing manual infrastructure drift.

