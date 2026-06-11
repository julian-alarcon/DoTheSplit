import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const id = url.searchParams.get("id");
  const group = url.searchParams.get("group");
  if (!id || !group) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const res = await apiFetch(`/v1/expenses/${id}`, {
    method: "DELETE",
    cookie,
  });
  if (!res.ok && res.status !== 204) {
    return redirect(`/groups/${group}?error=expense_delete`, 302);
  }
  return redirect(`/groups/${group}`, 302);
};
