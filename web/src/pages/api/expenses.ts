import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();

  const amountCents = Math.round(Number(form.get("amount_dollars") ?? "0") * 100);
  const payerID = String(form.get("payer_id") ?? "");
  const description = String(form.get("description") ?? "");
  const categoryID = (form.get("category_id") ?? "").toString().trim();
  const splitIDs = form.getAll("split_user_id").map(String).filter(Boolean);

  const body: Record<string, unknown> = {
    description,
    amount_cents: amountCents,
    payer_id: payerID,
    mode: "equal",
    splits: splitIDs.map((id) => ({ user_id: id })),
  };
  if (categoryID) body.category_id = categoryID;

  await fetch(`${internalBase}/v1/groups/${groupID}/expenses`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  return redirect(`/groups/${groupID}`, 302);
};
