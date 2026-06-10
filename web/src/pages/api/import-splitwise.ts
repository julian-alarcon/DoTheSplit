import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

// Phase-2 endpoint. Submitted as a regular HTML form, so we read formData()
// and forward a JSON commit (dry_run=false) to the Go API. On success we
// redirect to the new group page; on failure we send the user back to the
// importer with an error code in the query string.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);

  const csv = (form.get("csv") ?? "").toString();
  const groupName = (form.get("group_name") ?? "").toString().trim();
  const currency = (form.get("default_currency") ?? "").toString().trim().toUpperCase();
  const rawCount = parseInt((form.get("member_count") ?? "0").toString(), 10);
  const n = Number.isFinite(rawCount) && rawCount >= 2 && rawCount <= 32 ? rawCount : 0;
  if (!n) {
    return redirect("/import/splitwise?error=missing_members", 302);
  }
  const members = [];
  for (let i = 0; i < n; i++) {
    const csvName = (form.get(`member_${i}_name`) ?? "").toString().trim();
    const email = (form.get(`member_${i}_email`) ?? "").toString().trim().toLowerCase();
    if (!csvName || !email) {
      return redirect("/import/splitwise?error=missing_fields", 302);
    }
    members.push({ csv_name: csvName, email });
  }

  if (!csv || !groupName) {
    return redirect("/import/splitwise?error=missing_fields", 302);
  }

  const res = await apiFetch("/v1/imports/splitwise", {
    method: "POST",
    cookie,
    json: {
      csv,
      group_name: groupName,
      default_currency: currency || "EUR",
      members,
      dry_run: false,
    },
  });
  if (!res.ok) {
    let code = "import_failed";
    try {
      const body = (await res.json()) as { code?: string };
      if (body && typeof body.code === "string") code = body.code;
    } catch {
      /* empty */
    }
    return redirect(`/import/splitwise?error=${encodeURIComponent(code)}`, 302);
  }
  const out = (await res.json()) as { group_id?: string };
  if (!out.group_id) {
    return redirect("/import/splitwise?error=no_group_id", 302);
  }
  return redirect(`/groups/${out.group_id}`, 302);
};
