import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, redirectWithCookies } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const password = (form.get("password") ?? "").toString();
  if (!password) return redirect("/settings?error=delete_password", 302);
  const res = await apiFetch("/v1/me", {
    method: "DELETE",
    cookie: cookieFrom(request),
    json: { password },
  });
  if (res.status === 401) return redirect("/settings?error=delete_password", 302);
  if (res.status === 423) return redirect("/settings?error=delete_locked", 302);
  // Forward the session-clearing Set-Cookie from the API, and send the user
  // home with a goodbye.
  return redirectWithCookies(res, "/login");
};
