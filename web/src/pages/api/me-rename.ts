import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const display_name = (form.get("display_name") ?? "").toString().trim();
  if (!display_name) return redirect("/settings?error=rename", 302);

  const res = await apiFetch(`/v1/me`, {
    method: "PATCH",
    cookie,
    json: { display_name },
  });
  if (!res.ok) return redirect("/settings?error=rename", 302);
  return redirect("/settings?ok=rename", 302);
};
