import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const settlementID = url.searchParams.get("id");
  const groupID = url.searchParams.get("group");
  if (!settlementID || !groupID) return new Response("missing id", { status: 400 });

  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();

  const body: Record<string, unknown> = {};
  const fromUserID = (form.get("from_user_id") ?? "").toString().trim();
  if (fromUserID) body.from_user_id = fromUserID;
  const toUserID = (form.get("to_user_id") ?? "").toString().trim();
  if (toUserID) body.to_user_id = toUserID;
  const amount = (form.get("amount_dollars") ?? "").toString().trim();
  if (amount) body.amount_cents = Math.round(Number(amount) * 100);
  const settledAt = (form.get("settled_at") ?? "").toString().trim();
  if (settledAt && /^\d{4}-\d{2}-\d{2}$/.test(settledAt)) {
    body.settled_at = `${settledAt}T12:00:00Z`;
  }
  const noteRaw = form.get("note");
  if (noteRaw !== null) {
    body.note = noteRaw.toString();
  }

  if (Object.keys(body).length === 0) {
    return redirect(`/groups/${groupID}/settlements/${settlementID}`, 302);
  }

  const res = await fetch(`${internalBase}/v1/settlements/${settlementID}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    return redirect(`/groups/${groupID}/settlements/${settlementID}?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/settlements/${settlementID}`, 302);
};
