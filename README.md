<p align="center">
  <img src="logo.svg" alt="DoTheSplit logo" width="180" />
</p>

<h1 align="center">DoTheSplit</h1>

<p align="center">
  Open-source expense-sharing app. See <a href="BLUEPRINT.md">BLUEPRINT.md</a> for the product definition.
</p>

---

DoTheSplit runs on **SQLite** (the default, zero-dependency single container) or
**PostgreSQL** (for multi-instance / scale-out). Each compose file sets
`DATABASE_DRIVER` for you (it is required and has no default, so an unset env
fails fast).

## Install

For production, run the pinned GHCR image (no build step). First generate the
four encryption keys into `.env` - they protect every email and password, so
generate them **once** and keep a copy somewhere safe (see [Backup](#backup)):

```bash
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
} >> .env
```

### SQLite (default)

One `dothesplit` container, one DB file on a volume. No Postgres, no migrate
step, no separate worker (all run in-process). Substitute your release tag for
`1.2.0`:

```yaml
# docker-compose.yml
services:
  dothesplit:
    image: ghcr.io/julian-alarcon/dothesplit:1.2.0
    environment:
      DATABASE_DRIVER: sqlite
      DATABASE_URL: file:/data/dts.db
      WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:8080}
      EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
      EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
      PASSWORD_PEPPER: ${PASSWORD_PEPPER}
      JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
    volumes:
      - dts_sqlite_data:/data
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/dothesplit", "--healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s

volumes:
  dts_sqlite_data:
```

```bash
docker compose up -d
```

### PostgreSQL (scale-out)

Add a Postgres password and point `DATABASE_URL` at the database. The Postgres
compose file (`docker-compose.postgres.yml`) sets `DATABASE_DRIVER=postgres` on
the `dothesplit` service and adds the `postgres` + `migrate` services:

```bash
echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)" >> .env
# Update DATABASE_URL in .env so the password matches POSTGRES_PASSWORD.

docker compose -f docker-compose.postgres.yml up -d
```

### First-run setup

Read the one-time setup token from the logs, open http://localhost:8080/setup,
and paste it:

```bash
docker compose logs dothesplit | grep -A2 'first-run setup'
```

See [INSTALL.md](INSTALL.md) for the full production guide: hardened compose,
HTTPS / reverse-proxy exposure, and updates.

### Install on TrueNAS

Running on TrueNAS SCALE? See [INSTALL.md](INSTALL.md#truenas-scale-custom-app)
for the Custom App walkthrough: dataset layout for either engine, host-path bind
mounts for snapshots, and consuming the first-run setup token from the logs.

## Container images

Tagged releases publish multi-arch (`linux/amd64`, `linux/arm64`) OCI images to
the GitHub Container Registry:

| Image                               | Tags                                 |
| ----------------------------------- | ------------------------------------ |
| `ghcr.io/julian-alarcon/dothesplit` | `X.Y.Z`, `X.Y`, `X`, `latest`, `dev` |

`:dev` always points at the latest commit on `main`. The image embeds the SPA
and runs a single `/dothesplit` binary that serves the API, the SPA, and the
in-process background worker. Pull a pinned release for production:

```bash
docker pull ghcr.io/julian-alarcon/dothesplit:1.2.0
```

The running version is reported by `GET /healthz` and the page footer,
so you can confirm what's deployed at a glance.

## Backup

A complete backup is **the data (DB volume) plus the `.env` keys**. Either half
alone is useless: the database is encrypted at the application layer, so a
data-only backup can't be decrypted without the keys, and the keys alone hold no
data.

### Secrets you must back up

Four values in `.env` are **the** load-bearing secrets for this app:

| Variable          | What it does                                  | If you lose it                                  | If it leaks                                        |
| ----------------- | --------------------------------------------- | ----------------------------------------------- | -------------------------------------------------- |
| `EMAIL_ENC_KEY`   | AES-GCM key that encrypts every email at rest | Existing emails are unrecoverable               | Attacker can decrypt every email                   |
| `EMAIL_HMAC_KEY`  | HMAC key for email lookup hashes              | Login by email stops working for existing users | Attacker can enumerate which emails are registered |
| `PASSWORD_PEPPER` | Server-side pepper added before Argon2id      | Existing passwords are unrecoverable            | Attacker can crack stolen password hashes offline  |
| `JWT_SIGNING_KEY` | HS256 key signing SPA / native access tokens  | All token clients are logged out (recoverable)  | Attacker can mint valid access tokens for any user |

`POSTGRES_PASSWORD` (Postgres deployments only) is also sensitive but resettable later as long as you can reach the database. `JWT_SIGNING_KEY` is the least catastrophic to lose: rotating it only forces every token client to log in again.

These keys are required and identical for both engines: the encryption is application-level (emails AES-GCM, passwords Argon2id + pepper), done above the storage layer, so switching between SQLite and Postgres does not change what's protected. Neither engine encrypts the whole database file/volume.

- Generate these once on first install. Don't regenerate on a rebuild - the database won't decrypt anymore.
- Store a copy in your password manager or secrets vault. Treat them like the master password to a vault: this app is the vault.
- Never commit `.env`. It's gitignored for a reason.

### SQLite

Stop the app first so the WAL is checkpointed into `dts.db`, then snapshot the
volume that holds `dts.db` (plus its `-wal` / `-shm` sidecars):

```bash
docker compose stop
docker run --rm -v dts_sqlite_data:/data -v "$PWD":/backup alpine \
  tar czf /backup/dts-sqlite-$(date +%F).tar.gz -C /data .
docker compose start
```

Restore by extracting the tarball back into a fresh `dts_sqlite_data` volume,
then bring the stack up.

### PostgreSQL

Dump the database with `pg_dump` (no downtime needed):

```bash
docker compose -f docker-compose.postgres.yml exec postgres \
  pg_dump -U dts dts | gzip > dts-pg-$(date +%F).sql.gz
```

Restore into a fresh database with `psql` (or `pg_restore` for a custom-format
dump). Alternatively, stop the stack and snapshot the `dts_pg_data` volume.

In both cases, back up `.env` alongside the data.

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full build / test / deploy
guide, the repository layout, and the contract-first workflow. The targets you'll
reach for most:

```bash
make gen            # regenerate Go + TS API bindings from openapi.yaml
make lint           # golangci-lint (Go) + eslint (SPA)
make test           # unit + integration tests
make test-go-both   # run the Go integration suite against both SQLite and Postgres
```

## Features

See [docs/FEATURES.md](docs/FEATURES.md) for the long-form description. In short:
Accounts, First-run setup, Admin, Groups, Expenses, Balances & settle-up,
Recurring expenses, Transaction feed, Real-time updates, Activity log, Email
notifications, Themes & PWA, Search, Import & export, Settings & about, dual
SQLite/PostgreSQL storage, Security, and a contract-first OpenAPI 3.0.3 API.

## Related / similar projects

Other open-source apps in this space, all worth a look:

- **[spliit-app/spliit](https://github.com/spliit-app/spliit)**: Next.js, Prisma, PostgreSQL. Splitwise alternative; receipt scanning and image storage lean on AWS S3 and OpenAI. Compared to DoTheSplit: no required third-party clouds (single self-contained Go binary), encryption at rest by default, and built-in Splitwise import. Status: active.
- **[oss-apps/split-pro](https://github.com/oss-apps/split-pro)**: Next.js, tRPC, Prisma, PostgreSQL. Polished, but auth is provider-only (NextAuth) with no built-in username/password login. Compared to DoTheSplit: built-in username/password auth (no external identity provider or SMTP to get started), and one container instead of a Node runtime plus `pg_cron` for recurring expenses. Status: actively maintained.
- **[spiral-project/ihatemoney](https://github.com/spiral-project/ihatemoney)**: Python, Flask, SQLite/PostgreSQL/MariaDB. Shared budget tool with one shared password per project, no per-person accounts. Compared to DoTheSplit: real per-user accounts and authentication, plus encryption at rest by default. Status: maintenance mode.
- **[eneiluj/moneybuster](https://gitlab.com/eneiluj/moneybuster)**: Android (Java/Kotlin). A client, not a server: needs a Nextcloud Cospend or IHateMoney backend. Compared to DoTheSplit: self-contained server and web UI, so there's no separate backend to stand up. Status: actively maintained.
- **[DennisBauer/RecurringExpenseTracker](https://github.com/DennisBauer/RecurringExpenseTracker)**: Kotlin Multiplatform (Android, iOS, desktop), Room. Single-user, local-only recurring-bill tracker; no sharing or settle-up. Compared to DoTheSplit: multi-member groups, shared ledgers, and settle-up (this overlaps only with DoTheSplit's recurring expenses). Status: actively maintained.
- **[fer0n/SplitBill](https://github.com/fer0n/SplitBill)**: native iOS (Swift). Photographs a receipt and allocates line items; local-only, no server or accounts. Compared to DoTheSplit: a shared multi-member ledger with accounts and sync, not a single-device companion. Status: actively maintained.
- **[lyskouski/app-finance](https://github.com/lyskouski/app-finance)** (Fingrom): Flutter/Dart (Android, iOS, desktop, web). Personal finance app, not a shared splitter; alpha-stage, Creative Commons license. Compared to DoTheSplit: a group/member model with settle-up between people, and an MIT license without commercial-use restrictions. Status: active.

**Where DoTheSplit is deliberately different:**

- **Self-contained, one container.** A single Go binary serves both the API and
  the embedded SPA - no Node runtime, no separate web service, no required
  third-party cloud (no S3, no OpenAI, no external auth provider). Runs on an
  embedded SQLite file out of the box - no external database to stand up - or
  point it at Postgres for scale-out.
- **Encryption at rest by default.** Emails are AES-GCM encrypted (HMAC for
  lookup), passwords are Argon2id with a server-side pepper, and the load-bearing
  keys live in `.env`, never the database. See [Secrets you must back up](#secrets-you-must-back-up).
- **Privacy-minimizing by construction.** Avatars are 8×8 pixel images (64 color
  samples) so they can't identify a person; accounts soft-delete to stable,
  non-identifying tombstones so co-members' ledgers survive.
- **Built-in username/password auth** with a provable first-run setup token - no
  external identity provider needed to get started.

## Roadmap

Reasonable next steps, roughly prioritized. Contributions welcome: open an issue first so we can scope.

### Near term

- Extend search filters with date range and member.
- Add **Filter** to expenses transaction list by category, member, date range.
- **Native mobile app-store builds**: the installable PWA (read-only offline) already ships, and the Vue SPA is client-rendered + mobile-first styled. The remaining step is wrapping the same bundle with Capacitor for the app stores. The service worker is served same-origin by the Go binary so it's covered by `script-src 'self'`; add `manifest-src`/`worker-src` to the CSP only if a `default-src` is ever introduced.

### Medium term

- **i18n** (app is English-only today; amount and date formatting already respect the browser locale).
- **Optimistic UI + refresh-on-focus** (the perf budget is ≤100ms perceived: reads are close but mutations still block). The hand-rolled composables would gain a caching/invalidation layer if this becomes painful.
- **Import** from Tricount

### Longer term / ideas

- **OAuth / passkeys** alongside passwords.
- **TLS terminated by Caddy in-compose** as a first-class option, replacing the current "terminate outside the stack" note below.
- **Multi-currency FX**: today each group picks one default currency; cross-currency groups would need conversion rates and a locked-at-time-of-entry policy.
- **Expense attachments / receipts** (photo or PDF).

Explicitly not planned: file hosting of full-resolution avatars (the 8×8 format is a deliberate GDPR-minimizing choice), account hard-delete (soft delete preserves other members' ledgers).

## Deployment note: HTTPS deviation

[BLUEPRINT.md](BLUEPRINT.md) states **"HTTPS only"**. The v1 LAN profile ships
**HTTP-only** for TrueNAS LAN use: the `dts_refresh` cookie uses `Secure=false`. For
internet-exposed deployments, terminate TLS at an upstream reverse proxy (Caddy,
Traefik, Cloudflare Tunnel), flip `COOKIE_SECURE=true`, and set `TRUSTED_PROXIES`
to the proxy's IP/CIDR so rate limiting and audit logs see the real client IP.
See [INSTALL.md](INSTALL.md#https--internet-exposure) for the full checklist.

## License & compliance

DoTheSplit is released under the [MIT License](LICENSE).

Third-party attribution lives in two places:

- [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md): generated list of every direct and transitive Go module and npm package with SPDX license + source link. Includes the Font Awesome CC BY 4.0 attribution.
- `/about` route in the running app: human-readable summary linked from the user menu in the header.

CycloneDX SBOMs (`sbom/server.cdx.json`, `sbom/frontend.cdx.json`) are attached as artifacts to every tagged GitHub Release, so auditors can ingest them into Dependency-Track, Trivy, OSV-Scanner, Grype, or any CycloneDX 1.5+ consumer.

Regenerate locally:

```bash
make compliance   # licenses + SBOMs
```
