import { reactive, readonly } from "vue";
import { api, baseUrl } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { clearAccessToken, setAccessToken } from "@/lib/api/token-store";

type User = components["schemas"]["User"];

// Module-singleton reactive auth state. This is the one genuinely shared
// cross-route piece of state in the app, so it lives in a plain reactive
// module rather than pulling in Pinia (per the migration simplicity rules).
const state = reactive({
  user: null as User | null,
  // ready flips true once the initial boot refresh + /me probe has settled,
  // so route guards can wait instead of bouncing to /login on a hard reload.
  ready: false,
});

async function fetchMe(): Promise<void> {
  const { data, error } = await api.GET("/v1/me");
  state.user = error ? null : data;
}

/**
 * Login via the token flow. On success the refresh token is set as the
 * httpOnly dts_refresh cookie by the API; we keep only the access token in
 * memory.
 */
async function login(email: string, password: string): Promise<{ ok: boolean; code?: string }> {
  const { data, error, response } = await api.POST("/v1/auth/token", {
    body: { email, password },
  });
  if (error || !data) {
    const code = response.status === 403 ? "email_unverified" : "invalid_credentials";
    return { ok: false, code };
  }
  setAccessToken(data.access_token, data.expires_in);
  await fetchMe();
  return { ok: true };
}

/**
 * Complete first-run setup, then establish a bearer session. The setup
 * endpoint sets a cookie session, which the SPA doesn't use, so we follow up
 * with the token flow using the same credentials.
 */
async function completeSetup(input: {
  token: string;
  email: string;
  password: string;
  display_name: string;
}): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/setup/admin", { body: input });
  if (error) {
    if (response.status === 401) return { ok: false, code: "invalid" };
    if (response.status === 409) return { ok: false, code: "email_taken" };
    if (response.status === 410) return { ok: false, code: "completed" };
    if (response.status === 429) return { ok: false, code: "rate_limited" };
    return { ok: false, code: "bad_request" };
  }
  return login(input.email, input.password);
}

/**
 * Register an account. When verification is required the caller should route
 * to /verify; otherwise we auto-login via the token flow (we hold the
 * password). `verificationRequired` is only meaningful when ok is true.
 */
async function register(input: {
  email: string;
  password: string;
  display_name: string;
}): Promise<{ ok: boolean; verificationRequired?: boolean; code?: string }> {
  const { data, error, response } = await api.POST("/v1/auth/register", {
    body: input,
  });
  if (error || !data) {
    const code = response.status === 409 ? "email_taken" : "bad_request";
    return { ok: false, code };
  }
  if (data.verification_required) return { ok: true, verificationRequired: true };
  const res = await login(input.email, input.password);
  return { ok: res.ok, verificationRequired: false, code: res.code };
}

/**
 * Verify a pending registration with the emailed code. The endpoint sets a
 * cookie session but we have no password to mint a bearer, so the caller
 * routes to /login on success.
 */
async function verifyEmail(
  email: string,
  code: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/auth/verify", {
    body: { email, code },
  });
  if (error) {
    if (response.status === 410) return { ok: false, code: "expired" };
    if (response.status === 429) return { ok: false, code: "rate_limited" };
    return { ok: false, code: "invalid" };
  }
  return { ok: true };
}

/** Resend the registration verification code. Always best-effort (204). */
async function resendVerification(email: string): Promise<void> {
  await api.POST("/v1/auth/verify/resend", { body: { email } }).catch(() => {});
}

/** Begin a password reset. Always resolves (the API returns 204 either way). */
async function requestPasswordReset(email: string): Promise<void> {
  await api
    .POST("/v1/auth/password-reset/request", { body: { email } })
    .catch(() => {});
}

/**
 * Complete a password reset with the emailed code, then auto-login via the
 * token flow (we hold the new password).
 */
async function confirmPasswordReset(
  email: string,
  code: string,
  newPassword: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/auth/password-reset/confirm", {
    body: { email, code, new_password: newPassword },
  });
  if (error) {
    if (response.status === 410) return { ok: false, code: "code_expired" };
    if (response.status === 429) return { ok: false, code: "too_many_attempts" };
    return { ok: false, code: "invalid_code" };
  }
  return login(email, newPassword);
}

