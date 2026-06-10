import type { APIRoute } from "astro";
import { apiFetch } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = String(form.get("email") ?? "");
  await apiFetch("/v1/auth/verify/resend", {
    method: "POST",
    json: { email },
  });
  // Always redirect with a friendly notice - the API also always returns
  // 204 to avoid account enumeration.
  return redirect(
    `/verify?email=${encodeURIComponent(email)}&resent=1`,
    302,
  );
};
