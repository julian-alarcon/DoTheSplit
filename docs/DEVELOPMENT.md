# Development guide

How to check, build, test, and deploy DoTheSplit. See [../BLUEPRINT.md](../BLUEPRINT.md) for product scope and [../CLAUDE.md](../CLAUDE.md) for the conventions we enforce when editing the code.

## Prerequisites

- **Docker** + **Docker Compose v2** (only strict requirement for running the stack)
- **Go 1.26+** (for local dev and unit tests outside Docker)
- **Node 24+** and **npm 10+** (for local dev and the SPA build / type-check)
- `make`, `openssl` (for key generation), `curl` and `python3` (used in smoke scripts)

## First-time setup

```bash
cp .env.example .env
echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"   >> .env
echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"  >> .env
echo "PASSWORD_PEPPER=$(openssl rand -base64 32)" >> .env
echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)" >> .env

docker compose up -d --build
```

The app and API are both served by the server binary at <http://localhost:8080> (the Vue SPA is embedded in the Go binary). Health probes: `/healthz`, `/readyz`.

## Repository layout

- `/server`: Go 1.26 backend (standard-library `net/http`, oapi-codegen) with a single binary (`server/cmd/server`) that serves the API, the embedded SPA, and the in-process recurring-expense worker. Persistence is behind a `repo.Store` abstraction with two engines - `server/internal/repo/postgres` (pgx/v5) and `server/internal/repo/sqlite` (modernc.org/sqlite, pure Go) - selected by `DATABASE_DRIVER`.
- `/frontend`: Vue 3 + Vite single-page app (client-rendered, plain CSS), built to static files and embedded into the Go binary via `go:embed`.
- `/docs/openapi.yaml`: API contract (source of truth, drives Go + TypeScript codegen).
- `/docs/DEVELOPMENT.md`, `/docs/FEATURES.md`: developer guide and feature catalogue.
- `/docs/IMPORT.md`: importing a group (Splitwise or DoTheSplit CSV) and exporting one.
- `/server/migrations`: append-only PostgreSQL 18 migrations (`golang-migrate`, paired `.up.sql` / `.down.sql`). SQLite migrations live in `/server/internal/repo/sqlite/migrations` and are embedded in the binary, applied in-process on first boot.
- `/docker-compose.yml`: default (SQLite) deployment stack; `/docker-compose.postgres.yml` is the Postgres stack (both run one `dothesplit` service).
- `/scripts`: SBOM and third-party-license generators.

## Database engines (dual)

DoTheSplit speaks to **either SQLite or PostgreSQL** behind one `repo.Store` interface, with two implementations:

- `server/internal/repo/postgres` - pgx/v5.
- `server/internal/repo/sqlite` - `database/sql` + the pure-Go `modernc.org/sqlite` driver (no CGO). Connection pragmas: WAL, `busy_timeout`, `foreign_keys=ON`, `synchronous=NORMAL`.

The `DATABASE_DRIVER` env var selects the engine (`sqlite` or `postgres`); it is **required with no default**, so `config.Load()` errors if it is unset. `server/internal/storefactory` constructs the right `Store` and wires migrations. Because both implementations satisfy the same interface, the service/handler layers are engine-agnostic.

