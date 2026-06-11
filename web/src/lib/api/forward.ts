// Shared plumbing for the SSR /api/*.ts forwarders, which proxy browser
// form-posts to the Go API on the Astro origin so the session cookie stays
// first-party. The typed `apiFor` client (./client.ts) can't express raw
// Set-Cookie passthrough or response streaming, so these forwarders use a
// thin fetch wrapper instead.

export const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export function cookieFrom(request: Request): string {
  return request.headers.get("cookie") ?? "";
}

// Canonical UUID shape (8-4-4-4-12 hex). The Go API mints every resource id as
// a UUID, so route handlers can validate an untrusted id from a query string or
// form field before interpolating it into a /v1/... template.
const UUID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

export function isUuid(s: string | null | undefined): s is string {
  return typeof s === "string" && UUID_RE.test(s);
}

// Our internal templates are fixed ASCII paths with UUID segments spliced in;
// they never legitimately contain a percent-encoding, a "..", or control/space
// characters. Anything else is a crafted attempt to reach a different upstream
// path (encoded traversal like %2e%2e, CRLF injection, an absolute URL, etc.).
const SAFE_PATH_RE = /^\/[A-Za-z0-9/_.-]*$/;

function isSafeUpstreamPath(path: string): boolean {
  return SAFE_PATH_RE.test(path) && !path.includes("..");
}

type ApiFetchOptions = {
  method?: string;
  cookie?: string;
  // When set, the value is JSON-encoded and Content-Type is added. Use this
  // for request bodies; omit it for GET/DELETE-without-body calls.
  json?: unknown;
  // Abort timeout in milliseconds. Defaults to 15s, which suits ordinary
  // CRUD. Heavy writes (e.g. committing a CSV import of a whole group) pass a
  // larger value so a slow-but-progressing request isn't aborted mid-flight.
  timeoutMs?: number;
};

const DEFAULT_TIMEOUT_MS = 15000;

export function apiFetch(path: string, opts: ApiFetchOptions = {}): Promise<Response> {
  // Defense-in-depth: callers interpolate user-supplied ids into fixed /v1/...
  // templates. Reject anything that isn't a plain ASCII path so encoded
  // traversal (%2e%2e), CRLF injection, or an absolute URL can't reach a
  // different upstream path. The Go API's RequireMember/RequireAdmin remains
  // the real authz gate; this just keeps malformed input from leaving Astro.
  if (!isSafeUpstreamPath(path)) {
    return Promise.resolve(new Response("bad path", { status: 400 }));
  }
  const headers: Record<string, string> = {};
  if (opts.cookie) headers.cookie = opts.cookie;
  let body: string | undefined;
  if (opts.json !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(opts.json);
  }
  return fetch(`${internalBase}${path}`, {
    method: opts.method ?? "GET",
    headers,
    body,
    // Bound the request so a hung Go backend can't tie up the Astro worker.
    signal: AbortSignal.timeout(opts.timeoutMs ?? DEFAULT_TIMEOUT_MS),
  });
}

// Builds a 302 that forwards every Set-Cookie the upstream issued. Used by the
// auth/account flows where the Go API rotates or clears the session cookie and
// the browser must see it on the Astro origin.
export function redirectWithCookies(
  upstream: Response,
  location: string,
): Response {
  const headers = new Headers({ location });
  for (const c of upstream.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  return new Response(null, { status: 302, headers });
}

// Pipes an upstream JSON response straight back to the caller, preserving its
// status. Used by the import-preview endpoints, where the client reads the
// JSON and the Go service is the source of truth.
export function passthroughJSON(upstream: Response, text: string): Response {
  return new Response(text, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "application/json",
    },
  });
}
