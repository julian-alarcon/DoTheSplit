import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

// POST /api/me-password-recover: invoked by the "Recover password by email"
// button on /settings/password. Looks up the caller's own email, fires the
// password-reset request through the normal forgot-password backend path
// (so the user lands in the same /reset code-paste flow as a logged-out
// user would), then redirects to /reset with the email pre-filled.
export const POST: APIRoute = async ({ request, redirect }) => {
  const cookie = cookieFrom(request);

  const meRes = await apiFetch("/v1/me", { cookie });
  if (!meRes.ok) {
    return redirect("/login", 302);
  }
  const me = (await meRes.json()) as { email?: string };
  const email = me.email ?? "";
  if (!email) {
    return redirect("/settings/password?error=unknown", 302);
  }

  // Fire-and-forget: backend always returns 204 to avoid enumeration.
  await apiFetch("/v1/auth/password-reset/request", {
    method: "POST",
    cookie,
    json: { email },
  });

  return redirect(`/reset?email=${encodeURIComponent(email)}&from=settings`, 302);
};
