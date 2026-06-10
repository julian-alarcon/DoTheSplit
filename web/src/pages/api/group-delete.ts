import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });

  const cookie = cookieFrom(request);
  const res = await apiFetch(`/v1/groups/${groupID}`, {
    method: "DELETE",
    cookie,
  });
  if (!res.ok && res.status !== 204) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }
  return redirect(`/groups`, 302);
};
