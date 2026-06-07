<p align="center">
  <img src="logo.svg" alt="DoTheSplit logo" width="180" />
</p>

<h1 align="center">DoTheSplit</h1>

<p align="center">
  Open-source expense-sharing app. See <a href="BLUEPRINT.md">BLUEPRINT.md</a> for the product definition.
</p>

---

## Quick start

```bash
cp .env.example .env

# Generate the three encryption keys + a Postgres password and append to .env.
# These end up in the database protecting every email and password - generate
# them ONCE and keep a copy somewhere safe. See "Secrets you must back up"
# below.
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)"
} >> .env

# Update DATABASE_URL in .env so the password matches POSTGRES_PASSWORD.

docker compose up -d
```

Open http://localhost:3000.

## Layout

- `/api`: Go 1.25 backend (Gin, pgx/v5, oapi-codegen) plus a separate `worker` binary for recurring expenses
- `/web`: Astro 6 + Tailwind v4 frontend, server-rendered via `@astrojs/node`
- `/docs/openapi.yaml`: API contract (source of truth, drives Go + TypeScript codegen)
- `/docs/DEVELOPMENT.md`, `/docs/FEATURES.md`: developer guide and feature catalogue
- `/docs/IMPORT.md`: importing a group (Splitwise or DoTheSplit CSV) and exporting one
- `/api/migrations`: append-only PostgreSQL 18 migrations (`golang-migrate`, paired `.up.sql` / `.down.sql`)
- `/docker-compose.yml`: local + LAN deployment stack
- `/scripts`: SBOM and third-party-license generators

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full build / test / deploy
guide and [INSTALL.md](INSTALL.md) for production install paths.

## Install on TrueNAS

Running on TrueNAS SCALE? See [INSTALL.md](INSTALL.md) for the Custom App
walkthrough: dataset layout, the host-path Postgres mount, and consuming the
first-run setup token from the API logs.

## Container images

Tagged releases publish multi-arch (`linux/amd64`, `linux/arm64`) OCI images to
the GitHub Container Registry:

| Image                                   | Tags                                    |
| --------------------------------------- | --------------------------------------- |
| `ghcr.io/julian-alarcon/dothesplit-api` | `vX.Y.Z`, `vX.Y`, `vX`, `latest`, `dev` |
| `ghcr.io/julian-alarcon/dothesplit-web` | `vX.Y.Z`, `vX.Y`, `vX`, `latest`, `dev` |

`:dev` always points at the latest commit on `main`. The api image hosts both
the `/api` and `/worker` entrypoints; in compose, override `entrypoint:
["/worker"]` to run the worker. Pull a pinned release for production:

```bash
docker pull ghcr.io/julian-alarcon/dothesplit-api:v0.3.0
docker pull ghcr.io/julian-alarcon/dothesplit-web:v0.3.0
```

The running version is reported by `GET /healthz` (api) and the page footer
(web), so you can confirm what's deployed at a glance.

## Secrets you must back up

Three values in `.env` are **the** load-bearing secrets for this app:

| Variable          | What it does                                  | If you lose it                                  | If it leaks                                        |
| ----------------- | --------------------------------------------- | ----------------------------------------------- | -------------------------------------------------- |
| `EMAIL_ENC_KEY`   | AES-GCM key that encrypts every email at rest | Existing emails are unrecoverable               | Attacker can decrypt every email                   |
| `EMAIL_HMAC_KEY`  | HMAC key for email lookup hashes              | Login by email stops working for existing users | Attacker can enumerate which emails are registered |
| `PASSWORD_PEPPER` | Server-side pepper added before Argon2id      | Existing passwords are unrecoverable            | Attacker can crack stolen password hashes offline  |

`POSTGRES_PASSWORD` is also sensitive but resettable later as long as you can reach the database.

**What this means for you:**

- Generate these once on first install. Don't regenerate on a rebuild - the database won't decrypt anymore.
- Store a copy in your password manager or secrets vault. Treat them like the master password to a vault: this app is the vault.
- When you back up the Postgres data volume (`dts_pg_data`), back up the `.env` alongside it. A backup without the keys is useless.
- Never commit `.env`. It's gitignored for a reason.

## Development

```bash
make gen            # regenerate Go + TS API bindings from openapi.yaml
make migrate-up     # apply DB migrations
make dev            # run api + web against a local postgres
make test           # unit + integration tests
```

