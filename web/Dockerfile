# Build context is the repo root so we can pull in /docs/openapi.yaml.
# NODE_VERSION must stay in sync with web/.nvmrc; ALPINE_VERSION is the base image variant.
ARG NODE_VERSION=24.15.0
ARG ALPINE_VERSION=3.22

FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS deps
WORKDIR /app
COPY web/package.json web/package-lock.json* ./
RUN npm install --ignore-scripts

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
# The build-time commit SHA is surfaced to the SSR runtime via process.env so
# the footer in every page can show "build abc1234". Defaults to "dev" when no
# git context is available (e.g. building from a tarball or outside CI).
ARG BUILD_COMMIT=dev
ENV BUILD_COMMIT=${BUILD_COMMIT}
COPY --from=build /app/dist ./dist
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/package.json ./package.json
EXPOSE 3000
CMD ["node", "./dist/server/entry.mjs"]
