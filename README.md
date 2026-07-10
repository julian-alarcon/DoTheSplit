<p align="center">
  <img src="logo.svg" alt="DoTheSplit logo" width="180" />
</p>

<h1 align="center">DoTheSplit</h1>

<p align="center">
  Open-source expense-sharing app. See <a href="BLUEPRINT.md">BLUEPRINT.md</a> for the product definition.
</p>

---

DoTheSplit runs on **SQLite** (default, zero-dependency single container) or
**PostgreSQL** (for multi-instance / scale-out), chosen by `DATABASE_DRIVER`.

### SQLite (default, simplest)

```bash
cp .env.example .env

# Generate the four encryption keys and append to .env. These protect every
# email and password - generate them ONCE and keep a copy somewhere safe.
# See "Secrets you must back up" below.
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
} >> .env

# One api container, one DB file on a volume. No Postgres, no migrate step,
# no separate worker (all run in-process).
docker compose -f docker-compose.sqlite.yml up -d --build
```

### PostgreSQL (scale-out)

```bash
# Same four keys as above, plus a Postgres password:
echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)" >> .env
# Update DATABASE_URL in .env so the password matches POSTGRES_PASSWORD.
# The compose file sets DATABASE_DRIVER=postgres on the api + worker services.

docker compose up -d
```

Open http://localhost:8080.

## Layout

- `/api`: Go 1.26 backend (standard-library `net/http`, oapi-codegen) plus a `worker` binary for recurring expenses. Persistence is behind a `repo.Store` abstraction with two engines - `api/internal/repo/postgres` (pgx/v5) and `api/internal/repo/sqlite` (modernc.org/sqlite, pure Go) - selected by `DATABASE_DRIVER`. The api binary also serves the embedded SPA.
- `/frontend`: Vue 3 + Vite single-page app (client-rendered, plain CSS), built to static files and embedded into the Go binary via `go:embed`
- `/docs/openapi.yaml`: API contract (source of truth, drives Go + TypeScript codegen)
- `/docs/DEVELOPMENT.md`, `/docs/FEATURES.md`: developer guide and feature catalogue
- `/docs/IMPORT.md`: importing a group (Splitwise or DoTheSplit CSV) and exporting one
- `/api/migrations`: append-only PostgreSQL 18 migrations (`golang-migrate`, paired `.up.sql` / `.down.sql`). SQLite migrations live in `/api/internal/repo/sqlite/migrations` and are embedded in the binary, applied in-process on first boot.
- `/docker-compose.yml`: local + LAN deployment stack
- `/scripts`: SBOM and third-party-license generators

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full build / test / deploy
guide and [INSTALL.md](INSTALL.md) for production install paths.

## Install on TrueNAS

Running on TrueNAS SCALE? See [INSTALL.md](INSTALL.md) for the Custom App
walkthrough: dataset layout for either engine, and consuming the first-run
setup token from the API logs.

## Container images

Tagged releases publish multi-arch (`linux/amd64`, `linux/arm64`) OCI images to
the GitHub Container Registry:

| Image                               | Tags                                 |
| ----------------------------------- | ------------------------------------ |
| `ghcr.io/julian-alarcon/dothesplit` | `X.Y.Z`, `X.Y`, `X`, `latest`, `dev` |

`:dev` always points at the latest commit on `main`. The api image embeds the
SPA and hosts both the `/api` and `/worker` entrypoints; in compose, override
`entrypoint: ["/worker"]` to run the worker. Pull a pinned release for
production:

```bash
docker pull ghcr.io/julian-alarcon/dothesplit:0.3.0
```

The running version is reported by `GET /healthz` (api) and the page footer,
so you can confirm what's deployed at a glance.

## Secrets you must back up

Three values in `.env` are **the** load-bearing secrets for this app:

| Variable          | What it does                                  | If you lose it                                  | If it leaks                                        |
| ----------------- | --------------------------------------------- | ----------------------------------------------- | -------------------------------------------------- |
| `EMAIL_ENC_KEY`   | AES-GCM key that encrypts every email at rest | Existing emails are unrecoverable               | Attacker can decrypt every email                   |
| `EMAIL_HMAC_KEY`  | HMAC key for email lookup hashes              | Login by email stops working for existing users | Attacker can enumerate which emails are registered |
| `PASSWORD_PEPPER` | Server-side pepper added before Argon2id      | Existing passwords are unrecoverable            | Attacker can crack stolen password hashes offline  |
| `JWT_SIGNING_KEY` | HS256 key signing SPA / native access tokens  | All token clients are logged out (recoverable)  | Attacker can mint valid access tokens for any user |

