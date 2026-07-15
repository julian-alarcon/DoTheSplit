# Install

## Choose a database engine first

DoTheSplit runs on **SQLite or PostgreSQL**, selected by the `DATABASE_DRIVER` env var. This var is **required and has no default** - the app refuses to start if it is unset, so the engine is always an explicit choice. The compose files below each set it for you.

- **`DATABASE_DRIVER=sqlite`**: a single `dothesplit` container plus one DB-file volume. No separate Postgres or migrate containers. Migrations are embedded in the binary and applied in-process on first boot; the recurring-expense worker runs in-process. This is the simplest path and the right choice for a single-node install. Compose file: `docker-compose.yml` (the default).
- **`DATABASE_DRIVER=postgres`**: a separate database server (`postgres`, `migrate`, `dothesplit` services), which also requires `DATABASE_URL`. Compose file: `docker-compose.postgres.yml`.

The engine choice does **not** change encryption at rest: emails, passwords, SMTP credentials, and token hashes are encrypted/hashed at the application layer above the DB on both engines (see [Secrets](#secrets)). Neither engine encrypts the whole database file. The four crypto keys are required for both.

Install paths below:

- **[Generic Docker host](#generic-docker-host)**: any Linux box (or VM) with Docker and Docker Compose v2. Works for self-hosters who already run a compose stack, for evaluation on a workstation, or as the basis for adapting to other orchestrators (Portainer, Komodo, plain `systemd` units, etc.). Covers both the SQLite and Postgres stacks.
- **[TrueNAS SCALE Custom App](#truenas-scale-custom-app)**: full SQLite walkthrough through the Apps wizard, with host-path bind mounts so snapshots and replication can target the dataset (Postgres differences noted at the end).

Both paths ship HTTP-only on the LAN by default. For internet-exposed deployments see [HTTPS / internet exposure](#https--internet-exposure).

---

## Generic Docker host

For local development you usually want the quick start in [README.md](README.md#sqlite-default) instead, since it builds from the working tree. The flow below installs from the **pinned release images on GHCR**, which is the right choice for any deployment you're not actively hacking on.

### Requirements

- Docker Engine 24+ and Docker Compose v2.
- `openssl` for key generation.
- ~200 MB of disk for images plus whatever your data grows to.

### SQLite (single container)

The simplest install: one `dothesplit` container and one DB-file volume. No Postgres or migrate containers. Migrations apply in-process on first boot and the recurring-expense worker runs in-process.

**Step 1: Generate the secrets.** SQLite needs only the four crypto keys - no `POSTGRES_PASSWORD`, no `DATABASE_URL` (it defaults to `file:./dts.db`):

```sh
mkdir -p ~/dothesplit && cd ~/dothesplit
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
} >> .env
```

Save the secrets in a password manager **now** - see [Secrets](#secrets).

**Step 2: Write `docker-compose.yml`.** Save this next to `.env`. It runs a single service, storing `dts.db` (plus its `-wal`/`-shm` sidecars) on a named volume mounted at `/data`. Substitute your release tag for `v1.0.0`:

```yaml
services:
  dothesplit:
    image: ghcr.io/julian-alarcon/dothesplit:1.0.0
    environment:
      # sqlite is the default, but set it explicitly for clarity.
      DATABASE_DRIVER: sqlite
      # Land the DB file on the mounted volume rather than the (read-only) workdir.
      DATABASE_URL: file:/data/dts.db
      API_HTTP_ADDR: ":8080"
      WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:8080}
      COOKIE_SECURE: ${COOKIE_SECURE:-false}
      EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
      EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
      PASSWORD_PEPPER: ${PASSWORD_PEPPER}
      JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
      LOG_LEVEL: ${LOG_LEVEL:-info}
    volumes:
      - dts_sqlite_data:/data
    ports:
      # The server binary serves both the JSON API and the embedded Vue SPA here.
      - "127.0.0.1:8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/dothesplit", "--healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=32m

volumes:
  dts_sqlite_data:
```

The app writes `dts.db` and its WAL/shm sidecars to `/data`; the container connects with WAL journaling, `busy_timeout`, `foreign_keys=ON`, and `synchronous=NORMAL` pragmas (pure-Go `modernc.org/sqlite` driver, no CGO). The host port binding is loopback-only; change `127.0.0.1:` to `0.0.0.0:` to expose it to the LAN.

**Step 3: Bring it up.**

```sh
docker compose up -d
docker compose ps   # dothesplit healthy after ~10s
```

Then continue at [First-run setup](#first-run-setup) below.

**Backups (SQLite):** stop the app (`docker compose stop`) before snapshotting the `dts_sqlite_data` volume so the WAL is checkpointed, and back up `.env` separately - a DB file without the keys cannot be decrypted.

**Updates (SQLite):** no migrations directory to refresh (they're embedded). Bump the image tag, then `docker compose pull && docker compose up -d`. Embedded migrations apply in-process on the next boot.

---

### First-run setup

The first time the app boots, it prints a one-time setup token to its logs and refuses to create users until the token is consumed:

```sh
docker compose logs dothesplit | grep -A2 'first-run setup'
```

The log line includes `token=<value>` and `url=…/setup`. Open `http://<host>:8080/setup`, paste the token, fill in display name + email + password (≥10 chars). On success you are redirected to `/groups` and the setup form is permanently locked, even across restarts.

### Verify

```sh
curl -fsS http://localhost:8080/healthz   # 200 with version + commit
curl -fsS http://localhost:8080/readyz    # 200 once DB is reachable
```

The web footer also shows the running version, linked to the GitHub Release.

---

### PostgreSQL (what changes vs SQLite)

Reach for Postgres only when you need multi-instance / scale-out; a single-node install should stay on SQLite. Everything above (secrets, first-run setup, verify, HTTPS) is identical. The differences:

- **Extra services.** The stack grows from one container to three: `postgres` (the database), `migrate` (a one-shot that applies `server/migrations/*.up.sql`, then exits), and `dothesplit`. The recurring-expense worker still runs in-process inside `dothesplit`, so there is no separate worker service.
- **Two more env vars.** Set `DATABASE_DRIVER=postgres` (sqlite is the default, so Postgres must opt in) and add `POSTGRES_PASSWORD`. `DATABASE_URL` is now required (no `file:` default): `postgres://dts:<POSTGRES_PASSWORD>@postgres:5432/dts?sslmode=disable`. URL-encode the password if it contains any of `: / ? # [ ] @`.
- **Migrations live on disk.** Unlike SQLite (migrations embedded in the binary, applied in-process), the Postgres path bind-mounts `server/migrations` into the `migrate` container. Fetch them per release tag.

**Step 1: Get migrations + secrets.** Substitute your release tag for `v1.0.0`:

```sh
mkdir -p ~/dothesplit && cd ~/dothesplit
curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v1.0.0.tar.gz \
  | tar -xz --strip-components=1 \
      'dothesplit-1.0.0/server/migrations' \
      'dothesplit-1.0.0/.env.example'
cp .env.example .env
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
  echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)"
} >> .env
```

Edit `.env` and set `DATABASE_URL` so its password matches `POSTGRES_PASSWORD`. Save all five secrets in a password manager now: see [Secrets](#secrets).

**Step 2: Write `docker-compose.postgres.yml`** next to `.env`. It pulls pinned GHCR release images instead of building from source. Substitute your release tag for `v1.0.0`:

```yaml
services:
  postgres:
    image: postgres:18.4-alpine3.22
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-dts}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set in .env}
      POSTGRES_DB: ${POSTGRES_DB:-dts}
    volumes:
      # Mount the parent dir, not …/data: PG 18 stores data in a
      # major-version-specific subdir so pg_upgrade --link works in place.
      - dts_pg_data:/var/lib/postgresql
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "pg_isready -U ${POSTGRES_USER:-dts} -d ${POSTGRES_DB:-dts}",
        ]
      interval: 5s
      timeout: 3s
      retries: 10
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETUID, SETGID]
    security_opt: ["no-new-privileges:true"]
    mem_limit: 512m
    pids_limit: 200

  migrate:
    image: migrate/migrate:v4.19.1
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./server/migrations:/migrations:ro
    environment:
      DATABASE_URL: ${DATABASE_URL}
    entrypoint: ["/bin/sh", "-c"]
    command:
      - 'exec migrate -path /migrations -database "$$DATABASE_URL" up'
    restart: "no"
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=8m

  dothesplit:
    image: ghcr.io/julian-alarcon/dothesplit:1.0.0
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
    environment:
      DATABASE_DRIVER: postgres
      DATABASE_URL: ${DATABASE_URL}
      API_HTTP_ADDR: ":8080"
      WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:8080}
      COOKIE_SECURE: ${COOKIE_SECURE:-false}
      EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
      EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
      PASSWORD_PEPPER: ${PASSWORD_PEPPER}
      JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
      LOG_LEVEL: ${LOG_LEVEL:-info}
    ports:
      # The server binary serves both the JSON API and the embedded Vue SPA here.
      - "127.0.0.1:8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/dothesplit", "--healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=32m

volumes:
  dts_pg_data:
```

**Step 3: Bring it up**, then follow [First-run setup](#first-run-setup) and [Verify](#verify) above (swap `docker compose` for `docker compose -f docker-compose.postgres.yml`):

```sh
docker compose -f docker-compose.postgres.yml up -d
docker compose -f docker-compose.postgres.yml ps   # all three services healthy after ~20s
```

**Updates (Postgres):** refresh the migrations to the new tag, bump image tags, then pull + up. The `migrate` one-shot re-runs on every up and is idempotent:

```sh
curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v0.8.0.tar.gz \
  | tar -xz --strip-components=1 'dothesplit-0.8.0/server/migrations'
docker compose -f docker-compose.postgres.yml pull
docker compose -f docker-compose.postgres.yml up -d
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md#major-postgres-upgrades) for major Postgres version bumps.

**Backups (Postgres):** stop the stack (`docker compose -f docker-compose.postgres.yml stop postgres`) before snapshotting the `dts_pg_data` volume, or run `pg_dump` against the running container. Back up `.env` separately: a dump without the keys cannot be decrypted.

---

## Secrets

The four crypto keys - `EMAIL_ENC_KEY`, `EMAIL_HMAC_KEY`, `PASSWORD_PEPPER`, `JWT_SIGNING_KEY` - are required by **both** engines. They drive application-level encryption at rest (emails AES-256-GCM + HMAC-SHA256 lookup hash; passwords Argon2id + pepper; SMTP password, verification-code hashes, and refresh-token hashes stored as hashed/encrypted BLOBs), which is identical on SQLite and Postgres. Neither engine encrypts the whole database file, so protecting these keys is what protects the data. Losing `EMAIL_ENC_KEY`, `EMAIL_HMAC_KEY`, or `PASSWORD_PEPPER` after the database has data in it makes that data unrecoverable; rotating `JWT_SIGNING_KEY` only forces token clients to log in again. See [Secrets you must back up](README.md#secrets-you-must-back-up) for the rationale. Back them up alongside whichever data volume you use (`dts_pg_data` or `dts_sqlite_data`).

---

## TrueNAS SCALE Custom App

This path uses host-path bind mounts under `/mnt/<pool>/apps-data/dothesplit/` so TrueNAS Periodic Snapshots and Replication Tasks can target the dataset directly. It walks through the **SQLite** stack (one `dothesplit` container), which is the right choice for a single-node TrueNAS install. To run Postgres instead, see [Postgres on TrueNAS](#postgres-on-truenas) at the end.

### Prerequisites

- TrueNAS SCALE 24.10 or newer with the **Apps** service started.
- A pool/dataset for app data. The rest of this guide assumes `ssd-storage/apps-data/dothesplit`; substitute your own pool name throughout.
- Shell access (System Settings → Shell, or SSH).
- A machine with `openssl` available (TrueNAS shell works).

### Step 1: Pre-create the dataset and host directory

The TrueNAS Custom App wizard cannot create directories on save, so the install will fail if the bind-mount source is missing. Create the dataset `ssd-storage/apps-data/dothesplit`, then the host directory that holds the DB file:

```sh
mkdir -p /mnt/ssd-storage/apps-data/dothesplit/data
```

The distroless `dothesplit` image runs as UID 65532 (`nonroot`). Make the `data` directory writable by that user so SQLite can create `dts.db` and its WAL sidecars:

```sh
chown -R 65532:65532 /mnt/ssd-storage/apps-data/dothesplit/data
```

### Step 2: Generate the secrets

SQLite needs only the four crypto keys - no `POSTGRES_PASSWORD`, no external `DATABASE_URL`. Run on any machine with `openssl` and write the output down somewhere safe **before** continuing:

```sh
echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
```

Losing `EMAIL_ENC_KEY`, `EMAIL_HMAC_KEY`, or `PASSWORD_PEPPER` after the database has data in it makes that data unrecoverable. See [Secrets you must back up](README.md#secrets-you-must-back-up) in the README for the full rationale.

### Step 3: Install the Custom App

1. **Apps → Discover Apps → Custom App**.
2. **Application Name**: `dothesplit`.
3. **Install via custom YAML**: paste the compose below. It is the project's [`docker-compose.yml`](docker-compose.yml) with the named volume replaced by a host-path bind mount at the `data` dataset you created in Step 1. The `dothesplit` service uses the pinned GHCR release image (one image serves the JSON API, the embedded SPA, and the in-process worker) instead of building from source. Substitute your release tag for `v1.0.0`:

   ```yaml
   services:
     dothesplit:
       image: ghcr.io/julian-alarcon/dothesplit:1.0.0
       environment:
         # sqlite is the default, but set it explicitly for clarity.
         DATABASE_DRIVER: sqlite
         # Land the DB file on the bind-mounted dataset, not the read-only workdir.
         DATABASE_URL: file:/data/dts.db
         API_HTTP_ADDR: ":8080"
         WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:8080}
         COOKIE_SECURE: ${COOKIE_SECURE:-false}
         EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
         EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
         PASSWORD_PEPPER: ${PASSWORD_PEPPER}
         JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
         LOG_LEVEL: info
       volumes:
         - /mnt/ssd-storage/apps-data/dothesplit/data:/data
       ports:
         # The server binary serves both the JSON API and the embedded Vue SPA.
         - "8080:8080"
       restart: unless-stopped
       healthcheck:
         test: ["CMD", "/dothesplit", "--healthcheck"]
         interval: 10s
         timeout: 3s
         retries: 5
         start_period: 10s
       read_only: true
       cap_drop: [ALL]
       security_opt: ["no-new-privileges:true"]
       tmpfs:
         - /tmp:rw,noexec,nosuid,size=32m
   ```

   The published port mapping drops the upstream `127.0.0.1:` host prefix because TrueNAS expects the app to be reachable on the LAN. If you put the app behind Traefik or another reverse proxy on the same host, prefer to attach it to the proxy's Docker network and stop publishing 8080 to the host at all.

4. **Environment Variables**: add the four secrets to the Custom App's environment table:

   | Name              | Value          |
   | ----------------- | -------------- |
   | `EMAIL_ENC_KEY`   | (from Step 2)  |
   | `EMAIL_HMAC_KEY`  | (from Step 2)  |
   | `PASSWORD_PEPPER` | (from Step 2)  |
   | `JWT_SIGNING_KEY` | (from Step 2)  |

5. **Networking**: leave on the default bridge. Port 8080 (which also serves the SPA) is exposed on the TrueNAS host. For internet exposure, see the section below.

6. **Storage**: nothing to add in the Storage step; the host path is already wired in the YAML.

7. Click **Install** and watch the **Containers** tab until the `dothesplit` service is healthy (~10s).

### Step 4: Consume the first-run setup token

The first time the container boots, it prints a one-time setup token to its logs and refuses to create users until that token is consumed. From the TrueNAS shell:

```sh
docker logs ix-dothesplit-dothesplit-1 2>&1 | grep -A2 'first-run setup'
```

(Container name pattern is `ix-<app>-<service>-1`. If TrueNAS picks a different name, find it via `docker ps | grep dothesplit`.) The log line includes `token=<value>` and `url=…/setup`.

Open `http://<truenas-ip>:8080/setup`, paste the token, fill in display name + email + password (≥10 chars), and submit. On success you are redirected to `/groups` and the setup form is permanently locked: even after restarts, only an explicit DB edit can re-open it.

### Step 5: Verify

```sh
curl -fsS http://<truenas-ip>:8080/healthz   # 200 with version + commit
curl -fsS http://<truenas-ip>:8080/readyz    # 200 once DB is reachable
```

The web footer also shows the running version, linked to the GitHub Release.

### Backups

- Snapshot the dataset `ssd-storage/apps-data/dothesplit` recursively (Periodic Snapshot Tasks). The SQLite `dts.db` file (plus its `-wal`/`-shm` sidecars) lives in the bind-mounted `data` dataset. Stop the app before snapshotting so the WAL is checkpointed.
- Keep the four secrets out of band, in a password manager or a separate secrets vault. A snapshot without the keys cannot be decrypted (the encryption is application-level, above the DB).

### Updates

**Apps → dothesplit → Edit** → bump the `image:` tag to the new `:X.Y.Z` and click **Save**. Embedded migrations apply in-process on the next boot; there is no migrations directory to refresh.

### Postgres on TrueNAS

To run the Postgres stack instead, the [PostgreSQL (what changes vs SQLite)](#postgresql-what-changes-vs-sqlite) differences apply, plus these TrueNAS specifics:

- **Two extra directories.** Alongside (or instead of) `data`, create `pgdata` (the database volume) and `migrations` (the `migrate` bind mount):

  ```sh
  mkdir -p /mnt/ssd-storage/apps-data/dothesplit/{pgdata,migrations}
  chown -R 70:70 /mnt/ssd-storage/apps-data/dothesplit/pgdata
  chmod 700      /mnt/ssd-storage/apps-data/dothesplit/pgdata
  ```

  UID **70** is the `postgres` user in the `postgres:18.4-alpine3.22` image (Alpine convention; Debian-based images use 999). **Do not** apply the dataset's `Apps` permission preset (UID 568) to `pgdata`, or Postgres refuses to start on a directory it can't own. The `migrations` directory needs no special ownership (`migrate` reads it `:ro`).

- **Drop the migrations on disk** before installing, and refresh them on every upgrade:

  ```sh
  cd /mnt/ssd-storage/apps-data/dothesplit/migrations
  curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v1.0.0.tar.gz \
    | tar -xz --strip-components=3 --wildcards '*/server/migrations'
  ```

- **Use the three-service YAML** from [PostgreSQL (what changes vs SQLite)](#postgresql-what-changes-vs-sqlite), swapping the named `dts_pg_data` volume for the `pgdata` bind mount and the `./server/migrations` mount for `/mnt/.../migrations`. Add `POSTGRES_PASSWORD` and `DATABASE_URL` (`postgres://dts:<POSTGRES_PASSWORD>@postgres:5432/dts?sslmode=disable`) to the environment table. Keep the Postgres mount target at `/var/lib/postgresql` (parent dir, not `…/data`).

- **Backups:** the Postgres data dir lives inside the recursively-snapshotted dataset. Updates also require refreshing the `migrations` directory before bumping the image tag.

---

## HTTPS / internet exposure

The app ships HTTP-only by default; see [Deployment note: HTTPS deviation](README.md#deployment-note-https-deviation) in the README. To put it on the internet:

1. Terminate TLS at an upstream reverse proxy (Caddy, Traefik, nginx, Cloudflare Tunnel: anything that speaks HTTP/1.1 upstream).
2. Set `COOKIE_SECURE=true` and `WEB_ORIGIN=https://split.yourdomain.tld` (in `.env` for the generic path, or in the Custom App env table on TrueNAS). `WEB_ORIGIN` must include the port if the proxy listens on a non-standard one (e.g. `https://split.yourdomain.tld:35000`).
3. Set `TRUSTED_PROXIES` to the proxy's IP or CIDR (e.g. `192.168.1.200/32`). The API otherwise ignores `X-Forwarded-For` and attributes every request to the proxy's address, which breaks per-client rate limiting and audit logs. Leave it empty when no proxy is in front, so a client can't forge `X-Forwarded-For` to dodge the limiter.
4. Stop publishing port 8080 on the host and instead attach the app to the proxy's Docker network.
5. Restart the stack.

When `COOKIE_SECURE=true` the `dts_refresh` cookie gains the `Secure` flag (sent only over HTTPS) and the API emits HSTS; the backend handles this automatically.

## Rootless posture

For reference, the containers run as follows:

| Service      | User                                         | Filesystem  | Notes                                                                                                                                                           |
| ------------ | -------------------------------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `dothesplit` | `nonroot:nonroot` (UID 65532)                | `read_only` | Distroless image (serves the API, the embedded SPA, and the in-process worker), all caps dropped                                                                |
| `postgres`   | starts as root, drops to `postgres` (UID 70) | writable    | Standard upstream behaviour; needs root for `initdb`, runs as 70 thereafter, which is why the TrueNAS path chowns `pgdata` to 70 rather than the `apps` UID 568 |

On TrueNAS specifically: the data directory belongs to the database engine, not to the shared `apps` user pool, so deviating from the `Apps` permission preset for `pgdata` is intentional.
