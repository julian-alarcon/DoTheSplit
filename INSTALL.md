# Install

Two install paths:

- **[Generic Docker host](#generic-docker-host)**: any Linux box (or VM) with Docker and Docker Compose v2. Works for self-hosters who already run a compose stack, for evaluation on a workstation, or as the basis for adapting to other orchestrators (Portainer, Komodo, plain `systemd` units, etc.).
- **[TrueNAS SCALE Custom App](#truenas-scale-custom-app)**: full walkthrough through the Apps wizard, with host-path bind mounts so snapshots and replication can target the dataset.

Both paths ship HTTP-only on the LAN by default. For internet-exposed deployments see [HTTPS / internet exposure](#https--internet-exposure).

---

## Generic Docker host

For local development you usually want the Quick start in [README.md](README.md#quick-start) instead, since it builds from the working tree. The flow below installs from the **pinned release images on GHCR**, which is the right choice for any deployment you're not actively hacking on.

### Requirements

- Docker Engine 24+ and Docker Compose v2.
- `openssl` for key generation.
- ~200 MB of disk for images plus whatever your data grows to.

### Step 1: Get the release artifacts

Pick a release tag from <https://github.com/julian-alarcon/dothesplit/releases> and substitute it for `v1.0.0` below.

```sh
mkdir -p ~/dothesplit && cd ~/dothesplit
curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v1.0.0.tar.gz \
  | tar -xz --strip-components=1 \
      'dothesplit-1.0.0/api/migrations' \
      'dothesplit-1.0.0/.env.example'
```

You should now have `api/migrations/*.sql` and `.env.example` in `~/dothesplit`.

### Step 2: Generate the secrets

```sh
cp .env.example .env
{
  echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
  echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
  echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
  echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
  echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)"
} >> .env
```

Edit `.env` and update `DATABASE_URL` so the password matches `POSTGRES_PASSWORD`. URL-encode the password if it contains any of `: / ? # [ ] @`.

Save the secrets in a password manager **now**. Losing `EMAIL_ENC_KEY`, `EMAIL_HMAC_KEY`, or `PASSWORD_PEPPER` after the database has data in it makes that data unrecoverable; rotating `JWT_SIGNING_KEY` only forces token clients to log in again. See [Secrets you must back up](README.md#secrets-you-must-back-up) for the rationale.

### Step 3: Write `docker-compose.yml`

Save this as `docker-compose.yml` next to `.env`. It mirrors the project's compose file but pulls pinned release images from GHCR instead of building from source. Substitute your release tag for `v1.0.0`:

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
      - ./api/migrations:/migrations:ro
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

  api:
    image: ghcr.io/julian-alarcon/dothesplit:1.0.0
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
    environment:
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
      # The api binary serves both the JSON API and the embedded Vue SPA here.
      - "127.0.0.1:8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/api", "--healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=32m

  worker:
    image: ghcr.io/julian-alarcon/dothesplit:1.0.0
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
    entrypoint: ["/worker"]
    environment:
      DATABASE_URL: ${DATABASE_URL}
      EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
      EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
      PASSWORD_PEPPER: ${PASSWORD_PEPPER}
      JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
      LOG_LEVEL: ${LOG_LEVEL:-info}
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=32m

volumes:
  dts_pg_data:
```

The Postgres mount target is `/var/lib/postgresql` (the parent dir, not `…/data`). PG 18 stores data in a major-version-specific subdir so future `pg_upgrade --link` works in place; mounting at `…/data` makes the container fail to start.

The host port bindings are loopback-only (`127.0.0.1:`); change to `0.0.0.0:` (or just drop the prefix) to expose them to the LAN, or attach the services to a reverse-proxy network and stop publishing them to the host at all.

### Step 4: Bring it up

```sh
docker compose up -d
docker compose ps        # all services should be healthy after ~20s
```

### Step 5: Consume the first-run setup token

```sh
docker compose logs api | grep -A2 'first-run setup'
```

The log line includes `token=<value>` and `url=…/setup`. Open `http://<host>:8080/setup`, paste the token, fill in display name + email + password (≥10 chars). On success you are redirected to `/groups` and the setup form is permanently locked.

### Step 6: Verify

```sh
curl -fsS http://localhost:8080/healthz   # 200 with version + commit
curl -fsS http://localhost:8080/readyz    # 200 once DB is reachable
```

### Updates

```sh
# Refresh migrations to the new tag (replace v0.8.0)
curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v0.8.0.tar.gz \
  | tar -xz --strip-components=1 'dothesplit-0.8.0/api/migrations'

# Bump image tags in docker-compose.yml, then:
docker compose pull
docker compose up -d
```

The `migrate` one-shot runs again on every up; it is idempotent. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md#major-postgres-upgrades) for the special case of major Postgres version bumps.

### Backups

Stop the stack (`docker compose stop postgres`) before snapshotting the named volume, or run `pg_dump` against the running container. Back up `.env` separately: a database dump without the keys cannot be decrypted.

---

## TrueNAS SCALE Custom App

This path uses host-path bind mounts under `/mnt/<pool>/apps-data/dothesplit/` so TrueNAS Periodic Snapshots and Replication Tasks can target the dataset directly.

### Prerequisites

- TrueNAS SCALE 24.10 or newer with the **Apps** service started.
- A pool/dataset for app data. The rest of this guide assumes `ssd-storage/apps-data/dothesplit`; substitute your own pool name throughout.
- Shell access (System Settings → Shell, or SSH).
- A machine with `openssl` available (TrueNAS shell works).

### Step 1: Pre-create the dataset and host directories

The TrueNAS Custom App wizard cannot create directories on save, so the install will fail if the bind-mount source is missing. Create everything up front from the shell:

Datasets:

- `ssd-storage/apps-data/dothesplit`

Then create the host directories:

```sh
mkdir -p /mnt/ssd-storage/apps-data/dothesplit/pgdata
mkdir -p /mnt/ssd-storage/apps-data/dothesplit/migrations
chown -R 70:70 /mnt/ssd-storage/apps-data/dothesplit/pgdata
chmod 700      /mnt/ssd-storage/apps-data/dothesplit/pgdata
```

About UID **70**: the `postgres:18.4-alpine3.22` image ships an internal `postgres` user with UID/GID 70 (the Alpine convention; the Debian-based `postgres` images use 999 instead), and that's the user the engine drops to after `initdb`. **Do not** apply the dataset's `Apps` permission preset (UID 568) to `pgdata`, or Postgres will refuse to start on a directory it can't own.

The `migrations` directory does not need special ownership; the `migrate` container reads it `:ro`.

### Step 2: Drop the migrations on disk

The `migrate` one-shot container reads SQL files from a host path. Fetch the migrations matching the release you intend to install (replace `v1.0.0` with the tag you want):

```sh
cd /mnt/ssd-storage/apps-data/dothesplit/migrations
curl -fsSL https://github.com/julian-alarcon/dothesplit/archive/refs/tags/v1.0.0.tar.gz \
  | tar -xz --strip-components=3 --wildcards '*/api/migrations'
ls   # should list 0001_*.up.sql, 0001_*.down.sql, …
```

When you upgrade the app later, refresh this directory to the new tag before bumping the image versions; the migrate container applies whatever `*.up.sql` files it finds.

### Step 3: Generate the secrets

Run on any machine with `openssl` and write the output down somewhere safe **before** continuing:

```sh
echo "EMAIL_ENC_KEY=$(openssl rand -base64 32)"
echo "EMAIL_HMAC_KEY=$(openssl rand -base64 32)"
echo "PASSWORD_PEPPER=$(openssl rand -base64 32)"
echo "JWT_SIGNING_KEY=$(openssl rand -base64 32)"
echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)"
```

Losing `EMAIL_ENC_KEY`, `EMAIL_HMAC_KEY`, or `PASSWORD_PEPPER` after the database has data in it makes that data unrecoverable. See [Secrets you must back up](README.md#secrets-you-must-back-up) in the README for the full rationale.

Construct the `DATABASE_URL` from the Postgres password you just generated:

```
postgres://dts:<POSTGRES_PASSWORD>@postgres:5432/dts?sslmode=disable
```

URL-encode the password if it contains any of `: / ? # [ ] @`.

### Step 4: Install the Custom App

1. **Apps → Discover Apps → Custom App**.
2. **Application Name**: `dothesplit`.
3. **Install via custom YAML**: paste the compose below. It is the project's [`docker-compose.yml`](docker-compose.yml) with two TrueNAS-specific changes: the named Postgres volume is replaced by a host-path bind mount, and the `migrate` bind mount points at the host path you populated in Step 2. The `api` and `worker` services use the pinned GHCR release image (one image serves both the JSON API and the embedded SPA) instead of building from source. Substitute your release tag for `v1.0.0`:

   ```yaml
   services:
     postgres:
       image: postgres:18.4-alpine3.22
       restart: unless-stopped
       environment:
         POSTGRES_USER: dts
         POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
         POSTGRES_DB: dts
       volumes:
         - /mnt/ssd-storage/apps-data/dothesplit/pgdata:/var/lib/postgresql
       healthcheck:
         test: ["CMD-SHELL", "pg_isready -U dts -d dts"]
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
         - /mnt/ssd-storage/apps-data/dothesplit/migrations:/migrations:ro
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

     api:
       image: ghcr.io/julian-alarcon/dothesplit:1.0.0
       depends_on:
         postgres:
           condition: service_healthy
         migrate:
           condition: service_completed_successfully
       environment:
         DATABASE_URL: ${DATABASE_URL}
         API_HTTP_ADDR: ":8080"
         WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:8080}
         COOKIE_SECURE: ${COOKIE_SECURE:-false}
         EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
         EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
         PASSWORD_PEPPER: ${PASSWORD_PEPPER}
         JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
         LOG_LEVEL: info
       ports:
         # The api binary serves both the JSON API and the embedded Vue SPA.
         - "8080:8080"
       restart: unless-stopped
       healthcheck:
         test: ["CMD", "/api", "--healthcheck"]
         interval: 10s
         timeout: 3s
         retries: 5
         start_period: 10s
       read_only: true
       cap_drop: [ALL]
       security_opt: ["no-new-privileges:true"]
       tmpfs:
         - /tmp:rw,noexec,nosuid,size=32m

     worker:
       image: ghcr.io/julian-alarcon/dothesplit:1.0.0
       depends_on:
         postgres:
           condition: service_healthy
         migrate:
           condition: service_completed_successfully
       entrypoint: ["/worker"]
       environment:
         DATABASE_URL: ${DATABASE_URL}
         EMAIL_ENC_KEY: ${EMAIL_ENC_KEY}
         EMAIL_HMAC_KEY: ${EMAIL_HMAC_KEY}
         PASSWORD_PEPPER: ${PASSWORD_PEPPER}
         JWT_SIGNING_KEY: ${JWT_SIGNING_KEY}
         LOG_LEVEL: info
       restart: unless-stopped
       read_only: true
       cap_drop: [ALL]
       security_opt: ["no-new-privileges:true"]
       tmpfs:
         - /tmp:rw,noexec,nosuid,size=32m
   ```

   Two details worth not changing:
   - The Postgres mount target stays `/var/lib/postgresql` (parent dir, not `…/data`). PG 18 stores data in a major-version-specific subdir so future `pg_upgrade --link` works in place; mounting at `…/data` makes the container fail to start.
   - The published port mapping drops the upstream `127.0.0.1:` host prefix because TrueNAS expects the app to be reachable on the LAN. If you put the app behind Traefik or another reverse proxy on the same host, prefer to attach it to the proxy's Docker network and stop publishing 8080 to the host at all.

4. **Environment Variables**: add the four secrets and the connection string to the Custom App's environment table:

   | Name                | Value                                                                  |
   | ------------------- | ---------------------------------------------------------------------- |
   | `POSTGRES_PASSWORD` | (from Step 3)                                                          |
   | `DATABASE_URL`      | `postgres://dts:<POSTGRES_PASSWORD>@postgres:5432/dts?sslmode=disable` |
   | `EMAIL_ENC_KEY`     | (from Step 3)                                                          |
   | `EMAIL_HMAC_KEY`    | (from Step 3)                                                          |
   | `PASSWORD_PEPPER`   | (from Step 3)                                                          |
   | `JWT_SIGNING_KEY`   | (from Step 3)                                                          |

5. **Networking**: leave on the default bridge. Port 8080 (api, which also serves the SPA) is exposed on the TrueNAS host. For internet exposure, see the section below.

6. **Storage**: nothing to add in the Storage step; the host paths are already wired in the YAML.

7. Click **Install** and watch the **Containers** tab until all four services (postgres, migrate, api, worker) are healthy.

### Step 5: Consume the first-run setup token

The first time the API container boots, it prints a one-time setup token to its logs and refuses to create users until that token is consumed. From the TrueNAS shell:

```sh
docker logs ix-dothesplit-api-1 2>&1 | grep -A2 'first-run setup'
```

(Container name pattern is `ix-<app>-<service>-1`. If TrueNAS picks a different name, find it via `docker ps | grep dothesplit-api`.) The log line includes `token=<value>` and `url=…/setup`.

Open `http://<truenas-ip>:8080/setup`, paste the token, fill in display name + email + password (≥10 chars), and submit. On success you are redirected to `/groups` and the setup form is permanently locked: even after restarts, only an explicit DB edit can re-open it.

### Step 6: Verify

```sh
curl -fsS http://<truenas-ip>:8080/healthz   # 200 with version + commit
curl -fsS http://<truenas-ip>:8080/readyz    # 200 once DB is reachable
```

The web footer also shows the running version, linked to the GitHub Release.

### Backups

- Snapshot the dataset `ssd-storage/apps-data/dothesplit` recursively (Periodic Snapshot Tasks). The Postgres data dir lives inside it.
- Keep the four secrets out of band, in a password manager or a separate secrets vault. A snapshot without the keys cannot be decrypted.

### Updates

1. Refresh the migrations directory to the new release (re-run the `curl | tar` command from Step 2 with the new tag).
2. **Apps → dothesplit → Edit** → bump the `image:` tags for `api` and `worker` (they share one image) to the new `:X.Y.Z` and click **Save**.

The `migrate` one-shot runs again on every up; it is idempotent and applies any new `*.up.sql` files. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md#major-postgres-upgrades) for the special case of major Postgres version bumps.

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

| Service    | User                                         | Filesystem  | Notes                                                                                                                                                           |
| ---------- | -------------------------------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api`      | `nonroot:nonroot` (UID 65532)                | `read_only` | Distroless image (also serves the embedded SPA), all caps dropped                                                                                               |
| `worker`   | `nonroot:nonroot` (UID 65532)                | `read_only` | Same image as `api`                                                                                                                                             |
| `postgres` | starts as root, drops to `postgres` (UID 70) | writable    | Standard upstream behaviour; needs root for `initdb`, runs as 70 thereafter, which is why the TrueNAS path chowns `pgdata` to 70 rather than the `apps` UID 568 |

On TrueNAS specifically: the data directory belongs to the database engine, not to the shared `apps` user pool, so deviating from the `Apps` permission preset for `pgdata` is intentional.
