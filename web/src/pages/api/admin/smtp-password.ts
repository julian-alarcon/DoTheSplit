import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

/**
 * Reveals the stored SMTP password. Proxies to the audit-logged Go endpoint.
 * Used by the SMTP admin page to prefill the field on load.
 */
export const GET: APIRoute = async ({ request }) => {
  const res = await apiFetch("/v1/admin/smtp/password", {
    cookie: cookieFrom(request),
  });
  return new Response(await res.text(), {
    status: res.status,
    headers: { "content-type": "application/json" },
  });
};
