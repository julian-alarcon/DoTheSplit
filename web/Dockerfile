# Build context is the repo root so we can pull in /docs/openapi.yaml.
# NODE_VERSION must stay in sync with web/.nvmrc; ALPINE_VERSION is the base image variant.
ARG NODE_VERSION=24.16.0
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

# Production-only dependency tree for the runtime image. The build stage above
# needs the full dev tree (astro build, openapi-typescript); the SSR server does
# not. `--omit=dev` drops TypeScript, the openapi/redocly toolchain, Playwright,
# tailwind/lightningcss, etc. The follow-up `find` strips docs, source maps, and
# non-runtime TypeScript sources the server never loads. We do NOT strip dirs
# named test/tests: some packages ship loadable runtime code there (e.g.
# @iconify/utils' lib/emoji/test/parse.mjs), so blanket removal breaks SSR. We
# also avoid node-prune: it installs via `wget | sh` of an unpinned binary,
# which contradicts the `--ignore-scripts` / no-curl posture of this image.
FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS prod-deps
WORKDIR /app
COPY web/package.json web/package-lock.json* ./
RUN npm ci --omit=dev --ignore-scripts
RUN find node_modules -name "*.md" -delete \
 && find node_modules -name "*.txt" -delete \
 && find node_modules -name "CHANGELOG*" -delete \
 && find node_modules -name "*.ts" ! -name "*.d.ts" -delete \
 && find node_modules -name "*.map" -delete

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
COPY --from=prod-deps --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/package.json ./package.json
USER node
EXPOSE 3000
CMD ["node", "./dist/server/entry.mjs"]
