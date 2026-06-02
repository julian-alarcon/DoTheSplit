import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const display_name = (form.get("display_name") ?? "").toString().trim();
  if (!display_name) return redirect("/settings?error=rename", 302);

  const res = await fetch(`${internalBase}/v1/me`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ display_name }),
  });
  if (!res.ok) return redirect("/settings?error=rename", 302);
  return redirect("/settings?ok=rename", 302);
};
