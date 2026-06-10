import type { APIRoute } from "astro";
import { apiFetch, redirectWithCookies } from "@/lib/api/forward";

/**
 * Forwards the register form-post. The API returns 201 either way; the
 * `verification_required` flag in the body decides whether we redirect to
 * /verify (SMTP configured) or /groups (auto-verified, cookie set).
 */
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = form.get("email");
  const res = await apiFetch("/v1/auth/register", {
    method: "POST",
    json: {
      email,
      password: form.get("password"),
      display_name: form.get("display_name"),
    },
  });
  if (!res.ok) {
    return redirect("/register?error=1", 302);
  }
  const body = await res.json().catch(() => ({}));
  // The API only issues a cookie when SMTP is unset and the user was
  // auto-verified; otherwise we send them to /verify to enter the code.
  const location = body?.verification_required
    ? `/verify?email=${encodeURIComponent(String(email ?? ""))}`
    : "/groups";
  return redirectWithCookies(res, location);
};
