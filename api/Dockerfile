# Build context is the repo ROOT (see docker-compose.yml), because the API
# binary now embeds the Vue SPA built from frontend/ and typed against docs/.

# --- Stage 1: build the Vue SPA static bundle -------------------------------
ARG NODE_VERSION=24.16.0
ARG ALPINE_VERSION=3.22
FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS web
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --ignore-scripts
COPY frontend ./
COPY docs/openapi.yaml /openapi/openapi.yaml
# Build identity must reach Vite (process.env.BUILD_*) so the SPA bakes the real
# version into the About page. Declared here (after npm ci) so a version bump
# doesn't bust the dependency-install cache layer.
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=dev
ENV BUILD_VERSION=$BUILD_VERSION
ENV BUILD_COMMIT=$BUILD_COMMIT
RUN npx openapi-typescript /openapi/openapi.yaml -o src/lib/api/schema.d.ts \
 && npm run build

# --- Stage 2: build the Go binaries with the SPA embedded -------------------
FROM golang:1.26.4-alpine3.22 AS build
WORKDIR /src
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=dev
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
# Drop the SPA bundle into the embed dir so //go:embed picks up the real
# assets instead of the committed .gitkeep placeholder.
COPY --from=web /app/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${BUILD_VERSION} -X main.commit=${BUILD_COMMIT}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${BUILD_VERSION} -X main.commit=${BUILD_COMMIT}" \
      -o /out/worker ./cmd/worker
# Pre-create the SQLite data dir owned by the distroless nonroot uid so a fresh
# named volume mounted at /data inherits that ownership (Docker seeds an empty
# volume from the image path). The distroless runtime has no shell to mkdir it.
RUN mkdir -p /data

# --- Stage 3: distroless runtime --------------------------------------------
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY api/migrations /migrations
# nonroot in distroless is uid:gid 65532; owning /data lets the SQLite
# deployment write dts.db + WAL there. Postgres deployments ignore it.
COPY --from=build --chown=65532:65532 /data /data
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
