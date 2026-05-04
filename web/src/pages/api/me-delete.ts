import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/me`, {
    method: "DELETE",
    headers: { cookie },
  });
  // Forward the session-clearing Set-Cookie from the API, and send the user
  // home with a goodbye.
  const headers = new Headers({ location: "/login" });
  for (const c of res.headers.getSetCookie?.() ?? []) headers.append("set-cookie", c);
  return new Response(null, { status: 302, headers });
};
