import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

/**
 * Reveals the stored SMTP password. Proxies to the audit-logged Go endpoint.
 * Used by the SMTP admin page to prefill the field on load.
 */
export const GET: APIRoute = async ({ request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/admin/smtp/password`, {
    headers: { cookie },
  });
  return new Response(await res.text(), {
    status: res.status,
    headers: { "content-type": "application/json" },
  });
};
