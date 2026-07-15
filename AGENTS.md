# CLAUDE.md

Project-specific instructions for Claude Code. Loaded automatically for every session in this repo.

See also: [BLUEPRINT.md](BLUEPRINT.md) for product scope and [README.md](README.md) for local setup.

## What this project is

DoTheSplit - a expense-sharing app.

- **Backend**: Go 1.26, standard-library `net/http` (the 1.22+ `ServeMux` with method+wildcard patterns), pgx/v5, `golang-migrate`, `oapi-codegen`. No web framework. Source in [server/](server/) (module `github.com/julian-alarcon/dothesplit/server`, single binary [server/cmd/server/](server/cmd/server/)). The server binary serves the API, the embedded SPA (see "Frontend" below), and the in-process background worker.
- **Frontend**: Vue 3 (Composition API, `<script setup>` SFCs) + Vite, client-side-rendered. TailwindCSS + PlainCSS when needed (no other UI library) - design tokens + the `.field-*`/`.btn-*` system live in [frontend/src/styles/global.css](frontend/src/styles/global.css); per-view styles are scoped `<style>` blocks. Source in [frontend/](frontend/). Built to static files and embedded into the Go binary via `go:embed` ([server/internal/webui/](server/internal/webui/)), so there is one container, not two.
- **Auth**: JWT bearer tokens for all clients (SPA + native). `POST /v1/auth/token` exchanges credentials for a short-lived access token (sent as `Authorization: Bearer`) plus a rotating refresh token in the httpOnly `dts_refresh` cookie; `POST /v1/auth/refresh` rotates it. The `mw.Bearer` middleware attaches the authenticated user to the request context (`middleware.WithUser`, read via `middleware.User(r.Context())`), so `RequireSession`/`RequireAdmin` gate every authenticated route. (There is no cookie-session auth: the old Astro SSR `dts_session` flow was removed in migration `0004`.)
- **Database**: SQLite **or** PostgreSQL 18, selected by `DATABASE_DRIVER` (**required, no default** - the app refuses to boot if unset; `sqlite` or `postgres`). Postgres additionally needs `DATABASE_URL`. Postgres migrations in [server/migrations/](server/migrations/); SQLite migrations are embedded in [server/internal/repo/sqlite/migrations/](server/internal/repo/sqlite/migrations/) and applied in-process on first boot. See "Database" below.
- **Worker**: recurring-expense materialization + email-outbox drain, ticking every 60s ([server/internal/worker/](server/internal/worker/)). Runs as an in-process goroutine started by the server at boot on both engines: the app is single-node by design, so there is no separate worker process and the tick runs unguarded.
- **Infra**: Docker Compose on TrueNAS LAN (HTTP-only).

## The golden rule: contract-first

**[docs/openapi.yaml](docs/openapi.yaml) is the source of truth.** Go server types, a Go client for integration tests, and TypeScript types for the SPA client are all generated from it.

**Order for any user-facing change:**

1. Edit [docs/openapi.yaml](docs/openapi.yaml) first - schemas, paths, responses.
2. Run `make gen` to regenerate Go (`server/internal/apigen/`) and TypeScript (`frontend/src/lib/api/schema.d.ts`) types. The build won't compile until the code matches.
3. Add a migration if the DB schema changes ([server/migrations/NNNN\_\*.up.sql](server/migrations/) + matching `.down.sql`).
4. Wire the backend in this exact order: **repo → service → handlers → router**.
5. Wire the SPA against the new generated types: add a typed call in a composable ([frontend/src/composables/](frontend/src/composables/)) and the view/route that uses it. Never hand-write fetch calls or URLs - go through the `openapi-fetch` client in [frontend/src/lib/api/client.ts](frontend/src/lib/api/client.ts).
6. Update the worker only if the recurring flow is affected - it reuses services, so most changes propagate for free.
7. Build, test, rebuild the container (the SPA is embedded in the image, so a frontend change still means rebuilding the `dothesplit` image).

Don't change generated files by hand - re-run `make gen` instead.

### OpenAPI conventions we enforce

