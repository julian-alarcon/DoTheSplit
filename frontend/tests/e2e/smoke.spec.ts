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

  // This serial suite shares one stack, and Playwright doesn't reset the DB
  // between retries, so a retry of this test runs against an instance where the
  // admin already exists. In that state /setup redirects to /login and the
  // token field is gone. Detect both resting states and stay meaningful on
  // retry: either mint the admin via /setup, or (already minted) log in. Both
  // assert the admin exists and the app reaches /groups.
  await page.goto("/setup");
  const tokenField = page.getByLabel("Setup token");
  const loginButton = page.getByRole("button", { name: /log in/i });
  await expect(tokenField.or(loginButton)).toBeVisible();

  if (await tokenField.isVisible()) {
    await tokenField.fill(TOKEN!);
    await page.getByLabel("Display name").fill(ADMIN_NAME);
    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await page.getByRole("button", { name: /create admin/i }).click();
  } else {
    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await loginButton.click();
  }

  // Successful setup auto-logs-in via the token flow and lands on /groups.
  await expect(page).toHaveURL(/\/groups\/?$/);
});

test("admin can create a new group", async ({ page }) => {
  test.skip(!TOKEN, "SETUP_TOKEN env var required");

  // Each Playwright test runs in a fresh context with no cookies, so the SPA's
  // boot refresh always 401s and redirects to /login. (On a real reload with a
  // live refresh cookie it would land on /groups instead.) `goto` resolves on
  // the /groups document load, BEFORE that client-side redirect settles, so we
  // must wait for the SPA to come to rest rather than reading page.url()
  // eagerly. Race the two possible resting states: the authenticated landing
  // (the "New group" link) or the login form.
  await page.goto("/groups");
  const newGroupLink = page.getByRole("link", { name: /new group/i }).first();
  const loginButton = page.getByRole("button", { name: /log in/i });
  await expect(newGroupLink.or(loginButton)).toBeVisible();
  if (await loginButton.isVisible()) {
    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await loginButton.click();
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
