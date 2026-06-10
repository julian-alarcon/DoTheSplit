import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });

  const form = await request.formData();
  const cookie = cookieFrom(request);

  const body: Record<string, unknown> = {};
  const name = (form.get("name") ?? "").toString().trim();
  if (name) body.name = name;
  const currency = (form.get("default_currency") ?? "").toString().trim();
  if (currency) body.default_currency = currency.toUpperCase();

  if (Object.keys(body).length === 0) {
    return redirect(`/groups/${groupID}/settings`, 302);
  }

  const res = await apiFetch(`/v1/groups/${groupID}`, {
    method: "PATCH",
    cookie,
    json: body,
  });
  if (!res.ok) {
    if (res.status === 409) {
      const reason = "Currency is locked once the group has expenses or settlements.";
      return redirect(
        `/groups/${groupID}/settings?error=1&reason=${encodeURIComponent(reason)}`,
        302,
      );
    }
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/settings`, 302);
};
