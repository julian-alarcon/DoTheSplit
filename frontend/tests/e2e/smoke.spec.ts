import { expect, test } from "@playwright/test";

/**
 * Smoke E2E: drives the install ceremony + group create through the Vue SPA,
 * hitting the real Go API on the same origin. The point is to catch contract
 * drift between the SPA and the API end-to-end - a green Go suite + green Vue
 * build can both pass while the client sends fields the API rejects (or vice
 * versa). One end-to-end test catches that class of bug.
 *
 * Stack assumptions:
 *   - APP_BASE_URL points at the running api container, which serves the
 *     embedded SPA (defaults to localhost:8080).
 *   - SETUP_TOKEN env var carries the install token. The CI job greps it from
 *     `docker compose logs api` after boot. Locally, copy it from the API
 *     container's startup log.
 *
 * The Field component wraps each input in a <label> whose floating caption is
 * the accessible name, so getByLabel resolves them. Buttons are matched by
 * their accessible role+name.
 */

const TOKEN = process.env.SETUP_TOKEN;
const ADMIN_EMAIL = "admin-e2e@test.dev";
const ADMIN_NAME = "Admin E2E";
const PASSWORD = "passwordpassword"; // 16 chars, satisfies the 10-char min.

test.describe.configure({ mode: "serial" });

test("first-run setup mints an admin", async ({ page }) => {
  test.skip(!TOKEN, "SETUP_TOKEN env var is required for E2E (see docker compose logs api)");

  await page.goto("/setup");
  await expect(page.getByLabel("Setup token")).toBeVisible();

  await page.getByLabel("Setup token").fill(TOKEN!);
  await page.getByLabel("Display name").fill(ADMIN_NAME);
  await page.getByLabel("Email").fill(ADMIN_EMAIL);
  await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
  await page.getByRole("button", { name: /create admin/i }).click();

  // Successful setup auto-logs-in via the token flow and lands on /groups.
  await expect(page).toHaveURL(/\/groups\/?$/);
});

test("admin can create a new group", async ({ page }) => {
  test.skip(!TOKEN, "SETUP_TOKEN env var required");

  // The SPA restores the session from the httpOnly refresh cookie on reload;
  // if that has lapsed, log back in through the token flow.
  await page.goto("/groups");
  await page.waitForURL(/\/(groups|login)\/?$/);
  if (page.url().includes("/login")) {
    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await page.getByRole("button", { name: /log in/i }).click();
    await expect(page).toHaveURL(/\/groups\/?$/);
  }

  await page.getByRole("link", { name: /new group/i }).first().click();
  await expect(page).toHaveURL(/\/groups\/new$/);

  await page.getByLabel("Group name").fill("Smoke Trip");
  await page.getByRole("button", { name: /create group/i }).click();

  // Group dashboard URL is /groups/<uuid>; assert we landed there and the
  // name we just typed is rendered.
  await expect(page).toHaveURL(/\/groups\/[0-9a-f-]+/i);
  await expect(page.getByRole("heading", { name: "Smoke Trip" })).toBeVisible();
});