## Features

See [docs/FEATURES.md](docs/FEATURES.md) for the long-form description. In short:

- **Accounts**: register / login, display name + password change, personal timezone, 8×8 pixel avatars (reducing privacy concerns on GDPR), soft-delete with stable tombstones.
- **First-run setup**: boot-time token gate so the first user is provably the operator.
- **Admin**: `/admin` area for users, groups, SMTP and audit, with step-up password prompts for destructive actions.
- **Groups**: create / rename / delete, **single currency per group** (multi-currency groups are intentionally unsupported, see [Roadmap](#roadmap) for the FX deferral), invites, leave, transfer ownership, default percent split for 2-member groups.
- **Expenses**: equal / exact / percent splits, ten categories, custom date, optional free-text notes, full edit history with per-member split diffs.
- **Balances & settle-up**: net balances, simplified "X owes Y" view, settlements in a paginated activity feed with detail pages. Pick who is paying when settling up; any member can later edit from / to / amount / note / date.
- **Recurring expenses**: daily / weekly / biweekly / monthly / yearly templates materialized by a background worker (UI shipped).
- **Search**: cross-group substring search over expense descriptions / notes and settlement notes, with collapsible Group and Category filters. The category picker only lists categories present in the current result set.
- **Import & export**: CSV in / out via `/import` (Splitwise or DoTheSplit) and group settings → Export. The DoTheSplit format keeps the Splitwise prefix and adds `Time`, `Payer`, `Notes`, `Created`, `CreatedBy`, so a round-trip preserves second-precision timestamps, explicit payers, and per-expense notes.
- **Security**: Argon2id, AES-GCM email at rest, rate-limited auth + setup, strict JSON bodies, hashed-inline CSP, password confirmation for self-delete.
- **API**: OpenAPI 3.0.3 contract at [docs/openapi.yaml](docs/openapi.yaml); every business endpoint is under `/v1/...`.

## Roadmap

Reasonable next steps, roughly prioritized. Contributions welcome: open an issue first so we can scope.

### Near term

- Extend search filters with date range and member.
- Add **Filter** to expenses activity list by category, member, date range.
- **Native mobile** via the PWA path (the Astro side is already SSR-first and mobile-first styled).

### Medium term

- **Backup**
- **i18n** (app is English-only today; amount and date formatting already respect the browser locale).
- **Optimistic UI + refresh-on-focus** via `@tanstack/react-query` (the perf budget is ≤100ms perceived: we're close on SSR but mutations still block).
- **Import** from Tricount

### Longer term / ideas

- **OAuth / passkeys** alongside passwords.
- **Real-time sync** (push updates via SSE or WebSockets instead of the current polling / refresh-on-focus model).
- **TLS terminated by Caddy in-compose** as a first-class option, replacing the current "terminate outside the stack" note below.
- **Multi-currency FX**: today each group picks one default currency; cross-currency groups would need conversion rates and a locked-at-time-of-entry policy.
- **Expense attachments / receipts** (photo or PDF).

Explicitly not planned: file hosting of full-resolution avatars (the 8×8 format is a deliberate GDPR-minimizing choice), account hard-delete (soft delete preserves other members' ledgers).

## Deployment note: HTTPS deviation

[BLUEPRINT.md](BLUEPRINT.md) states **"HTTPS only"**. The v1 LAN profile ships
**HTTP-only** for TrueNAS LAN use: session cookies use `Secure=false`. For
internet-exposed deployments, terminate TLS at an upstream reverse proxy (Caddy,
Traefik, Cloudflare Tunnel) and flip `COOKIE_SECURE=true`.

## License & compliance

DoTheSplit is released under the [MIT License](LICENSE).

Third-party attribution lives in two places:

- [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md): generated list of every direct and transitive Go module and npm package with SPDX license + source link. Includes the Font Awesome CC BY 4.0 attribution.
- `/about` route in the running app: human-readable summary linked from the user menu in the header.

CycloneDX SBOMs (`sbom/api.cdx.json`, `sbom/worker.cdx.json`, `sbom/web.cdx.json`) are attached as artifacts to every tagged GitHub Release, so auditors can ingest them into Dependency-Track, Trivy, OSV-Scanner, Grype, or any CycloneDX 1.5+ consumer.

Regenerate locally:

```bash
make compliance   # licenses + SBOMs
```
