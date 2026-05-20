import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = String(form.get("email") ?? "");
  const code = String(form.get("code") ?? "");
  const res = await fetch(`${internalBase}/v1/auth/verify`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, code }),
  });
  const back = (err: string) =>
    redirect(
      `/verify?email=${encodeURIComponent(email)}&error=${err}`,
      302,
    );
  if (res.status === 400) return back("invalid");
  if (res.status === 410) return back("expired");
  if (res.status === 429) return back("rate_limited");
  if (!res.ok) return back("1");
  const headers = new Headers();
  for (const c of res.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  headers.set("location", "/groups");
  return new Response(null, { status: 302, headers });
};
