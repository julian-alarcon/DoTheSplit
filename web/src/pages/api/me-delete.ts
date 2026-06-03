import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const password = (form.get("password") ?? "").toString();
  if (!password) return redirect("/settings?error=delete_password", 302);
  const res = await fetch(`${internalBase}/v1/me`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ password }),
  });
  if (res.status === 401) return redirect("/settings?error=delete_password", 302);
  if (res.status === 423) return redirect("/settings?error=delete_locked", 302);
  // Forward the session-clearing Set-Cookie from the API, and send the user
  // home with a goodbye.
  const headers = new Headers({ location: "/login" });
  for (const c of res.headers.getSetCookie?.() ?? []) headers.append("set-cookie", c);
  return new Response(null, { status: 302, headers });
};
