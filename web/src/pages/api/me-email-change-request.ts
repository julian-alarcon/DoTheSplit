import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/me/email/change-request`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({
      new_email: form.get("new_email"),
      password: form.get("password"),
    }),
  });
  if (res.status === 401) return redirect("/settings?error=email_password", 302);
  if (res.status === 409) return redirect("/settings?error=email_taken", 302);
  if (!res.ok) return redirect("/settings?error=email_invalid", 302);
  return redirect("/settings?ok=email_requested", 302);
};
