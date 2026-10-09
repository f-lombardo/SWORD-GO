# Cloudflare Integration in SWORD-GO

## What Cloudflare is (in the context of SWORD-GO)

Cloudflare is an external edge platform that can act as:

- **authoritative DNS provider** (records like `A`, `CNAME`, etc.),
- **reverse proxy/CDN edge** in front of site origins,
- **cache layer** with purge controls,
- and a security/performance control point (TLS mode, bot/rate/security products outside this doc’s current scope).

For SWORD-GO, we currently use Cloudflare primarily as a **DNS + cache control API surface**.  
This means SWORD-GO can programmatically keep domain routing aligned with infrastructure changes (new server IPs, domain
onboarding, cache invalidation after key operations) without manual dashboard work.

---

## Goal of this integration

The Cloudflare integration is the control-plane bridge between SWORD-GO and public DNS/edge behavior.  
Its purpose is to let the platform:

1. **discover and manage DNS zones** owned by your Cloudflare account,
2. **create/update/delete DNS records** required for server/site provisioning flows,
3. **purge cache** when operational changes require immediate edge invalidation,
4. provide a **single operational UI** (inside SWORD-GO) for infrastructure + DNS tasks.

In practical terms, this removes manual DNS steps during server/site lifecycle management and keeps DNS actions
auditable from the same admin interface.

---

## Why this is important for this project

SWORD-GO provisions and operates WordPress runtimes. That only becomes useful to end users when DNS is correctly pointed
and consistently managed. This integration is important because it:

1. **Closes the provisioning loop**: infra can be created and DNS can be updated in the same control plane.
2. **Reduces human error**: no copy/paste of record data across systems.
3. **Improves recovery speed**: cache purge and record updates are available immediately from SWORD-GO workflows.
4. **Enables automation-first operations**: future onboarding flows can become one-click (server + site + DNS).
5. **Creates a clean abstraction seam**: SWORD-GO can later add other DNS providers behind the same domain-level intent.

---

## Supported authentication modes

SWORD-GO supports the same two credential styles used by the Laravel app:

- **API Token** (`type=api_token`) — recommended, scoped and safer.
- **Global API Key** (`type=global_key`) — legacy-style full-account access (use only when token scope cannot satisfy
  your needs).

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

## Can Cloudflare be substituted? (future directions)

Yes. Cloudflare is a strong default, but the architecture can evolve toward a **provider-agnostic DNS edge interface**.

### Candidate alternatives

| Option                               | What it replaces         | Pros                                                      | Trade-offs / migration impact                                                                 |
|--------------------------------------|--------------------------|-----------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| AWS Route53                          | DNS management           | Deep AWS integration, mature APIs, health-check ecosystem | Different auth model (IAM), different DNS semantics/features; cache purge must move elsewhere |
| Cloud DNS (GCP) / Azure DNS          | DNS management           | Good cloud-native fit if infra is on same cloud           | Similar to Route53 trade-offs; no Cloudflare edge cache controls                              |
| DNSMadeEasy, Namecheap API, etc.     | DNS management           | Lower cost / registrar coupling in some contexts          | API quality/features vary, higher adapter maintenance                                         |
| PowerDNS (self-hosted)               | DNS management           | Full control, no vendor lock-in                           | Operational burden, HA/security/on-call ownership                                             |
| Traefik + ACME + direct DNS only     | Partial edge replacement | Simpler stack for small setups                            | Loses Cloudflare CDN/proxy/cache capabilities                                                 |
| Multi-provider strategy (abstracted) | Vendor dependency        | Portability and resilience                                | More complexity in provider abstraction and test matrix                                       |

### Practical design recommendation for SWORD-GO

To keep future migration simple, structure Cloudflare as one implementation of a generic interface, for example:

- `DNSProvider` (`ListZones`, `UpsertRecord`, `DeleteRecord`, `FindZoneForDomain`)
- `CacheProvider` (`PurgeZone`, optionally tag/path purge later)

Then:

1. keep existing Cloudflare implementation as `cloudflare` adapter,
2. add provider-specific adapters (`route53`, `gcpdns`, etc.),
3. select provider per integration row (`provider`),
4. keep UI intent-based (record operations) rather than provider-feature-based by default.

This preserves compatibility while allowing gradual multi-provider support.

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

It keeps DNS automation colocated with the rest of SWORD-GO orchestration, minimizing context switches and reducing
manual infrastructure drift.