- **Spec version**: `3.0.3`. We'd use 3.1 but `oapi-codegen` doesn't fully support it yet - keep it on 3.0.3 until that changes.
- **API versioning**: all business endpoints live under `/v1/…`. Health probes (`/healthz`, `/readyz`) are _not_ versioned. Breaking changes cut a new `/v2`, run both mounts in parallel during migration, then retire `/v1` when clients are gone.
- **Request bodies**: must have `additionalProperties: false`. Unknown fields are a 400 - typos should fail loudly, not silently.
- **Error responses**: always reference the named `components.responses.{BadRequest,Unauthorized,Forbidden,NotFound,Conflict,TooManyRequests,ServiceUnavailable}` - never inline `schema: { $ref: ".../Error" }` under a status code.
- **Examples**: add an `example:` to request schemas that anyone would want to try in a docs viewer (create/login flows at minimum).
- **Tags**: every operation has a tag; every tag has a description at the top of the spec.
- **`operationId`**: camelCase verb-object (`createGroup`, `listExpenses`); drives the generated function name in both Go and TS clients.

## Backend layering (strict)

```
handlers → services → repositories → DB
```

- **Handlers** ([server/internal/handlers/](server/internal/handlers/)): bind JSON, call services, translate errors to HTTP status codes. No business logic. Use `errors.Is` on service sentinels.
- **Services** ([server/internal/service/](server/internal/service/)): validate, orchestrate, enforce invariants. Return sentinel errors (`ErrNotMember`, `ErrBadSplit`, etc.). Use transactions for anything that writes more than one table.
- **Repositories** ([server/internal/repo/](server/internal/repo/)): the `repo` package defines engine-neutral domain types + interfaces (`repo.Store`, `repo.Tx`, one interface per table) in [store.go](server/internal/repo/store.go)/[types.go](server/internal/repo/types.go). Two implementations satisfy them: [server/internal/repo/postgres/](server/internal/repo/postgres/) (pgx/v5) and [server/internal/repo/sqlite/](server/internal/repo/sqlite/) (`database/sql` + modernc.org/sqlite). Services depend only on the interfaces; [storefactory](server/internal/storefactory/) builds the right `Store` from `DATABASE_DRIVER`. No domain rules in repos; each maps its no-rows error → `repo.ErrNotFound` and its unique-violation → `repo.ErrConflict`. A `...Tx` method takes a `repo.Tx` (the engine unwraps it to its native `pgx.Tx`/`*sql.Tx`); a nilable `tx` means "run on the pool".
- **Router** ([server/internal/server/router.go](server/internal/server/router.go)): register endpoints on a `net/http.ServeMux` with method+pattern routes (`"POST /v1/groups/{id}/expenses"`, read params via `r.PathValue`); compose global + per-route middleware with `mw.Chain`, gating authenticated routes with `mw.RequireSession()` (and `mw.RequireAdmin()` for admin). Middleware are `func(http.Handler) http.Handler`; the authenticated user and request id travel through the request context, not a framework context.

Rules of thumb:

- Always call `GroupService.RequireMember` (or equivalent `IsMember`) before reading/writing group-scoped data.
- Expense creation must validate: split-sum invariant, payer ∈ members, all split users ∈ members, mode matches supplied values. All inside one tx.
- Currency is optional on the wire. Empty string means "use the group's `default_currency`" - the service layer looks it up.
- Soft-delete via `deleted_at` for expenses and settlements; every _list_ read filters `WHERE deleted_at IS NULL` or joins with that filter. The single-item `ExpenseService.Get` / `SettlementService.Get` deliberately return soft-deleted rows (with `deleted_at` set) so the detail page can render a read-only "deleted" view and offer Restore; `Update`/`Delete` keep their own `deleted_at` guards so a deleted item still can't be edited or double-deleted.
- Soft-delete is reversible: `POST /v1/expenses/{id}/restore` and `POST /v1/settlements/{id}/restore` clear `deleted_at` (any group member) and write an `expense.restored` / `settlement.restored` activity event. Restoring an already-active row is a `409`. The `activity_events.action` CHECK constraint must list every action.

## Database

Two engines behind the `repo.Store` abstraction, chosen by `DATABASE_DRIVER` (**required, no default**: `sqlite` or `postgres` - `Load()` errors if unset). Pick per deployment: SQLite for a single-node, single-file, zero-dependency install (`DATABASE_URL` defaults to `file:./dts.db`); Postgres for multi-instance / scale-out (also set `DATABASE_URL`).

**Portability rules (both engines must stay in lockstep):**

