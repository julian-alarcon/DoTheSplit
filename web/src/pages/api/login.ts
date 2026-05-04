import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

/**
 * Forwards the login form-post to the Go API and passes the Set-Cookie back
 * so the browser stores the session cookie on the Astro origin.
 */
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const res = await fetch(`${internalBase}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email: form.get("email"),
      password: form.get("password"),
    }),
  });
  if (!res.ok) {
    return redirect("/login?error=1", 302);
  }
  const headers = new Headers();
  for (const c of res.headers.getSetCookie?.() ?? []) {
    headers.append("set-cookie", c);
  }
  headers.set("location", "/groups");
  return new Response(null, { status: 302, headers });
};
