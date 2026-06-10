import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const form = await request.formData();
  await apiFetch(`/v1/groups/${groupID}/members`, {
    method: "POST",
    cookie,
    json: { email: form.get("email") },
  });
  return redirect(`/groups/${groupID}/settings`, 302);
};