- ID/timestamp generation differs by engine but the domain result is identical. Postgres keeps its DB defaults (`gen_random_uuid()`, `now()`) and reads them back via `RETURNING` (the original migration is untouched). SQLite's schema has **no** such defaults - its repos generate `uuid.New()` / `time.Now().UTC()` in Go and insert them explicitly. When adding a column, keep this split: pg default + `RETURNING`, sqlite Go-generated.
- SQLite stores UUID/JSON/timestamps as TEXT, BYTEA as BLOB, BIGINT/SMALLINT as INTEGER, BOOLEAN as INTEGER 0/1. Timestamps are RFC3339Nano UTC text so lexical order == chronological order (keyset pagination relies on it) - use the `tsVal`/`scanTS` helpers, never scan a TEXT column straight into `time.Time` (modernc can't).
- Engine-divergent SQL lives only in the two repo packages: JSON reads use `->>'k'` (pg) vs `json_extract(col,'$.k')` (sqlite); `= ANY($1)` (pg) vs `IN (?,?,…)` (sqlite); `ILIKE` (pg) vs `lower() LIKE lower()` (sqlite); row locking (`FOR NO KEY UPDATE SKIP LOCKED`, `FOR UPDATE`) is pg-only and simply omitted on sqlite's single writer.
- **Never run a pool (non-`Tx`) write inside an open `Store.Begin` transaction** - on SQLite the single writer self-deadlocks. Use the `...Tx` variant so the write joins the transaction (also correct/atomic on Postgres).

**Postgres specifics:**

- PostgreSQL **18** in compose. The volume mounts at `/var/lib/postgresql` (the parent, not `/var/lib/postgresql/data` like in PG 16) because PG 18's image stores data in a version-specific subdir (`/var/lib/postgresql/18/docker/`) so `pg_upgrade --link` can work across majors. Mounting at `data` makes the container fail to start.
- Major-version upgrades require `pg_upgrade` or `pg_dump`/`pg_restore`. A plain image bump leaves the old data files unreadable.
- Migrations ([server/migrations/](server/migrations/)) are append-only. Never edit a committed `*.up.sql`; add a new migration. Every migration needs a matching `.down.sql`. Apply with `make migrate-up` or the Docker `migrate` one-shot. Real-time uses a `pg_notify` trigger + LISTEN (see [realtime](server/internal/realtime/)).

**SQLite specifics:**

- Migrations ([server/internal/repo/sqlite/migrations/](server/internal/repo/sqlite/migrations/)) are embedded via `go:embed` and applied in-process on first boot - there is no separate migrate container. Keep them a faithful translation of the Postgres schema; the category seed uses fixed UUIDs (stable across both engines). Same append-only rule.
- The connection DSN sets `foreign_keys(ON)` (FK enforcement is off by default per-connection - the group-delete cascade depends on it), `journal_mode(WAL)`, `busy_timeout`, `synchronous(NORMAL)`.
- No LISTEN/NOTIFY: the store publishes committed activity events to the in-process realtime hub after commit (`repo.ActivityPublisher`), so real-time only works within one process - consistent with the single-node, in-process worker.

**Shared:** Keep FK cascades explicit (group deletion cascades to `group_members`, `expenses`→`splits`, `settlements`, `recurring_expenses`, `activity_events`). Amounts are integer cents; IDs are UUIDs.

## Worker topology

The 60s tick (materialize due recurring expenses + drain the email outbox) is one code path ([server/internal/worker/](server/internal/worker/)) that runs as an in-process goroutine inside the server, started at boot by [server/cmd/server/main.go](server/cmd/server/main.go) (`go worker.Run(...)`) on both engines. There is no `WORKER_MODE` and no separate worker container.

The app is **single-node by design** (like memos and vikunja), so the tick runs unguarded: no advisory lock, no cross-process coordination. If you ever need multi-replica Postgres scale-out, you must reintroduce a singleton guard (e.g. a `pg_advisory_lock`) before running the tick on more than one instance, or run exactly one instance with the worker.

## Frontend conventions

- **Client-side-rendered Vue SPA** ([frontend/src/](frontend/src/)). Routes in [frontend/src/router/index.ts](frontend/src/router/index.ts); the `beforeEach` guard mirrors the old Astro middleware (first-run setup funnel, boot-refresh wait, auth gate, admin gate). Auth state is a module-singleton reactive composable, [useAuth.ts](frontend/src/composables/useAuth.ts) - the one piece of genuinely shared cross-route state. No Pinia, no TanStack Query: hand-rolled composables wrap the typed client until that becomes painful.
- **Typed API access only**: every call goes through the `openapi-fetch` client ([frontend/src/lib/api/client.ts](frontend/src/lib/api/client.ts)), which injects the bearer token and does a single-flight refresh + replay on 401. Wrap calls in a composable; never hand-write `fetch`/URLs. `import.meta.env.VITE_*` is the only env surface (e.g. `VITE_API_BASE_URL`, empty = same origin).
- **Bearer auth, not cookies**: the access token lives in memory ([token-store.ts](frontend/src/lib/api/token-store.ts)), the rotating refresh token in an httpOnly cookie set by the API. On boot, `useAuth.boot()` calls `/v1/auth/refresh` to restore the session. Bearer-authed binary endpoints (avatars, CSV export) can't be loaded via a plain `<img src>`/link - fetch the bytes through the client and use a blob URL (see [useAvatarUrl.ts](frontend/src/composables/useAvatarUrl.ts) and `exportCsv`).
- **Money formatting**: always `Intl.NumberFormat(undefined, { style: "currency", currency: <iso>, currencyDisplay: "narrowSymbol" })` via [currencies.ts](frontend/src/lib/currencies.ts) (`moneyFormatter`/`formatMoney`) with the group's `default_currency` (or the expense's own `currency`). Never hardcode `$`.
- **Currency dropdowns**: use [CurrencySelect.vue](frontend/src/components/CurrencySelect.vue) - defaults to `EUR`, common short list first, full Intl list second.
- **Plain CSS and Tailwind**: shared design tokens (OKLCH), the three themes (light/dark/high-contrast via `:root[data-theme=…]`), and the `.field-*`/`.btn-*`/`.toggle`/`.avatar-*` systems live in [global.css](frontend/src/styles/global.css) under `@layer`. The theme is applied pre-paint by [frontend/public/theme-boot.js](frontend/public/theme-boot.js) (a same-origin classic script, since the strict CSP forbids inline scripts).
- **Styling order of preference (Tailwind → global.css → scoped `<style>`)**: reach for each tier only when the one above genuinely can't express it.
  1. **Tailwind utility classes in the template** are the default for all one-off layout/spacing/color. Use first-class utilities backed by our semantic tokens (`text-foreground`, `bg-card`, `border-border`, `text-destructive`, `text-subtle-foreground`, `bg-hover-surface`, `bg-backdrop`, `outline-ring`, …) - every bespoke token is registered in the `@theme` block of [global.css](frontend/src/styles/global.css), so it resolves as a named utility. **Never write `text-[var(--token)]` for a token that has a `@theme` entry**; if a semantic token isn't a utility yet, add it to `@theme` first, then use the clean class. Arbitrary values (`text-[11px]`, `w-[calc(100%-2rem)]`, `pl-[max(1rem,env(safe-area-inset-left))]`, `bg-[color-mix(...)]`, `grid-cols-[20.5rem_minmax(0,1fr)_20.5rem]`) are fine **only** for true one-offs that aren't reusable design values - px font sizes, `calc()`/`env()`/`color-mix()`, grid templates.
  2. **A class in [global.css](frontend/src/styles/global.css)** when a pattern repeats across SFCs or is a design-system primitive (the `.field-*`/`.btn-*`/`.toggle`/`.avatar-*` families, category tints). Author it under the matching `@layer` (`tokens` for vars, `components` for class recipes) so the cascade stays predictable. A recurring value belongs here as a token, not copy-pasted as an arbitrary utility.
  3. **A scoped `<style>` block in the SFC** is the last resort, reserved for genuinely component-local styling that neither a utility nor a shared class can express cleanly (complex `:deep()` into a child, keyframes, a one-component layout quirk). Don't reach for it just to avoid a long `class=` string. As of the Tailwind migration, no view/component carries a scoped `<style>` block - keep it that way unless there's a real need.