`POSTGRES_PASSWORD` (Postgres deployments only) is also sensitive but resettable later as long as you can reach the database. `JWT_SIGNING_KEY` is the least catastrophic to lose: rotating it only forces every token client to log in again.

These keys are required and identical for both engines: the encryption is application-level (emails AES-GCM, passwords Argon2id + pepper), done above the storage layer, so switching between SQLite and Postgres does not change what's protected. Neither engine encrypts the whole database file/volume.

**What this means for you:**

- Generate these once on first install. Don't regenerate on a rebuild - the database won't decrypt anymore.
- Store a copy in your password manager or secrets vault. Treat them like the master password to a vault: this app is the vault.
- When you back up the data volume (`dts_pg_data` for Postgres, or the `/data` volume holding `dts.db` for SQLite), back up the `.env` alongside it. A backup without the keys is useless.
- Never commit `.env`. It's gitignored for a reason.

## Development

```bash
make gen            # regenerate Go + TS API bindings from openapi.yaml
make migrate-up     # apply Postgres migrations (SQLite migrates in-process on boot)
make dev            # run api + frontend against a local postgres
make lint           # golangci-lint (Go) + eslint (SPA)
make test           # unit + integration tests
make test-go-both   # run the Go integration suite against both SQLite and Postgres
```

## Features

See [docs/FEATURES.md](docs/FEATURES.md) for the long-form description. In short:

- **Accounts**: register / login, display name + password change, verified email change (6-digit code), 8×8 pixel avatars (reducing privacy concerns on GDPR), soft-delete with stable tombstones.
- **First-run setup**: boot-time token gate so the first user is provably the operator.
- **Admin**: `/admin` area for users, groups, SMTP and audit, with step-up password prompts for destructive actions.
- **Groups**: create / rename / delete, **single currency per group** (multi-currency groups are intentionally unsupported, see [Roadmap](#roadmap) for the FX deferral), invites, leave, transfer ownership, default percent split for 2-member groups.
- **Expenses**: equal / exact / percent splits, ten categories, custom date, optional free-text notes, full edit history with per-member split diffs, reversible soft-delete (any member can restore from the expense's detail page).
- **Balances & settle-up**: net balances, simplified "X owes Y" view, settlements in a paginated transaction feed with detail pages. Pick who is paying when settling up; any member can later edit from / to / amount / note / date, or soft-delete and restore.
- **Recurring expenses**: daily / weekly / biweekly / monthly / yearly templates materialized by a background worker (UI shipped).
- **Real-time updates**: the group dashboard subscribes to a per-group Server-Sent Events stream (`/v1/groups/{id}/events`) and re-fetches affected views the moment any member (or the worker) creates, edits, deletes, or restores an expense or settlement. Only minimal id/action signals ride the channel, never amounts or notes.
- **Activity log & notifications**: an append-only per-group activity log with unread tracking, plus opt-in email notifications (recurring run, settlement recorded, added to a group), all off by default and gated on SMTP being configured.
- **Themes & PWA**: light / dark / high-contrast themes (per-device, applied pre-paint), and an installable PWA with read-only offline (network-first `/v1` GET cache, offline banner, mutations short-circuited offline).
- **Search**: cross-group substring search over expense descriptions / notes and settlement notes, with collapsible Group and Category filters. The category picker only lists categories present in the current result set.
- **Import & export**: CSV in / out via `/import` (Splitwise or DoTheSplit) and group settings → Export. The DoTheSplit format keeps the Splitwise prefix and adds `Time`, `Payer`, `Notes`, `Created`, `CreatedBy`, so a round-trip preserves second-precision timestamps, explicit payers, and per-expense notes.
- **Security**: Argon2id, AES-GCM email at rest, rate-limited auth + setup, strict JSON bodies, hashed-inline CSP, password confirmation for self-delete.
- **API**: OpenAPI 3.0.3 contract at [docs/openapi.yaml](docs/openapi.yaml); every business endpoint is under `/v1/...`.

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

- **Backup**
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

CycloneDX SBOMs (`sbom/api.cdx.json`, `sbom/worker.cdx.json`, `sbom/frontend.cdx.json`) are attached as artifacts to every tagged GitHub Release, so auditors can ingest them into Dependency-Track, Trivy, OSV-Scanner, Grype, or any CycloneDX 1.5+ consumer.

Regenerate locally:

```bash
make compliance   # licenses + SBOMs
```
