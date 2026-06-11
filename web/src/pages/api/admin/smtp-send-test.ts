import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, redirect }) => {
  const cookie = cookieFrom(request);
  const res = await apiFetch("/v1/admin/smtp/send-test", {
    method: "POST",
    cookie,
  });
  if (res.status === 404) return redirect("/admin/smtp?send=not_configured", 302);
  if (!res.ok) return redirect("/admin/smtp?send=error", 302);
  const data = (await res.json()) as { success: boolean; error?: string };
  if (data.success) return redirect("/admin/smtp?send=ok", 302);
  return redirect("/admin/smtp?send=fail", 302);
};
