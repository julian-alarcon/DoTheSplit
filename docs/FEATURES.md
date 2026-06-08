# Features

Detailed reference for what currently ships in DoTheSplit. The
[README](../README.md) keeps a one-liner per area; this file is the long form.

## Accounts

Register, log in, log out, change display name, change password (old password
required). Set a personal **timezone** override (otherwise resolved from a
device-detected `dts_tz` cookie, falls back to UTC). Upload an **8×8 pixel
avatar** generated in-browser from any image: pixelated PNG ≤ 1024 bytes,
re-encoded server-side; falls back to deterministic initials when absent.
Soft-delete your account with a stable `Deleted user #<short-uuid>` tombstone so
shared history stays traceable; the email index is partial-unique on
`deleted_at IS NULL`, so the address is reusable after deletion. Self-delete
requires the current password as a confirmation step in the same form, and all
existing sessions for the user are revoked atomically.

## First-run setup

The first boot prints a single-use **setup token** in the API container log
(`docker compose logs api`). Until that token is consumed via `/setup`, every
UI route redirects there and `/v1/auth/register` returns
`403 setup_required`. The token is 32 bytes of `crypto/rand` entropy, stored
only as SHA-256, rotated on every boot, and consumed atomically with the first
admin's account creation in a single transaction (advisory lock + `SELECT FOR
UPDATE`). Replays return `410 Gone`. After completion the page is closed:
authenticated visitors are redirected to `/groups`, the rest to `/login`.

## Admin role

A separate `is_admin` flag on `users`, granted to the bootstrap admin and any
user another admin promotes. The `/admin` area exposes:

- **Users**: list, search, toggle admin, reset password (scrambles the
  password hash and emails the user a 6-digit code so they pick a new one),
  soft-delete, optional `?include_deleted=1` toggle.
- **Groups**: oversight view of every group with member count and creator.
- **SMTP**: configure outbound mail; the password column is encrypted at rest
  with AES-GCM using `EMAIL_ENC_KEY`.
- **Audit**: paginated log of every admin action with actor, IP, UA, target.

Destructive actions (delete user, reset password, toggle admin) require a
**step-up password prompt** in the same browser session, even if the admin is
already logged in. SMTP is exempt (configuration, not a destructive action).

## Groups

Create, rename, set a per-group **default currency** (defaults to EUR; full
list: EUR, USD, GBP, CHF, CAD, AUD, JPY, SEK, NOK, DKK, plus a few others).
Invite existing members by email, leave a group, remove a member (creator
only), **transfer ownership** to another member, delete (creator only;
cascades to expenses, splits, settlements, recurring templates). Settings live
on a dedicated `/groups/{id}/settings` page. For 2-member groups, pin a
**default percentage split** (e.g. 60/40) that prefills new expenses;
auto-cleared when a 3rd member joins. Group creation supports adding initial
members in the same form.

## Expenses

Three split modes via a shared in-app editor:

- **equal**: round-robin remainder distribution
- **exact**: per-member cents with live remainder validation
- **percent**: per-member percentage with live total validation

Expenses carry a category (one of ten seeded categories, rendered with Font
Awesome icons), an optional custom date (defaults to today), a description,
and optional free-text **notes** for context the description shouldn't carry.
Any group member can edit description / amount / category / payer / splits /
notes / date after the fact; splits either rescale proportionally on
amount-only edits or are re-resolved when a new mode/split is supplied.
Soft-delete is open to any group member. The full edit history shows who /
when / field / old → new, including per-member split diffs.

## Balances & settle-up

Net-balance computation over all expenses + settlements, plus a simplified "X
owes Y" view. Settlements are recorded directly from the group page, appear in
the same paginated transaction feed as expenses, and have their own detail page.
The settle-up form lets the actor pick **who is paying** (defaults to
themselves), so a member can record a settlement on someone else's behalf
without impersonating. Any group member can later edit a settlement
(`PATCH /v1/settlements/{id}`): from / to / amount / note / settled-at are
all mutable as long as both parties are still group members and differ from
each other.

## Recurring expenses

Templates with daily / weekly / biweekly / monthly / yearly cadence. A
separate Go worker materializes a real expense on each cadence tick. Both the
API and the UI (`/groups/{id}/recurring`) are shipped.

## Transaction feed

Paginated, time-ordered feed of expenses + settlements per group. Months are
labelled, ordering matches the underlying timestamps regardless of insertion
order, and pagination state is URL-encoded so deep links work.

## Search

Cross-group search at `/search` over a case-insensitive substring of `q`
(min 2 chars) against expense **description** + **notes** and settlement
**notes**. Results are scoped to groups the actor is a member of; foreign
group ids in the query string are silently dropped, soft-deleted rows are
excluded, and ILIKE wildcards (`%`, `_`) in the user's query are escaped so
they match literally. Two optional filters narrow the set:

- **Group** (single-select): restricts hits to one of the actor's groups.
- **Category** (single-select): restricts hits to expenses in that category
  and excludes settlements entirely (settlements have no category).

The response carries an `available_category_ids` list — the distinct
categories present in the unfiltered result set for the current `q` + group
scope — so the category picker only offers categories that actually have
matches. The list is computed independently of the active category filter so
the user can still switch off it. Both filters live in a collapsible panel
below the search button and auto-open when any filter is active.

## Import & export

CSV-based import and export, anchored on the Splitwise format so users can
move ledgers in and out of the app without lock-in.

- **Splitwise import** at `/import/splitwise` parses an N-person Splitwise
  CSV export, shows a dry-run preview (member balances, first few expenses,
  settlement preview, mixed-currency warning, skipped rows), then commits as
  a brand-new group. Rows where multiple people paid become multiple
  expenses with a `[k/K]` description suffix; `Payment` rows become
  settlements. Unknown member emails get non-loginable placeholder accounts
  so the foreign keys stay valid (the real owner can claim later).
- **Group CSV export** lives in group settings (`Export` block, any
  member). The download is a Splitwise-shaped CSV plus dothesplit-only
  metadata columns (`Time`, `Payer`, `Notes`, `Created`, `CreatedBy`)
  inserted between `Currency` and the per-member block. The file is
  re-importable through both the legacy Splitwise importer (which silently
  skips the extra columns) and the dothesplit importer (which reads them).
- **DoTheSplit import** at `/import/dothesplit` is the inverse of the
  exporter: same dry-run/commit flow as the Splitwise importer, but the
  parser honors the extra columns - second-precision `incurred_at` from
  `Date + Time`, explicit `Payer` (overrides sign-based payer inference),
  and `Notes` round-trip into `expense.notes`. `Created` / `CreatedBy` are
  surfaced in the preview as provenance but not stored: the new group has
  fresh audit columns with the importing user as `created_by`.
- **Identity in the file**: column headers carry display names, never
  emails. Both importers ask the user to map each CSV name to an email at
  commit time, exactly the same flow.

## Settings & about

The personal area is at `/settings` (display name, password, timezone,
avatar, account deletion). The third-party attribution and license summary
lives at `/about`, linked from the header user menu. The header itself
exposes a collapsible user menu so navigation, theme switcher, and search
share one row on small screens.

## Security

- Argon2id passwords with a server-side pepper.
- Email stored as HMAC (lookup) + AES-GCM (display) with 32-byte keys held in
  env, never in the DB.
- Rate-limited `/v1/auth/*` and `/v1/setup/admin` (5/min/IP).
- Strict JSON bodies: unknown fields are a 400.
- CSP headers with SHA-256 hashes on inline scripts; no inline event handlers
  (e.g. `onchange`): auto-submit forms use a `data-auto-submit` attribute and
  a shared module.
- HSTS only when `COOKIE_SECURE=true`. Session cookie is `__Host-dts_session`
  on HTTPS, plain `dts_session` on the HTTP LAN profile.
- Step-up password prompt for destructive admin actions, and password
  confirmation before self-delete (with all sessions revoked on success).

## API contract

OpenAPI 3.0.3 at [openapi.yaml](openapi.yaml) is the source of truth. Every
business endpoint lives under `/v1/...`; health probes (`/healthz`, `/readyz`)
are the only unversioned routes. Go server types and a Go integration-test
client are generated via `oapi-codegen`; TypeScript types via
`openapi-typescript`. `make gen` regenerates both.
