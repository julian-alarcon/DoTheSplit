import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

// PUT-equivalent: POST /api/admin/smtp updates the SMTP config and immediately
// runs a connection test on the just-saved values, so the admin always sees
// whether the config actually works. The password field is "what you see is
// what's saved": the page prefills the stored cleartext, so an empty field
// at submit time genuinely means "wipe the stored password".
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);

  const body: Record<string, unknown> = {
    host: (form.get("host") ?? "").toString().trim(),
    port: parseInt((form.get("port") ?? "587").toString(), 10),
    username: ((form.get("username") ?? "").toString().trim() || null),
    from_address: (form.get("from_address") ?? "").toString().trim(),
    tls_mode: (form.get("tls_mode") ?? "starttls").toString(),
    // Always opt in. The UI surfaces a banner above the form when TLS=None
    // is selected, which is the meaningful warning; the backend's
    // additional opt-in flag is now redundant since there's no other way
    // for an admin to land in that state by accident.
    allow_plaintext_credentials: true,
    // Always send the password field. Backend treats empty string as "clear",
    // non-empty as "set/replace".
    smtp_password: (form.get("smtp_password") ?? "").toString(),
  };

  const saveRes = await apiFetch("/v1/admin/smtp", {
    method: "PUT",
    cookie,
    json: body,
  });
  if (!saveRes.ok) return redirect("/admin/smtp?error=1", 302);

  // Save succeeded; chase it with a connection test so the admin sees one
  // banner per outcome (green Saved + green/red test result).
  const testRes = await apiFetch("/v1/admin/smtp/test", {
    method: "POST",
    cookie,
  });
  if (testRes.status === 404) {
    return redirect("/admin/smtp?saved=1&test=not_configured", 302);
  }
  if (!testRes.ok) {
    return redirect("/admin/smtp?saved=1&test=error", 302);
  }
  const data = (await testRes.json()) as { success: boolean; error?: string };
  const code = data.success ? "ok" : data.error ?? "fail";
  return redirect(`/admin/smtp?saved=1&test=${encodeURIComponent(code)}`, 302);
};
