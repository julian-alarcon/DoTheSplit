import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const expenseID = url.searchParams.get("id");
  const groupID = url.searchParams.get("group");
  if (!expenseID || !groupID) return new Response("missing id", { status: 400 });

  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();

  const body: Record<string, unknown> = {};
  const description = (form.get("description") ?? "").toString().trim();
  if (description) body.description = description;
  const amount = (form.get("amount_dollars") ?? "").toString().trim();
  if (amount) body.amount_cents = Math.round(Number(amount) * 100);
  const categoryID = (form.get("category_id") ?? "").toString().trim();
  if (categoryID) body.category_id = categoryID;
  const payerID = (form.get("payer_id") ?? "").toString().trim();
  if (payerID) body.payer_id = payerID;

  if (Object.keys(body).length === 0) {
    return redirect(`/groups/${groupID}/expenses/${expenseID}`, 302);
  }

  const res = await fetch(`${internalBase}/v1/expenses/${expenseID}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    return redirect(`/groups/${groupID}/expenses/${expenseID}?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/expenses/${expenseID}`, 302);
};
