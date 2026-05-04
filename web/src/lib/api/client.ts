import createClient from "openapi-fetch";
import type { paths } from "./schema";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

/**
 * apiFor returns an openapi-fetch client bound to the given request's cookie
 * header so the Go API sees the user's session.
 */
export function apiFor(cookie: string) {
  return createClient<paths>({
    baseUrl: internalBase,
    headers: cookie ? { cookie } : {},
  });
}