// --- Account settings (all refresh `me` on success) -------------------------

async function updateProfile(patch: {
  display_name?: string;
  week_start?: 0 | 1;
  timezone?: string;
}): Promise<{ ok: boolean }> {
  const { error } = await api.PATCH("/v1/me", { body: patch });
  if (!error) await fetchMe();
  return { ok: !error };
}

async function changePassword(
  oldPassword: string,
  newPassword: string,
): Promise<{ ok: boolean }> {
  const { error } = await api.POST("/v1/me/password", {
    body: { old_password: oldPassword, new_password: newPassword },
  });
  return { ok: !error };
}

async function requestEmailChange(
  newEmail: string,
  password: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/me/email/change-request", {
    body: { new_email: newEmail, password },
  });
  if (error) {
    if (response.status === 409) return { ok: false, code: "email_taken" };
    if (response.status === 401) return { ok: false, code: "email_password" };
    return { ok: false, code: "email_invalid" };
  }
  return { ok: true };
}

async function confirmEmailChange(code: string): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/me/email/change-confirm", {
    body: { code },
  });
  if (error) {
    if (response.status === 410) return { ok: false, code: "email_confirm_expired" };
    return { ok: false, code: "email_confirm_invalid" };
  }
  await fetchMe();
  return { ok: true };
}

async function setAvatar(pngBase64: string): Promise<{ ok: boolean }> {
  const { error } = await api.PUT("/v1/me/avatar", { body: { png_base64: pngBase64 } });
  if (!error) await fetchMe();
  return { ok: !error };
}

async function clearAvatar(): Promise<{ ok: boolean }> {
  const { error } = await api.DELETE("/v1/me/avatar");
  if (!error) await fetchMe();
  return { ok: !error };
}

type NotificationPrefs = components["schemas"]["NotificationPrefs"];

async function getNotifications(): Promise<NotificationPrefs> {
  const { data, error } = await api.GET("/v1/me/notifications");
  return error || !data ? {} : data;
}

async function updateNotifications(prefs: NotificationPrefs): Promise<{ ok: boolean }> {
  const { error } = await api.PATCH("/v1/me/notifications", { body: prefs });
  return { ok: !error };
}

/**
 * Recover the current user's password by email: fires the same reset-request
 * flow a logged-out user would, so the caller lands in the /reset code flow.
 */
async function recoverPassword(): Promise<{ ok: boolean; email?: string }> {
  if (!state.user?.email) return { ok: false };
  await requestPasswordReset(state.user.email);
  return { ok: true, email: state.user.email };
}

async function deleteAccount(password: string): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.DELETE("/v1/me", { body: { password } });
  if (error) {
    if (response.status === 423) return { ok: false, code: "delete_locked" };
    return { ok: false, code: "delete_password" };
  }
  clearAccessToken();
  state.user = null;
  return { ok: true };
}

async function logout(): Promise<void> {
  // Best-effort server-side revocation; clear local state regardless.
  await fetch(`${baseUrl}/v1/auth/token/revoke`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
  }).catch(() => {});
  clearAccessToken();
  state.user = null;
}

/**
 * Restore a session on app boot: attempt to mint an access token from the
 * refresh cookie, then load /me. Always resolves; flips state.ready.
 */
async function boot(): Promise<void> {
  try {
    const res = await fetch(`${baseUrl}/v1/auth/refresh`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
    if (res.ok) {
      const body = (await res.json()) as { access_token: string; expires_in: number };
      setAccessToken(body.access_token, body.expires_in);
      await fetchMe();
    }
  } catch {
    // Offline or no session: stay logged out.
  } finally {
    state.ready = true;
  }
}

export function useAuth() {
  return {
    state: readonly(state),
    login,
    logout,
    boot,
    refreshMe: fetchMe,
    completeSetup,
    register,
    verifyEmail,
    resendVerification,
    requestPasswordReset,
    confirmPasswordReset,
    updateProfile,
    changePassword,
    requestEmailChange,
    confirmEmailChange,
    setAvatar,
    clearAvatar,
    getNotifications,
    updateNotifications,
    recoverPassword,
    deleteAccount,
  };
}
