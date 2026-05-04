import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const body: Record<string, unknown> = { name: form.get("name") };
  const currency = (form.get("default_currency") ?? "").toString().trim();
  if (currency) body.default_currency = currency.toUpperCase();
  const res = await fetch(`${internalBase}/v1/groups`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  if (!res.ok) return redirect("/groups/new?error=1", 302);
  const g = (await res.json()) as { id: string };
  return redirect(`/groups/${g.id}`, 302);
};
