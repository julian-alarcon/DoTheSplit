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

  // Optional: invite members by email at creation time. Each email becomes one
  // POST /v1/groups/{id}/members call; the API returns 404 for unregistered
  // emails and 200/201 otherwise. We surface a banner on the group page when
  // any invites failed so the user can retry from group settings.
  const rawEmails = (form.get("member_emails") ?? "").toString();
  const emails = rawEmails
    .split(/[\n,;]+/)
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean);
  if (emails.length === 0) {
    return redirect(`/groups/${g.id}`, 302);
  }
  const results = await Promise.all(
    emails.map((email) =>
      fetch(`${internalBase}/v1/groups/${g.id}/members`, {
        method: "POST",
        headers: { "Content-Type": "application/json", cookie },
        body: JSON.stringify({ email }),
      }),
    ),
  );
  const failed = results.filter((r) => !r.ok).length;
  const suffix = failed > 0 ? `?invite_failed=${failed}` : "";
  return redirect(`/groups/${g.id}${suffix}`, 302);
};
