import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// POST /api/admin/user-reset-password — set another user's password and
// force them to change it on next login. Always redirects back to the
// per-user detail page so the admin sees success/failure in context.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const id = (form.get("id") ?? "").toString();
  if (!id) return redirect("/admin/users?reset_error=missing_id", 302);
  const detail = `/admin/users/${id}`;
  const res = await fetch(`${internalBase}/v1/admin/users/${id}/password`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({
      new_password: (form.get("new_password") ?? "").toString(),
      password: (form.get("password") ?? "").toString(),
    }),
  });
  if (res.status === 204) return redirect(`${detail}?reset=1`, 302);
  if (res.status === 401) return redirect(`${detail}?reset_error=step_up`, 302);
  if (res.status === 423) return redirect(`${detail}?reset_error=locked`, 302);
  return redirect(`${detail}?reset_error=1`, 302);
};
