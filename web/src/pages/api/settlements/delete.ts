import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const id = url.searchParams.get("id");
  const group = url.searchParams.get("group");
  if (!id || !group) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  await apiFetch(`/v1/settlements/${id}`, {
    method: "DELETE",
    cookie,
  });
  return redirect(`/groups/${group}`, 302);
};
