import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const cookie = cookieFrom(request);
  const res = await apiFetch(`/v1/me/avatar`, {
    method: "DELETE",
    cookie,
  });
  if (!res.ok && res.status !== 204) {
    return redirect("/settings?error=avatar", 302);
  }
  return redirect("/settings?ok=avatar_cleared", 302);
};
