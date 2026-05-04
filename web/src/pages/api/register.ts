import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const res = await fetch(`${internalBase}/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email: form.get("email"),
      password: form.get("password"),
      display_name: form.get("display_name"),
    }),
  });
  if (!res.ok) {
    return redirect("/register?error=1", 302);
  }
  const headers = new Headers();
  for (const c of res.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  headers.set("location", "/groups");
  return new Response(null, { status: 302, headers });
};
