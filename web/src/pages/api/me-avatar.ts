import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const png_base64 = (form.get("png_base64") ?? "").toString();
  if (!png_base64) return redirect("/settings?error=avatar", 302);

  const res = await apiFetch(`/v1/me/avatar`, {
    method: "PUT",
    cookie,
    json: { png_base64 },
  });
  if (!res.ok) return redirect("/settings?error=avatar", 302);
  return redirect("/settings?ok=avatar", 302);
};
