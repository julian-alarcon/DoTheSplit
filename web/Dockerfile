# Build context is the repo root so we can pull in /docs/openapi.yaml.
# NODE_VERSION must stay in sync with web/.nvmrc; ALPINE_VERSION is the base image variant.
ARG NODE_VERSION=24.15.0
ARG ALPINE_VERSION=3.22

FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS deps
WORKDIR /app
COPY web/package.json web/package-lock.json* ./
# `--ignore-scripts` blocks postinstall hooks from transitive deps.
RUN npm ci --ignore-scripts

FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY web ./
COPY docs/openapi.yaml /openapi/openapi.yaml
RUN npx openapi-typescript /openapi/openapi.yaml -o src/lib/api/schema.d.ts
RUN npm run build

FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION}
WORKDIR /app
ENV NODE_ENV=production
# Build-time metadata surfaced to the SSR runtime via process.env so the
# footer on every page can show the version + commit. Defaults to "dev" when
# no git/release context is available (e.g. building from a tarball locally).
ARG BUILD_COMMIT=dev
ARG BUILD_VERSION=dev
ENV BUILD_COMMIT=${BUILD_COMMIT}
ENV BUILD_VERSION=${BUILD_VERSION}
COPY --from=build --chown=node:node /app/dist ./dist
COPY --from=build --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/package.json ./package.json
USER node
EXPOSE 3000
CMD ["node", "./dist/server/entry.mjs"]
