import type { APIRoute } from "astro";
import { apiFetch, redirectWithCookies } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const email = String(form.get("email") ?? "");
  const code = String(form.get("code") ?? "");
  const res = await apiFetch("/v1/auth/verify", {
    method: "POST",
    json: { email, code },
  });
  const back = (err: string) =>
    redirect(
      `/verify?email=${encodeURIComponent(email)}&error=${err}`,
      302,
    );
  if (res.status === 400) return back("invalid");
  if (res.status === 410) return back("expired");
  if (res.status === 429) return back("rate_limited");
  if (!res.ok) return back("1");
  return redirectWithCookies(res, "/groups");
};
