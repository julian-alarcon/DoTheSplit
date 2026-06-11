import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);

  // Empty value means "use device timezone" (clears the override). Any
  // non-empty value is forwarded as-is; the API validates IANA names.
  const tz = (form.get("timezone") ?? "").toString().trim();

  const res = await apiFetch("/v1/me", {
    method: "PATCH",
    cookie,
    json: { timezone: tz },
  });
  if (!res.ok) return redirect("/settings?error=tz", 302);
  return redirect("/settings?ok=tz", 302);
};
