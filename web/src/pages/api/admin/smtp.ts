import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// PUT-equivalent: POST /api/admin/smtp updates the SMTP config. Form fields
// allow_plaintext_credentials and clear_password are checkboxes. Empty
// `smtp_password` + `clear_password=on` means "wipe"; empty + no flag means
// "leave alone".
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";

  const smtpPasswordRaw = (form.get("smtp_password") ?? "").toString();
  const clearPassword = form.get("clear_password") === "on";

  const body: Record<string, unknown> = {
    host: (form.get("host") ?? "").toString().trim(),
    port: parseInt((form.get("port") ?? "587").toString(), 10),
    username: ((form.get("username") ?? "").toString().trim() || null),
    from_address: (form.get("from_address") ?? "").toString().trim(),
    tls_mode: (form.get("tls_mode") ?? "starttls").toString(),
    allow_plaintext_credentials: form.get("allow_plaintext_credentials") === "on",
  };
  if (smtpPasswordRaw !== "") body.smtp_password = smtpPasswordRaw;
  else if (clearPassword) body.smtp_password = "";
  // otherwise: omit smtp_password → leave existing value untouched

  const res = await fetch(`${internalBase}/v1/admin/smtp`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(body),
  });
  if (res.ok) return redirect("/admin/smtp?saved=1", 302);
  return redirect("/admin/smtp?error=1", 302);
};
