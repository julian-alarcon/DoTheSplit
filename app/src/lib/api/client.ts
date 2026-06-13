import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";
import { clearAccessToken, getAccessToken, setAccessToken } from "./token-store";

// Same-origin in production (the Go binary serves this bundle). In dev, Vite
// proxies /v1 to :8080, so an empty base still works. VITE_API_BASE_URL lets
// native builds point at an absolute API host.
const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

// Endpoints that must NOT carry the Authorization header or trigger the
// refresh-retry loop (they ARE the auth flow). Matched by pathname suffix.
const AUTH_PATHS = ["/v1/auth/token", "/v1/auth/refresh", "/v1/auth/token/revoke", "/v1/auth/login"];

function isAuthPath(url: string): boolean {
  return AUTH_PATHS.some((p) => url.includes(p));
}

// Single-flight refresh: concurrent 401s share one /v1/auth/refresh call.
let refreshInFlight: Promise<boolean> | null = null;

async function refreshAccessToken(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async () => {
    try {
      const res = await fetch(`${baseUrl}/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
      if (!res.ok) {
        clearAccessToken();
        return false;
      }
      const body = (await res.json()) as { access_token: string; expires_in: number };
      setAccessToken(body.access_token, body.expires_in);
      return true;
    } catch {
      clearAccessToken();
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    if (!isAuthPath(request.url)) {
      const token = getAccessToken();
      if (token) request.headers.set("Authorization", `Bearer ${token}`);
    }
    return request;
  },
  async onResponse({ request, response }) {
    if (response.status !== 401 || isAuthPath(request.url)) return response;
    // Try one refresh, then replay the original request once.
    const ok = await refreshAccessToken();
    if (!ok) return response;
    const retry = new Request(request.url, request);
    const token = getAccessToken();
    if (token) retry.headers.set("Authorization", `Bearer ${token}`);
    return fetch(retry);
  },
};

export const api = createClient<paths>({ baseUrl, credentials: "include" });
api.use(authMiddleware);

export { baseUrl };
