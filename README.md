# DoTheSplit

Open-source expense-sharing app. See [BLUEPRINT.md](BLUEPRINT.md) for the product
definition.

## Layout

- `/api`: Go backend (Gin, pgx, oapi-codegen)
- `/web`: Astro 6 + Tailwind v4 frontend (SSR via `@astrojs/node`)
- `/docs/openapi.yaml`: API contract (source of truth)
- `/docker-compose.yml`: local & LAN deployment

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full build / test / deploy
guide.

## Quick start

```bash
cp .env.example .env
# Generate keys
echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)" >> .env
echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)" >> .env
echo "PASSWORD_PEPPER=$(openssl rand -base64 32)" >> .env

docker compose up -d
```

Open http://localhost:3000.

## Development

```bash
make gen            # regenerate Go + TS API bindings from openapi.yaml
make migrate-up     # apply DB migrations
make dev            # run api + web against a local postgres
make test           # unit + integration tests
```

## Features

Currently shipped and usable:

- **Accounts**: register, log in, log out, change display name, change password (old password required), upload an **8×8 pixel avatar** (generated in-browser from any image; falls back to initials), soft-delete your account with a stable tombstone so shared history stays traceable.
- **Groups**: create, rename, set a per-group default currency (defaults to EUR), invite existing members by email, delete (creator only; cascades).
- **Expenses**: create with four split modes (equal / exact / percent / shares), categorize with one of ten seeded categories , edit description / amount / category / payer after the fact (payer or group creator; splits rescale proportionally), soft-delete, and inspect the full edit history with who / when / field / old → new.
- **Balances & settle-up**: net-balance computation over all expenses + settlements, plus a simplified "X owes Y" view. Record settlements directly.
- **Recurring expenses**: template + background worker that materializes a real expense on each cadence tick (daily / weekly / monthly). Backend API is complete; frontend UI is pending (see Roadmap).
- **Security**: Argon2id passwords; email stored as HMAC (lookup) + AES-GCM (display) with keys held outside the DB; rate-limited `/v1/auth/*`; strict JSON bodies reject unknown fields; CSP headers with SHA-256 hashes on inline scripts.
- **API**: OpenAPI 3.0.3 contract at [docs/openapi.yaml](docs/openapi.yaml) is the source of truth; every endpoint lives under `/v1/...` (health probes are the only unversioned routes).

## Roadmap

Reasonable next steps, roughly prioritized. Contributions welcome: open an issue first so we can scope.

### Near term

- **Group-level default split** (follow-up, not a blocker): in group settings, allow a 2-member group to pin a default non-equal split (e.g. 40/60) that prefills the create-expense form. Store as `(user_id, basis_points)` pairs so the mapping is stable. Invalidate / hide the default automatically when the group grows past 2 members.
- Frontend UI for **recurring expenses** (the backend + worker are already there).
- Themes
  - Use Inter font (in local to avoid calling thirt party entities)
- **Settlements UI**: list past settlements per group; today you can only record them.
- **Pagination** on expense and settlement lists. Load first 50 expenses and a a Button at the end to load 25 more.
- Publishing to gitHub and Github docker registry
- Deploy in TrueNAS with custom docker-compose

### Medium term

- **Native mobile** via the PWA path (the Astro side is already SSR-first and mobile-first styled).
- **i18n** (app is English-only today; amount and date formatting already respect the browser locale).
- **Optimistic UI + refresh-on-focus** via `@tanstack/react-query` (the perf budget is ≤100ms perceived: we're close on SSR but mutations still block).
- **Search & filter** expenses by category, member, date range.
- **Import** from CSV
- **Export** a group's ledger to CSV.
- **Expense attachments / receipts** (photo or PDF).

### Longer term / ideas

- **Password reset** via email (needs SMTP wiring).
- **OAuth / passkeys** alongside passwords.
- **Real-time sync** (push updates via SSE or WebSockets instead of the current polling / refresh-on-focus model).
- **TLS terminated by Caddy in-compose** as a first-class option, replacing the current "terminate outside the stack" note below.
- **Multi-currency FX**: today each group picks one default currency; cross-currency groups would need conversion rates and a locked-at-time-of-entry policy.

Explicitly not planned: file hosting of full-resolution avatars (the 8×8 format is a deliberate GDPR-minimizing choice), account hard-delete (soft delete preserves other members' ledgers).

## Deployment note: HTTPS deviation

[BLUEPRINT.md](BLUEPRINT.md) states **"HTTPS only"**. The v1 LAN profile ships
**HTTP-only** for TrueNAS LAN use: session cookies use `Secure=false`. For
internet-exposed deployments, terminate TLS at an upstream reverse proxy (Caddy,
Traefik, Cloudflare Tunnel) and flip `COOKIE_SECURE=true`.

## License

MIT.
