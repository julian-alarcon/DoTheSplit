import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const body = {
    // Unchecked checkboxes are absent from FormData; coerce to false explicitly
    // so the server PATCH replaces every flag rather than relying on omission.
    notify_recurring_run: form.get("notify_recurring_run") === "1",
    notify_settlement: form.get("notify_settlement") === "1",
    notify_group_added: form.get("notify_group_added") === "1",
  };
  const res = await apiFetch("/v1/me/notifications", {
    method: "PATCH",
    cookie,
    json: body,
  });
  if (!res.ok) return redirect("/settings/notifications?error=1", 302);
  return redirect("/settings/notifications?ok=saved", 302);
};
