import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, redirectWithCookies } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const res = await apiFetch("/v1/me/email/change-confirm", {
    method: "POST",
    cookie: cookieFrom(request),
    json: { code: form.get("code") },
  });
  if (res.status === 400) return redirect("/settings?error=email_confirm_invalid&ok=email_requested", 302);
  if (res.status === 410) return redirect("/settings?error=email_confirm_expired", 302);
  if (res.status === 409) return redirect("/settings?error=email_taken", 302);
  if (!res.ok) return redirect("/settings?error=email_confirm_invalid&ok=email_requested", 302);
  // The API rotated the session - forward Set-Cookie back to the browser so
  // the new cookie replaces the old one.
  return redirectWithCookies(res, "/settings?ok=email_changed");
};
