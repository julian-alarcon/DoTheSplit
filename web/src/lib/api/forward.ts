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

type ApiFetchOptions = {
  method?: string;
  cookie?: string;
  // When set, the value is JSON-encoded and Content-Type is added. Use this
  // for request bodies; omit it for GET/DELETE-without-body calls.
  json?: unknown;
};

export function apiFetch(path: string, opts: ApiFetchOptions = {}): Promise<Response> {
  // Reject path traversal. Callers interpolate user-supplied UUIDs into fixed
  // /v1/... templates; UUIDs never contain dots, so a ".." segment can only be
  // a crafted attempt to reach a different internal path.
  if (path.includes("..")) {
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
    signal: AbortSignal.timeout(15000),
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
