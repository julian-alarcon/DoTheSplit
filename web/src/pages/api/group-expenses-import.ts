import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

// Phase-2 endpoint: commits the import. Reads the raw CSV from the
// hidden form field, posts dry_run=false to the Go API, then redirects
// to the group dashboard so the user lands on the page with the new
// expenses already visible.
export const POST: APIRoute = async ({ request, url, redirect }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const id = url.searchParams.get("id") ?? "";
  if (!/^[0-9a-fA-F-]{36}$/.test(id)) {
    return redirect("/groups", 302);
  }
  const form = await request.formData();
  const csv = (form.get("csv") ?? "").toString();
  if (!csv.trim()) {
    return redirect(
      `/groups/${id}/import-expenses?error=missing_csv`,
      302,
    );
  }
  const res = await fetch(
    `${internalBase}/v1/groups/${id}/imports/expenses`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json", cookie },
      body: JSON.stringify({ csv, dry_run: false }),
    },
  );
  if (!res.ok) {
    let code = "import_failed";
    try {
      const body = (await res.json()) as { code?: string };
      if (body && typeof body.code === "string") code = body.code;
    } catch {
      /* empty */
    }
    return redirect(
      `/groups/${id}/import-expenses?error=${encodeURIComponent(code)}`,
      302,
    );
  }
  return redirect(`/groups/${id}`, 302);
};
