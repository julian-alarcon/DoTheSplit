import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);

  const raw = (form.get("week_start") ?? "").toString().trim();
  const weekStart = raw === "0" ? 0 : raw === "1" ? 1 : null;
  if (weekStart === null) return redirect("/settings?error=prefs", 302);

  const res = await apiFetch("/v1/me", {
    method: "PATCH",
    cookie,
    json: { week_start: weekStart },
  });
  if (!res.ok) return redirect("/settings?error=prefs", 302);
  return redirect("/settings?ok=prefs", 302);
};
