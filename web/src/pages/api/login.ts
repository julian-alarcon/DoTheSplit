import type { APIRoute } from "astro";
import { apiFetch, redirectWithCookies } from "@/lib/api/forward";

/**
 * Forwards the login form-post to the Go API and passes the Set-Cookie back
 * so the browser stores the session cookie on the Astro origin.
 *
 * 403 with code='email_unverified' means the account exists but hasn't
 * confirmed the email yet - redirect to /verify with a "resend" flag so the
 * user can pick up where they left off.
 */
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = form.get("email");
  const res = await apiFetch("/v1/auth/login", {
    method: "POST",
    json: {
      email,
      password: form.get("password"),
    },
  });
  if (res.status === 403) {
    const body = await res.json().catch(() => ({}));
    if (body?.code === "email_unverified") {
      return redirect(
        `/verify?email=${encodeURIComponent(String(email ?? ""))}&resend=1`,
        302,
      );
    }
  }
  if (!res.ok) {
    return redirect("/login?error=1", 302);
  }
  return redirectWithCookies(res, "/groups");
};
