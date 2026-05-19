import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// POST /api/setup forwards the install-ceremony form to /v1/setup/admin.
// On success: forward the Set-Cookie header (the new admin's session) and
// redirect to /groups so the operator lands on the post-install dashboard.
// On failure: redirect back to /setup with a typed `?error=` so the page
// can render the right banner.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const res = await fetch(`${internalBase}/v1/setup/admin`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      token: form.get("token"),
      email: form.get("email"),
      password: form.get("password"),
      display_name: form.get("display_name"),
    }),
  });
  if (res.ok) {
    const headers = new Headers();
    for (const c of res.headers.getSetCookie?.() ?? []) {
      headers.append("set-cookie", c);
    }
    headers.set("location", "/groups");
    return new Response(null, { status: 302, headers });
  }
  let code = "bad_request";
  switch (res.status) {
    case 401:
      code = "invalid";
      break;
    case 409:
      code = "email_taken";
      break;
    case 410:
      code = "completed";
      break;
    case 429:
      code = "rate_limited";
      break;
  }
  return redirect(`/setup?error=${code}`, 302);
};
