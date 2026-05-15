import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";

  // Empty value means "use device timezone" (clears the override). Any
  // non-empty value is forwarded as-is; the API validates IANA names.
  const tz = (form.get("timezone") ?? "").toString().trim();

  const res = await fetch(`${internalBase}/v1/me`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ timezone: tz }),
  });
  if (!res.ok) return redirect("/account?error=tz", 302);
  return redirect("/account?ok=tz", 302);
};
