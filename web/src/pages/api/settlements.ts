import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const form = await request.formData();
  const amountCents = Math.round(Number(form.get("amount_dollars") ?? "0") * 100);
  const toUserID = String(form.get("to_user_id") ?? "");
  const fromUserID = String(form.get("from_user_id") ?? "");
  const body: Record<string, unknown> = {
    to_user_id: toUserID,
    amount_cents: amountCents,
  };
  if (fromUserID) body.from_user_id = fromUserID;
  const res = await apiFetch(`/v1/groups/${groupID}/settlements`, {
    method: "POST",
    cookie,
    json: body,
  });
  if (!res.ok) {
    // Bounce back to the settle page so the user can fix their input.
    return redirect(`/groups/${groupID}/settle?error=1`, 302);
  }
  return redirect(`/groups/${groupID}`, 302);
};
