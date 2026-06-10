import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const form = await request.formData();
  const res = await apiFetch(`/v1/groups/${groupID}/members`, {
    method: "POST",
    cookie,
    json: { email: form.get("email") },
  });
  if (!res.ok) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/settings`, 302);
};
