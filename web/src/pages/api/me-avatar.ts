import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const png_base64 = (form.get("png_base64") ?? "").toString();
  if (!png_base64) return redirect("/settings?error=avatar", 302);

  const res = await fetch(`${internalBase}/v1/me/avatar`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ png_base64 }),
  });
  if (!res.ok) return redirect("/settings?error=avatar", 302);
  return redirect("/settings?ok=avatar", 302);
};
