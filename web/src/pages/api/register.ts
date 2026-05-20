import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

/**
 * Forwards the register form-post. The API returns 201 either way; the
 * `verification_required` flag in the body decides whether we redirect to
 * /verify (SMTP configured) or /groups (auto-verified, cookie set).
 */
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = form.get("email");
  const res = await fetch(`${internalBase}/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email,
      password: form.get("password"),
      display_name: form.get("display_name"),
    }),
  });
  if (!res.ok) {
    return redirect("/register?error=1", 302);
  }
  const body = await res.json().catch(() => ({}));
  const headers = new Headers();
  // Forward any cookies the API issued (only present when SMTP is unset and
  // the user was auto-verified).
  for (const c of res.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  if (body?.verification_required) {
    headers.set(
      "location",
      `/verify?email=${encodeURIComponent(String(email ?? ""))}`,
    );
  } else {
    headers.set("location", "/groups");
  }
  return new Response(null, { status: 302, headers });
};
