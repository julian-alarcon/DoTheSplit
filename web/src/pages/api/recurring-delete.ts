import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const recurringID = url.searchParams.get("id");
  const groupID = url.searchParams.get("group");
  if (!recurringID || !groupID) return new Response("missing id", { status: 400 });

  const cookie = cookieFrom(request);
  const res = await apiFetch(`/v1/recurring-expenses/${recurringID}`, {
    method: "DELETE",
    cookie,
  });
  if (!res.ok && res.status !== 204) {
    return redirect(`/groups/${groupID}/recurring?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/recurring`, 302);
};
