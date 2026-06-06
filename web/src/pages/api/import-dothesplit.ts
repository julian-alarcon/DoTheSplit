import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// Phase-2 endpoint for the dothesplit-flavored importer. Identical to
// import-splitwise.ts modulo the upstream URL.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = request.headers.get("cookie") ?? "";

  const csv = (form.get("csv") ?? "").toString();
  const groupName = (form.get("group_name") ?? "").toString().trim();
  const currency = (form.get("default_currency") ?? "")
    .toString()
    .trim()
    .toUpperCase();
  const rawCount = parseInt((form.get("member_count") ?? "0").toString(), 10);
  const n =
    Number.isFinite(rawCount) && rawCount >= 2 && rawCount <= 32 ? rawCount : 0;
  if (!n) {
    return redirect("/import/dothesplit?error=missing_members", 302);
  }
  const members = [];
  for (let i = 0; i < n; i++) {
    const csvName = (form.get(`member_${i}_name`) ?? "").toString().trim();
    const email = (form.get(`member_${i}_email`) ?? "")
      .toString()
      .trim()
      .toLowerCase();
    if (!csvName || !email) {
      return redirect("/import/dothesplit?error=missing_fields", 302);
    }
    members.push({ csv_name: csvName, email });
  }

  if (!csv || !groupName) {
    return redirect("/import/dothesplit?error=missing_fields", 302);
  }

  const res = await fetch(`${internalBase}/v1/imports/dothesplit`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({
      csv,
      group_name: groupName,
      default_currency: currency || "EUR",
      members,
      dry_run: false,
    }),
  });
  if (!res.ok) {
    let code = "import_failed";
    try {
      const body = (await res.json()) as { code?: string };
      if (body && typeof body.code === "string") code = body.code;
    } catch {
      /* empty */
    }
    return redirect(`/import/dothesplit?error=${encodeURIComponent(code)}`, 302);
  }
  const out = (await res.json()) as { group_id?: string };
  if (!out.group_id) {
    return redirect("/import/dothesplit?error=no_group_id", 302);
  }
  return redirect(`/groups/${out.group_id}`, 302);
};
