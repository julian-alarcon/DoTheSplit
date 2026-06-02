import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";

  const raw = (form.get("week_start") ?? "").toString().trim();
  const weekStart = raw === "0" ? 0 : raw === "1" ? 1 : null;
  if (weekStart === null) return redirect("/settings?error=prefs", 302);

  const res = await fetch(`${internalBase}/v1/me`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ week_start: weekStart }),
  });
  if (!res.ok) return redirect("/settings?error=prefs", 302);
  return redirect("/settings?ok=prefs", 302);
};
