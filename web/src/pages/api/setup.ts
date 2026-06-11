import type { APIRoute } from "astro";
import { apiFetch, redirectWithCookies } from "@/lib/api/forward";

// POST /api/setup forwards the install-ceremony form to /v1/setup/admin.
// On success: forward the Set-Cookie header (the new admin's session) and
// redirect to /groups so the operator lands on the post-install dashboard.
// On failure: redirect back to /setup with a typed `?error=` so the page
// can render the right banner.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const res = await apiFetch("/v1/setup/admin", {
    method: "POST",
    json: {
      token: form.get("token"),
      email: form.get("email"),
      password: form.get("password"),
      display_name: form.get("display_name"),
    },
  });
  if (res.ok) {
    return redirectWithCookies(res, "/groups");
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