- **Single-column layout, capped at 768px (default)**: `<AppLayout>` ([frontend/src/components/AppLayout.vue](frontend/src/components/AppLayout.vue)) caps `<main>` at 48rem. Pages stack vertically at every viewport; design mobile-first and verify at 768px. A `sm:`-style inline flip inside a row item is fine; page-level multi-column is not.
- **Opt-in wide layout for triptych pages**: pass `wide` to `<AppLayout>` to cap at 72rem / 1152px. Reserved for the group dashboard ([GroupDashboardView.vue](frontend/src/views/GroupDashboardView.vue)) where Balances / Transactions / Add-expense form a triptych: `grid-template-columns: 20.5rem minmax(0,1fr) 20.5rem` at ≥1024px, reordered Balances | Transactions | Add-expense, stacking single-column below.
- **Validation feedback**: rely on native HTML constraint validation (`required`, `pattern`, `type="email"`, `minlength`) plus `:user-invalid` styling. Use [Field.vue](frontend/src/components/Field.vue), which renders the floating-label + sibling `.field-error` (the only visible cue on Firefox for Android). Don't call `event.preventDefault()` for validation; for cross-field rules (password match) use `setCustomValidity` on the exposed input ref. Submit with `@submit.prevent` and call the composable.
- **Native form controls**: keep them. We polish the closed/inert state via `.field-*` classes but never replace `<select>`, `<input type="checkbox|radio|number">` with custom JS widgets (accessibility, IME, offline, install size). Exception: `<input type="date">` is replaced by [DatePicker.vue](frontend/src/components/DatePicker.vue) because the native popup sizes inconsistently and we need a today-overlay glyph + cadence dropdown.
- **Icons**: inline SVG via [Icon.vue](frontend/src/components/Icon.vue), which renders Font Awesome 7 path data from the generated [frontend/src/lib/icons.ts](frontend/src/lib/icons.ts). Inline SVG markup is CSP-clean (only inline `<script>`/`<style>` are blocked); to add a glyph, extend the generator's name list and re-run it.

