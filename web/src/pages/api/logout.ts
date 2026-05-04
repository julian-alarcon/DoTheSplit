import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/auth/logout`, {
    method: "POST",
    headers: { cookie },
  });
  const headers = new Headers();
  for (const c of res.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  headers.set("location", "/login");
  return new Response(null, { status: 302, headers });
};
