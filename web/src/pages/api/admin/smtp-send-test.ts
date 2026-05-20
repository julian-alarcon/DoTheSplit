import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/admin/smtp/send-test`, {
    method: "POST",
    headers: { cookie },
  });
  if (res.status === 404) return redirect("/admin/smtp?send=not_configured", 302);
  if (!res.ok) return redirect("/admin/smtp?send=error", 302);
  const data = (await res.json()) as { success: boolean; error?: string };
  if (data.success) return redirect("/admin/smtp?send=ok", 302);
  return redirect("/admin/smtp?send=fail", 302);
};