## PWA / offline (read-only)

The SPA is an installable PWA with **read-only offline**, via `vite-plugin-pwa` ([frontend/vite.config.ts](frontend/vite.config.ts), `generateSW`/Workbox).

- **Strategy**: precache the shell + hashed assets; runtime-cache `/v1` **GETs** network-first (cache `dts-v1-get`, `networkTimeoutSeconds: 3`, cache fallback when offline/stalled). NetworkFirst (not StaleWhileRevalidate) so a read issued right after a mutation - e.g. the dashboard `reload()` after creating an expense - reflects the write instead of returning a stale cached page while revalidating in the background. Mutations are GET-only-excluded by Workbox and pass straight to the network; `/v1/auth/*` is excluded so tokens are never cached, and the SSE stream `/v1/groups/{id}/events` is excluded so the SW never tees its never-ending body into the cache (which buffers frames and stalls real-time delivery after a few minutes). `registerType: 'autoUpdate'` - a new SW applies silently on the next navigation.
- **CSP-clean registration**: `injectRegister: null` plus a bundled `registerSW({ immediate: true })` import in [main.ts](frontend/src/main.ts). Never let the plugin inject its default inline registration snippet - the strict CSP (`script-src 'self'`) blocks inline scripts. The `virtual:pwa-register` type comes from `vite-plugin-pwa/client` referenced in [env.d.ts](frontend/env.d.ts).
- **Offline UX**: [useNetworkStatus.ts](frontend/src/composables/useNetworkStatus.ts) (module-singleton reactive, like `useTheme`) drives an offline banner in [AppLayout.vue](frontend/src/components/AppLayout.vue). Offline mutations are short-circuited in [client.ts](frontend/src/lib/api/client.ts) - `onRequest` returns a synthetic `503` for non-GET, non-auth requests while `navigator.onLine` is false, so every composable surfaces a clear message instead of an opaque failure.
- **Serving**: the Go binary serves `sw.js`, `workbox-*.js`, `manifest.webmanifest`, and the `pwa-*.png` icons from the dist root like any other static file. `.webmanifest` has no entry in Go's mime table, so [webui.go](server/internal/handlers/webui.go) sets `Content-Type: application/manifest+json` explicitly; [spa_test.go](server/internal/server/spa_test.go) asserts the SW/manifest/icon are served with the right content-types. Icons are rendered from `logo.svg` (the maskable one padded to its safe zone on the dark `theme_color`).

## Refresh cookie (important)

The only cookie the API sets is the rotating refresh token, `dts_refresh`: httpOnly, `SameSite=Lax`, scoped to `/v1/auth` (so only refresh/revoke ever receive it), with `Secure` set when `COOKIE_SECURE=true`. The access token is a stateless JWT held in memory by the client and sent as `Authorization: Bearer <token>` - never a cookie. The SPA restores its session on boot by calling `/v1/auth/refresh` against this cookie.

