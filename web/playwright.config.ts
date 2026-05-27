import { defineConfig, devices } from "@playwright/test";

/**
 * E2E config: tests assume a fully running stack at WEB_BASE_URL (defaults to
 * http://localhost:3000). In CI a docker compose step boots api + postgres +
 * web before invoking `npm run test:e2e`. Locally, run
 * `docker compose up -d --build` in the repo root, then
 * `cd web && npm run test:e2e`.
 *
 * We intentionally don't use Playwright's `webServer` field: the SSR Astro
 * bridge calls into the Go API, so spinning up `astro dev` alone wouldn't
 * give us a working stack.
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
    baseURL: process.env.WEB_BASE_URL || "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
});
