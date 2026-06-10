import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, redirectWithCookies } from "@/lib/api/forward";

// POST /api/me-password: handles the change-password form on
// /settings/password. Validates that `password_confirmation` matches
// `new_password` server-side too (we can't trust the inline JS check), then
// forwards to the Go API which enforces the current-password gate.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const old_password = (form.get("old_password") ?? "").toString();
  const new_password = (form.get("new_password") ?? "").toString();
  const password_confirmation = (form.get("password_confirmation") ?? "").toString();

  if (!old_password) {
    return redirect("/settings/password?error=current", 302);
  }
  if (new_password.length < 10) {
    return redirect("/settings/password?error=length", 302);
  }
  if (new_password !== password_confirmation) {
    return redirect("/settings/password?error=mismatch", 302);
  }

  const res = await apiFetch("/v1/me/password", {
    method: "POST",
    cookie,
    json: { old_password, new_password },
  });
  if (res.status === 401) {
    return redirect("/settings/password?error=wrong_current", 302);
  }
  if (!res.ok) {
    return redirect("/settings/password?error=unknown", 302);
  }

  // Backend revoked every session and issued a fresh cookie; forward it so
  // the user stays logged in on the same browser.
  return redirectWithCookies(res, "/settings?ok=password");
};
