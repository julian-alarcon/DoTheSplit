import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// Same endpoint as /api/me-password but redirects to /groups on success and
// keeps the user on /account/force-password-change on failure. Used after an
// admin reset, when EnforcePasswordChange gates the rest of the API.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const old_password = (form.get("old_password") ?? "").toString();
  const new_password = (form.get("new_password") ?? "").toString();
  if (!old_password || new_password.length < 10) {
    return redirect("/account/force-password-change?error=invalid", 302);
  }
  const res = await fetch(`${internalBase}/v1/me/password`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ old_password, new_password }),
  });
  if (!res.ok) return redirect("/account/force-password-change?error=wrong", 302);
  const headers = new Headers({ location: "/groups?password_changed=1" });
  for (const c of res.headers.getSetCookie?.() ?? []) headers.append("set-cookie", c);
  return new Response(null, { status: 302, headers });
};
