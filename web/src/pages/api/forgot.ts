import type { APIRoute } from "astro";
import { apiFetch } from "@/lib/api/forward";

// POST /api/forgot: fires the password-reset request and redirects to
// /reset. Always redirects to /reset whether or not the email is known
// (the backend already returns 204 unconditionally for enumeration safety,
// the SSR layer just keeps the same UX on top).
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = (form.get("email") ?? "").toString().trim();
  if (!email) return redirect("/forgot?error=1", 302);

  await apiFetch("/v1/auth/password-reset/request", {
    method: "POST",
    json: { email },
  });

  return redirect(`/reset?email=${encodeURIComponent(email)}`, 302);
};
