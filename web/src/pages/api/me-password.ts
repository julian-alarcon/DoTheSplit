import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const old_password = (form.get("old_password") ?? "").toString();
  const new_password = (form.get("new_password") ?? "").toString();
  if (!old_password || new_password.length < 10) {
    return redirect("/account?error=password", 302);
  }

  const res = await fetch(`${internalBase}/v1/me/password`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ old_password, new_password }),
  });
  if (!res.ok) return redirect("/account?error=password", 302);

  // The backend revoked every session and issued a fresh cookie; forward it
  // to the browser so the user stays logged in.
  const headers = new Headers({ location: "/account?ok=password" });
  for (const c of res.headers.getSetCookie?.() ?? []) headers.append("set-cookie", c);
  return new Response(null, { status: 302, headers });
};
