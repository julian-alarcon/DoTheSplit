import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";
import {
  clearAccessToken,
  getAccessToken,
  hasValidAccessToken,
  setAccessToken,
} from "./token-store";

// Same-origin in production (the Go binary serves this bundle). In dev, Vite
// proxies /v1 to :8080, so an empty base still works. VITE_API_BASE_URL lets
// native builds point at an absolute API host.
const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

// Endpoints that must NOT carry the Authorization header or trigger the
// refresh-retry loop (they ARE the auth flow). Matched by pathname suffix.
const AUTH_PATHS = ["/v1/auth/token", "/v1/auth/refresh", "/v1/auth/token/revoke"];

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

// Read-only offline: when offline, let GET/HEAD fall through to the service
// worker's cache, but short-circuit mutations with a clear 503 so composables
// surface a sensible message instead of an opaque network failure. Returning a
// Response from onRequest bypasses the network (openapi-fetch contract).
function offlineMutationResponse(request: Request): Response | undefined {
  if (navigator.onLine) return undefined;
  if (request.method === "GET" || request.method === "HEAD") return undefined;
  if (isAuthPath(request.url)) return undefined;
  return new Response(
    JSON.stringify({
      error: "offline",
      message: "You're offline. This change can't be saved right now.",
    }),
    { status: 503, headers: { "Content-Type": "application/json" } },
  );
}

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    const offline = offlineMutationResponse(request);
    if (offline) return offline;
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

// ensureFreshToken guarantees a usable access token before a raw fetch that
// bypasses the openapi-fetch middleware (e.g. the SSE stream, which is read via
// fetch + ReadableStream because EventSource can't set the Authorization
// header). It reuses the single-flight refresh so it never races the client's
// own 401 recovery. Returns true if a token is available afterwards.
export async function ensureFreshToken(): Promise<boolean> {
  if (hasValidAccessToken()) return true;
  return refreshAccessToken();
}

export { baseUrl };