Encryption at rest is application-level (above the DB) and identical on both engines - the engine choice never touches the crypto (see [What the keys do](#what-the-three-keys-do)).

## The contract-first workflow

Any change that touches the HTTP surface goes through the same loop:

1. Edit [docs/openapi.yaml](openapi.yaml) first.
2. `make gen` - regenerates Go types (`server/internal/apigen/`) and TypeScript types (`frontend/src/lib/api/schema.d.ts`). The build won't compile until your code matches.
3. If the DB schema changes, add a migration for **both** engines. Postgres migrations live under [server/migrations/](../server/migrations/) - `NNNN_*.up.sql` + a matching `.down.sql`, applied by golang-migrate (`make migrate-up` or the `migrate` one-shot container). SQLite migrations live under [server/internal/repo/sqlite/migrations](../server/internal/repo/sqlite/migrations/), embedded in the binary and applied in-process at boot (no make target, no migrate container). Migrations are append-only.
4. Backend order: **repo → service → handlers → router**.
5. Frontend: add a typed call in a composable under `frontend/src/composables/` and the view/route that uses it. Never hand-write fetch/URLs - go through the `openapi-fetch` client.
6. Tests, rebuild containers, smoke.

## Check (fast, no containers needed)

```bash
make gen            # regenerate Go + TS types from docs/openapi.yaml
make lint           # golangci-lint (Go) + eslint (SPA) - both gate CI
cd server && go vet ./...
cd server && go build ./...
cd frontend && npm run check    # vue-tsc type-check
cd frontend && npm run build    # vite build (static bundle the api embeds)
```

`vue-tsc` / `vite build` should produce 0 errors.

## Test

**Go**: unit + integration. The integration suite runs against **both engines**, selected by `TEST_DB_DRIVER`:

- `postgres` uses [testcontainers-go](https://golang.testcontainers.org/), so a running Docker daemon is required. Each run spins up its own short-lived Postgres container, applies all migrations, and tears it down.
- `sqlite` runs fully in-process (embedded migrations, no Docker needed).

```bash
make test-go                         # Go tests, Postgres engine (default)
make test-go-sqlite                  # Go tests, SQLite engine (in-process, no Docker)
make test-go-postgres                # Go tests, Postgres engine (testcontainers)
make test-go-both                    # run against both engines
cd server && go test ./... -race        # same as make test-go, one level down
cd server && go test ./internal/server/ -run TestGoldenPath -v    # one test
```

CI runs the integration suite against both engines on every PR.

The E2E suite in [server/internal/server/server_test.go](../server/internal/server/server_test.go) covers the full golden path (register, login, group, members, expense split modes, balances, settlements, soft-delete, category + revision log, payer swap, logout).

**App (Vue SPA)**: two layers.

- Unit (vitest, jsdom) under `frontend/src/**/*.test.ts`. Pure helpers only - the canvas-touching avatar pipeline isn't exercised here, only its color math. Run with `cd frontend && npm test` (or `npm run test:watch`).
- E2E (Playwright, Chromium) under `frontend/tests/e2e/`. Requires the full docker stack already running and the install token from `docker compose logs dothesplit`. The SPA is served by the app on `:8080`, so there's a single origin to target:

  ```bash
  docker compose up -d --build
  TOKEN=$(docker compose logs dothesplit | grep -oE 'token=[A-Za-z0-9_-]+' | head -1 | cut -d= -f2)
  SETUP_TOKEN=$TOKEN make test-e2e
  ```

  CI runs the same flow on every PR; locally it's optional, useful when changing SPA-to-API wiring.

## Build the container images

For local dev, build via compose. **Production deployments should pull pinned images from GHCR** (see "Releasing" below) - never build from `main` on a deployment host.

```bash
docker compose build                 # build the dothesplit image
docker compose build dothesplit      # rebuild just the app
make up                              # rebuild + start, stamping BUILD_COMMIT + BUILD_VERSION
```

Images:

- `dothesplit` - multi-stage build: a Node stage builds the Vue SPA, a Go stage copies the bundle into the embed dir and compiles the binary, and a distroless final stage runs the single `/dothesplit` binary. `BUILD_VERSION`/`BUILD_COMMIT` reach the SPA via Vite `define` (footer) and the Go binary via `-ldflags` (`/healthz`). One image serves the API, the frontend, and the in-process worker.

The `make up` target reads `frontend/package.json` (release-please-managed) for `BUILD_VERSION` and the current git short SHA for `BUILD_COMMIT`.

## Releasing

Releases are automated by [release-please](https://github.com/googleapis/release-please-action) on every push to `main`. You don't run release commands by hand - you write conventional commits and merge the Release PR when it looks right.

### The flow

1. **Land changes on `main`** with [conventional commit titles](https://www.conventionalcommits.org). Commit type drives the bump:

   | Type                                                     | Bump  | Example                                           |
   | -------------------------------------------------------- | ----- | ------------------------------------------------- |
   | `fix:`                                                   | patch | `fix(api): reject empty currency on group create` |
   | `feat:`                                                  | minor | `feat(app): currency picker flag glyphs`          |
   | `feat!:` / `BREAKING CHANGE` footer                      | major | `feat(api)!: drop /v1/legacy/expenses`            |
   | `chore:`, `docs:`, `style:`, `test:`, `ci:`, `refactor:` | none  | (still appears in CHANGELOG under their section)  |

2. **release-please opens (or updates) a Release PR** named `chore(main): release X.Y.Z`. It bumps `frontend/package.json` (the single version source of truth) and regenerates `CHANGELOG.md`. Review it like any other PR. If you don't like the proposed version, override via a commit footer (`Release-As: 1.2.0`) and push - the PR will rewrite itself.

3. **Merging the Release PR** auto-creates the git tag `vX.Y.Z` and a GitHub Release with the changelog body.

4. **The tag triggers two workflows in parallel**:
   - [`publish.yml`](../.github/workflows/publish.yml) builds the multi-arch (`linux/amd64,linux/arm64`) image (which embeds the SPA), pushes to `ghcr.io/julian-alarcon/dothesplit` with tags `:X.Y.Z`, `:X.Y`, `:X`, `:latest`, plus a build provenance attestation.
   - [`compliance.yml`](../.github/workflows/compliance.yml) regenerates SBOMs + `THIRD_PARTY_LICENSES.md` and attaches them to the GitHub Release.

5. **Every push to `main`** (including merges that are not the Release PR) also triggers `publish.yml`, which pushes `:dev`, `:main`, and `:sha-<short>` tags. The `:dev` tag tracks the latest `main` and is appropriate for a staging environment.

### Where the version surfaces

| Location                          | Source                                                |
| --------------------------------- | ----------------------------------------------------- |
| `frontend/package.json` `version` | release-please bump on merge (single source of truth) |
| GitHub Release page               | release-please on PR merge                            |
| `ghcr.io/.../dothesplit:X.Y.Z`    | `publish.yml` on tag                                  |
| API `GET /healthz` JSON           | `-ldflags` baked in by `server/Dockerfile`            |
| SPA page footer                   | `VITE_BUILD_VERSION` baked in by `server/Dockerfile`  |

### Emergency manual release

Only when release-please is broken or the queued Release PR can't be merged in time:

```bash
# 1. Bump frontend/package.json + .release-please-manifest.json BY HAND, commit.
# 2. Tag and push.
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

The tag still triggers `publish.yml` and `compliance.yml`. The CHANGELOG entry won't be auto-generated, so write it manually in the GitHub Release UI. **Do not push manual version bumps to `main` outside a release-please PR** - it desyncs the manifest and the next automated PR will produce a wrong version.

## Run

Bare `docker compose` targets the default **SQLite** stack (`docker-compose.yml`). For the Postgres stack, add `-f docker-compose.postgres.yml` to each command:

```bash
docker compose up -d                 # start/resume the stack
docker compose up -d --build         # rebuild only stale images, then start
docker compose up -d --build dothesplit  # rebuild + restart just the app (also rebuilds the embedded SPA)
docker compose logs -f dothesplit    # follow app logs
docker compose ps                    # service status
docker compose down                  # stop (keeps the DB volume)
docker compose down -v               # stop AND destroy the DB volume
```

### Services

The default stack is **SQLite** (`docker-compose.yml`, `DATABASE_DRIVER=sqlite`): a single `dothesplit` container plus one DB-file volume at `/data` - no `postgres`, `migrate`, or separate worker service. Migrations apply in-process at boot and the worker runs in-process (see below).

The **Postgres** stack (`docker-compose.postgres.yml`, which sets `DATABASE_DRIVER: postgres` on `dothesplit`) has three services:

| Service      | Image                | Purpose                                                    |
| ------------ | -------------------- | ---------------------------------------------------------- |
| `postgres`   | `postgres:18-alpine` | Database; mounted at `/var/lib/postgresql`                 |
| `migrate`    | `migrate/migrate`    | One-shot; runs all `*.up.sql` and exits                    |
| `dothesplit` | `dothesplit`         | HTTP API + embedded Vue SPA + in-process worker on `:8080` |

### Worker topology

The recurring-expense worker runs as an in-process goroutine inside the `dothesplit` server, started at boot on both engines. There is no `WORKER_MODE` and no separate worker container: the app is single-node by design, so the tick runs unguarded (no advisory lock). This matches how comparable self-hosted apps (memos, vikunja) run their background jobs. For multi-replica Postgres scale-out you would need to reintroduce a singleton guard before running the tick on more than one instance.

## Smoke test the running stack

```bash
# Stack health
curl -fsS http://localhost:8080/healthz     # 200 ok
curl -fsS http://localhost:8080/readyz      # 200 once DB is reachable

# End-to-end user flow
JAR=/tmp/c
curl -sS -c $JAR -X POST http://localhost:8080/v1/auth/register \
  -H 'content-type: application/json' \
  -d '{"email":"dev@test.dev","password":"password12","display_name":"Dev"}'

curl -sS -b $JAR http://localhost:8080/v1/me
curl -sS -b $JAR http://localhost:8080/v1/categories | python3 -m json.tool
```

Then open <http://localhost:8080/login>, log in with the credentials you just created, and walk through create-group → add-expense → edit-expense.

## Deploy (LAN / TrueNAS)

Operator-facing install steps live in [../INSTALL.md](../INSTALL.md). What
follows is the cryptographic background and update mechanics worth knowing
when changing the deploy.

### What the three keys do

The three `EMAIL_ENC_KEY` / `EMAIL_HMAC_KEY` / `PASSWORD_PEPPER` values are not config knobs - they're the cryptographic material the database is built around. Generate them once on first install and back them up; if you lose them the data is unrecoverable, and if they leak an attacker can decrypt every email and crack every password offline.

All three are 32 raw bytes, base64-encoded for transport. `openssl rand -base64 32` produces exactly that.

#### `EMAIL_ENC_KEY` - emails at rest

Code: [server/internal/crypto/email.go](../server/internal/crypto/email.go).

Every email address goes into the `users.email_encrypted` column as `key_id ‖ nonce ‖ AES-GCM(EMAIL_ENC_KEY, plaintext)`:

- **`key_id`** is a one-byte tag (currently `0x01`) that lets you rotate to a new key later without losing access to rows encrypted under the old one.
- **`nonce`** is 12 random bytes generated per row - required for AES-GCM, and the reason two users with the same email get two different ciphertexts.
- **AES-GCM** is authenticated encryption: the auth tag is appended after the ciphertext, so any tampering with the row (or with `key_id` / `nonce`) makes decryption fail rather than producing garbage plaintext.

The plaintext is only kept in memory for the duration of a request (e.g. when rendering an email template, when an admin views the user detail page, or when the SMTP outbox dispatcher mails it). The access logger ([server/internal/middleware/logger.go](../server/internal/middleware/logger.go)) never logs request bodies, only method, path, status, duration, client IP, and request id, so emails and passwords can't leak through it.

#### `EMAIL_HMAC_KEY` - login lookups without storing the address

You can't query "user with email X" against an AES-GCM column - every row has a different nonce, so ciphertexts don't match even when plaintexts do. We store a _separate_ deterministic fingerprint in `users.email_hash`:

```
email_hash = HMAC-SHA256(EMAIL_HMAC_KEY, normalize(email))
```

`normalize` lower-cases and trims (see `EmailCipher.HashEmail`). The HMAC is keyed, so an attacker who steals the database without the key can't brute-force the (small, finite) email space against `users.email_hash` - they have to break HMAC-SHA256 first.

Login, register-conflict-detection, password-reset and "is this email already on file" all hash the input email and look it up by `email_hash`. The encrypted column is decrypted only after that lookup succeeds.

Splitting the two keys is deliberate: it means a leak of `EMAIL_HMAC_KEY` lets an attacker test whether _specific_ emails are registered (still bad), but they still can't read any email plaintext without `EMAIL_ENC_KEY`. And vice-versa.

#### `PASSWORD_PEPPER` - server-side secret added to password hashes

Code: [server/internal/crypto/password.go](../server/internal/crypto/password.go).

Passwords are hashed with Argon2id (memory-hard, GPU-resistant), but Argon2id alone protects against an attacker with the database _and_ nothing else. If they also walk away with the binary they can run dictionary attacks at full speed against the salted hashes. The pepper closes that gap:

```
hash = Argon2id(password ‖ PASSWORD_PEPPER, salt, params)
```

The pepper is stored only in the env var - never in the database. So an attacker who exfiltrates `users.password_hash` and the salts but not the pepper can't even start cracking; they're missing 32 bytes of unguessable entropy that get mixed into every hash. The pepper is used at register, login, and `/me/password` change.

Salt + pepper + Argon2id is a three-part defense (per-user randomness, server-secret randomness, slow KDF). Take any one away and the others get weaker.

#### Rotation, when you'd actually do it

Today there's no rotation tool - that's a deliberate v1 cut. If a key leaks, the recovery is "mint a new key, dump and re-encrypt every affected row, then deploy with the new key." The `key_id` byte in the email ciphertext exists so a future rotation tool can read the old key for old rows and the new key for new rows during the cutover. None of that exists yet - if you suspect a key has leaked, the safe path today is to take the instance down, restore from a clean snapshot, rotate the key, and have users reset passwords.

### Updating a running deployment

For production deployments, **pull pinned images from GHCR** instead of building from `main` on the host. The release pipeline already built and signed them.

```bash
# On the deployment host, after a new release is tagged:
docker compose pull                  # pull the pinned :X.Y.Z (or :latest) images
docker compose up -d                 # restart services with the new images
```

Migrations run automatically via the `migrate` one-shot on every `up`.

For staging, point compose at the `:dev` tag (which tracks `main`) and pull on a schedule, or wire a webhook to your registry. For dev hosts where you want to reflect uncommitted local changes, the legacy build-from-source path still works:

```bash
git pull
docker compose up -d --build         # rebuild only changed services
```

But never do that on a release-tracking deployment - it desyncs from the version stamps in the image and breaks the "what's running?" answer at `/healthz`.

### Major Postgres upgrades

Postgres's on-disk format is not compatible across major versions. Bumping the image tag alone will not work - the container will refuse to start and log "there appears to be PostgreSQL data in /var/lib/postgresql/data (unused mount/volume)" or similar.

Two paths:

1. **Dev / LAN with no data you care about**: `docker compose -f docker-compose.postgres.yml down -v` (destroys the volume), then `docker compose -f docker-compose.postgres.yml up -d --build`. Migrations recreate the schema from scratch; all app data is lost.
2. **Preserving data**: `pg_dump` from the old container, `docker compose -f docker-compose.postgres.yml down -v`, bring up the new image, restore with `psql` or `pg_restore`.

The Postgres compose file mounts the volume at `/var/lib/postgresql` (not `/var/lib/postgresql/data`) because PG 18's official image expects the parent directory so `pg_upgrade --link` can work in place across majors. Don't change this.

SQLite has no cross-version on-disk incompatibility to manage - the `dts.db` file is portable and embedded migrations apply in-process on the next boot, so a SQLite deployment just bumps the image tag.

## Troubleshooting

### Login does nothing / bounces back to /login

The SPA logs in via `POST /v1/auth/token` (bearer), keeping the access token in memory and a rotating refresh token in the httpOnly `dts_refresh` cookie. On reload, `useAuth.boot()` calls `/v1/auth/refresh` to restore the session. If login appears to no-op: check the network tab for a failing `/v1/auth/token` (bad credentials → 401, unverified email → 403) and confirm the `dts_refresh` cookie is being set (it won't be over plain HTTP if `COOKIE_SECURE=true`).

### A form control doesn't "do" anything / a script didn't run

The SPA ships a strict CSP (`script-src 'self'`, no inline scripts). All JS is bundled by Vite and served same-origin, so this should never bite normal Vue code. The one hand-placed script is [frontend/public/theme-boot.js](../frontend/public/theme-boot.js), referenced from `index.html` without `defer` - it must stay a same-origin file, never an inline block. Inline SVG markup (the `Icon`/`CategoryIcon` components) is allowed; only inline `<script>`/`<style>` are blocked.

### Test container fails to pull Postgres image

`testcontainers-go` uses the same Docker daemon as compose. If compose works, tests will too. If not: `docker info` and `docker pull postgres:18-alpine` to prime the image cache.

### The api serves a stale SPA / a placeholder page

The Go binary embeds whatever is in `server/internal/webui/dist/` at compile time. A fresh `go build` without the bundle serves a code fallback page. `make build` (and the Docker build) rebuild the SPA and copy it in first; if you built the binary by hand, run `make embed-frontend` then rebuild.

## Useful targets

Run `make help` for the full list. The ones you'll actually reach for:

| Target                  | What it does                                                                            |
| ----------------------- | --------------------------------------------------------------------------------------- |
| `make gen`              | Regenerate Go + TS API bindings from `docs/openapi.yaml`                                |
| `make lint`             | Lint everything: golangci-lint (Go) + eslint (SPA)                                      |
| `make migrate-up`       | Apply all pending Postgres migrations (SQLite migrations are embedded, applied at boot) |
| `make test-go`          | Full Go test suite, Postgres engine (unit + integration via testcontainers)             |
| `make test-go-sqlite`   | Go test suite against SQLite (in-process, no Docker)                                    |
| `make test-go-postgres` | Go test suite against Postgres (testcontainers)                                         |
| `make test-go-both`     | Go test suite against both engines                                                      |
| `make test-e2e`         | Playwright e2e (needs the stack up + `SETUP_TOKEN`)                                     |
| `make dev-api`          | Run the Go API locally against Docker Postgres                                          |
| `make dev-frontend`     | Run the Vite dev server (proxies `/v1` to the local API)                                |
| `make build`            | Build the SPA, embed it, then build the Go binary (`bin/dothesplit`)                    |
| `make up`               | `docker compose up -d --build`, baking current SHA in (default SQLite stack)            |
| `make compliance`       | Regenerate `THIRD_PARTY_LICENSES.md` + CycloneDX SBOMs into `sbom/`                     |

**`make up`** computes `BUILD_COMMIT=$(git rev-parse --short HEAD)` and `BUILD_VERSION=$(node -p "require('./frontend/package.json').version")` and passes both to the server Dockerfile as build args. The SPA gets them via Vite `define` (`import.meta.env.VITE_BUILD_*`) and [`AppLayout.vue`](../frontend/src/components/AppLayout.vue) renders a footer with the version (linking to the GitHub Release) and the commit (linking to the commit page). The server binary gets them via `-ldflags` and surfaces them at `GET /healthz`. When building outside a git checkout, both default to `dev` and the surfaces show `dev` with no links.