## Account invariants

- **Soft delete, never hard delete.** Accounts have `deleted_at`; the foreign keys from expenses, splits, settlements, and recurring templates deliberately stay pointing at the tombstoned row so ledgers survive. If a requirement ever seems to want hard delete + CASCADE, stop and flag it - that's silent data loss for every other group member.
- **Tombstone format** is `"Deleted user #" + uuid[:8]`. It's stable (members can still identify _which_ deleted person paid for what) and non-identifying (no email, no real name). The full UUID is also the only non-scrambled column after delete, so operators can still answer "who was this?" from the audit trail.
- **Re-registration** with a soft-deleted email works because `users_email_hash_active_key` is a partial unique index (`WHERE deleted_at IS NULL`).
- **Token revocation on delete + password change**: both flows must call `AuthService.RevokeRefreshForUser` so every refresh-token chain is revoked. Password change (and email-change confirm) additionally mint a fresh token pair and rotate the `dts_refresh` cookie so the current browser stays logged in. Note access tokens are stateless JWTs: a still-valid access token keeps working until it expires, except for deleted users (`ResolveAccessToken` re-checks `deleted_at` on every request).

## Avatar invariants

- Avatars are **uploaded as an 8×8 PNG, ≤ 1024 bytes** (64 color samples). Client-side pipeline in [frontend/src/lib/avatar-pixelate.ts](frontend/src/lib/avatar-pixelate.ts) center-crops any source image to square, downsamples with `imageSmoothingEnabled = false` (nearest-neighbour), and pushes saturation to 1.0 before base64-encoding a PNG. Its color math is pinned by [avatar-pixelate.test.ts](frontend/src/lib/avatar-pixelate.test.ts) - keep that suite green. The server **re-encodes from a fresh RGBA canvas and nearest-neighbour upscales to `AvatarRenderSize`** (currently 256×256 = 8 × 32) before storing in `users.avatar BYTEA`. The pre-scaled bitmap renders crisp at any CSS size without `image-rendering: pixelated` hints, which have inconsistent browser support.
- GDPR-safe by construction: 64 pixels can't identify a human. Never add a "keep original" option without legal sign-off.
- Fallback when `has_avatar=false` is handled by [Avatar.vue](frontend/src/components/Avatar.vue) - initials from the display name. The bearer-authed PNG is fetched into a blob URL by [MemberAvatar.vue](frontend/src/components/MemberAvatar.vue) + [useAvatarUrl.ts](frontend/src/composables/useAvatarUrl.ts); use `MemberAvatar` for any member avatar so the token is carried correctly.

## Security invariants (don't regress)

- Passwords: Argon2id only, `golang.org/x/crypto/argon2`. Never accept reversibly-encrypted passwords.
- Emails: `email_hash = HMAC-SHA256(normalize(email), EMAIL_HMAC_KEY)` for lookups; `email_encrypted = key_id ‖ nonce ‖ AES-GCM(EMAIL_ENC_KEY, …)` for display. Keys are 32-byte base64 from env; fail fast if missing.
- `/auth/token`, `/auth/register`, and the other credential-bearing `/auth/*` endpoints (verify, password-reset) are rate-limited; keep them on the `authG` group in the router.
- Security headers middleware emits HSTS only when `COOKIE_SECURE=true`.
- Never log `email`, `password`, or session tokens. The access logger ([server/internal/middleware/logger.go](server/internal/middleware/logger.go)) logs only method, path, status, duration, client IP, and request id - never request bodies. Don't add body/field logging there; if you ever do, redact sensitive fields first.

## Testing

Three layers, all run in CI on every PR:

