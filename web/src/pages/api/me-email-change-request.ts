import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const res = await apiFetch("/v1/me/email/change-request", {
    method: "POST",
    cookie,
    json: {
      new_email: form.get("new_email"),
      password: form.get("password"),
    },
  });
  if (res.status === 401) return redirect("/settings?error=email_password", 302);
  if (res.status === 409) return redirect("/settings?error=email_taken", 302);
  if (!res.ok) return redirect("/settings?error=email_invalid", 302);
  return redirect("/settings?ok=email_requested", 302);
};
