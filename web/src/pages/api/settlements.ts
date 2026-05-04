import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();
  const amountCents = Math.round(Number(form.get("amount_dollars") ?? "0") * 100);
  const toUserID = String(form.get("to_user_id") ?? "");
  const res = await fetch(`${internalBase}/v1/groups/${groupID}/settlements`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({
      to_user_id: toUserID,
      amount_cents: amountCents,
    }),
  });
  if (!res.ok) {
    // Bounce back to the settle page so the user can fix their input.
    return redirect(`/groups/${groupID}/settle?error=1`, 302);
  }
  return redirect(`/groups/${groupID}`, 302);
};