- **Go unit tests** colocate with packages (`*_test.go`). Pure logic only - split math, balance simplification, Argon2 round-trip, config loading.
- **Go integration tests** run against a real database. The full [server/internal/server/](server/internal/server/) suite is **engine-parameterized** by `TEST_DB_DRIVER` (`postgres` default via `testcontainers-go/postgres`, or `sqlite` in-process/in-memory) - the `setup()` harness builds the matching `repo.Store`, so every HTTP-level test (golden path, admin authz, group authz matrix, strict-JSON matrix, recurring worker tick, SSE stream, avatar pipeline, bearer flow) runs on both engines. Schema-only invariants live in [server/internal/repo/migrations_test.go](server/internal/repo/migrations_test.go) (Postgres) and [server/internal/repo/sqlite/migrations_sqlite_test.go](server/internal/repo/sqlite/migrations_sqlite_test.go) (SQLite): up/down round-trip + group-delete FK cascades. Tests that need dialect-specific raw SQL go through the `testRawStore` helpers ([repo/{postgres,sqlite}/testsupport.go](server/internal/repo/)), never inline SQL.
- **SPA unit tests** via [vitest](https://vitest.dev) under [frontend/src/\*\*/\*.test.ts](frontend/src/). Pure helpers only (jsdom, no canvas) - the avatar-pixelate suite pins the GDPR-load-bearing color math; currency/short-name suites pin formatting.
- **End-to-end** via [Playwright](https://playwright.dev) under [frontend/tests/e2e/](frontend/tests/e2e/). Boots the actual `docker compose` stack, scrapes the install token from `docker compose logs api`, and drives `/setup` + group create through the Vue SPA on the single api origin (`:8080`). Catches contract drift between the SPA and the Go API.

Invariants for adding tests:

- **When adding endpoints**, extend the integration suite with at least one positive case AND one authz-negative case. The strict-JSON matrix test ([server/internal/server/strict_json_test.go](server/internal/server/strict_json_test.go)) and the group authz matrix ([server/internal/server/group_authz_test.go](server/internal/server/group_authz_test.go)) are parameterized - add your new endpoint there too.
- **Don't mock the DB.** We want real SQL behavior, including FK cascades, partial unique indexes, and `FOR UPDATE` semantics.
- **Don't mock the mailer outbox** in tests that assert a user receives a code - the outbox is part of the contract.
- **HTTP client in tests** uses the per-package `testHTTPClient` ([server_test.go:36](server/internal/server/server_test.go#L36)) with `DisableKeepAlives: true` and a 90s timeout. Don't reach for `http.DefaultClient` - pooled stale connections to torn-down `httptest` servers cause 19-minute hangs under `-race` on CI.

Run everything with `make test`. Go alone: `make test-go` (Postgres). Both engines: `make test-go-both` (or `make test-go-sqlite` / `make test-go-postgres` individually) - CI runs the Go suite once per engine. SPA unit alone: `make test-frontend`. E2E alone: boot a stack (`docker compose up -d --build` for SQLite, or `docker compose -f docker-compose.postgres.yml up -d --build` for Postgres), scrape the token from the api logs, then `SETUP_TOKEN=... make test-e2e`. CI runs the e2e suite against both engines.

Linting also gates CI on every PR: `make lint` runs golangci-lint (Go, pinned in the `lint-go` target) and eslint (SPA). Run it before pushing.

## Running the app

- `docker compose up -d --build` - default single-container SQLite deployment (`docker-compose.yml`: one `dothesplit` container + a DB-file volume; no postgres, no migrate one-shot, no separate worker - the tick runs in-process). The server binary serves both `/v1` and the embedded SPA on `http://localhost:8080`.
- `docker compose -f docker-compose.postgres.yml up -d --build` - full Postgres stack (postgres, migrate, dothesplit; requires `POSTGRES_PASSWORD` + `DATABASE_URL`). The tick runs in-process in the `dothesplit` container.
- `make up` - Postgres stack, but stamps `BUILD_COMMIT` (git short SHA) and `BUILD_VERSION` (from `frontend/package.json`) into the image so `/healthz` and the page footer self-identify.
- Local non-Docker dev: `make dev-api` (Go server on `:8080`) + `make dev-frontend` (Vite dev server on `:4321`, proxying `/v1` to `:8080`).
- `make build` builds the SPA, copies it into the embed dir, then builds the Go binary. After any change that affects the API contract: `make gen`, then rebuild the `dothesplit` image (a frontend-only change still means rebuilding it, since the SPA is embedded).
- Production: pull pinned images from GHCR (`ghcr.io/julian-alarcon/dothesplit:X.Y.Z` - one image). Don't build from `main` on the deployment host: releases are published by CI and tagged via release-please from conventional-commit titles. The `:dev` tag tracks `main` for staging.

## Scope boundaries (don't build these without asking)

Deferred from v1 - raise with the user before adding:

- OAuth / passkeys, multi-currency FX conversion, file receipts / expense attachments, full-resolution avatars (8x8 GDPR-minimisation is deliberate), account hard-delete (soft delete preserves co-members' ledgers).
