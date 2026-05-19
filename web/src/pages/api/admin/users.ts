import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// POST /api/admin/users — admin creates a user on behalf. Server-side form
// post; the SSR origin keeps the session cookie out of client JS.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";
  const body = {
    email: (form.get("email") ?? "").toString().trim(),
    display_name: (form.get("display_name") ?? "").toString().trim(),
    password: (form.get("password") ?? "").toString(),
    role: (form.get("role") ?? "user").toString(),
  };
  const res = await fetch(`${internalBase}/v1/admin/users`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  if (!res.ok) return redirect("/admin/users?create_error=1", 302);
  return redirect("/admin/users?created=1", 302);
};
