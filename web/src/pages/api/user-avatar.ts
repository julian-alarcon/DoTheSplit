import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

/**
 * Streaming proxy for `/v1/users/{id}/avatar` so the browser talks to the
 * same origin as Astro (avoids CORS + keeps the session cookie working).
 */
export const GET: APIRoute = async ({ request, url }) => {
  const id = url.searchParams.get("id");
  if (!id) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  const upstream = await fetch(`${internalBase}/v1/users/${id}/avatar`, {
    headers: { cookie },
  });
  if (!upstream.ok) {
    return new Response(null, { status: upstream.status });
  }
  const buf = await upstream.arrayBuffer();
  return new Response(buf, {
    status: 200,
    headers: {
      "Content-Type": "image/png",
      "Cache-Control": "private, max-age=86400",
    },
  });
};
