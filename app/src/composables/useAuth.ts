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
  };
}
