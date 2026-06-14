import { defineConfig, devices } from "@playwright/test";

/**
 * E2E config: tests assume a fully running stack at APP_BASE_URL (defaults to
 * http://localhost:8080 - the Go API binary now serves the embedded Vue SPA on
 * the same origin it serves /v1 from, so there's a single origin to point at).
 * In CI a docker compose step boots api + postgres before invoking
 * `npm run test:e2e`. Locally, run `docker compose up -d --build` in the repo
 * root, then `cd app && npm run test:e2e`.
 *
 * We intentionally don't use Playwright's `webServer` field: the SPA is a
 * static bundle embedded in the Go binary, so there's no separate dev server
 * to manage - the whole stack lives in docker compose.
 */
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false, // first-run setup is single-shot per stack; serialize.
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? "line" : "list",
  timeout: 30_000,
  use: {
    baseURL: process.env.APP_BASE_URL || "http://localhost:8080",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
