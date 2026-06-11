import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, passthroughJSON } from "@/lib/api/forward";

// Phase-1 endpoint for the per-group expense importer. Receives the raw
// CSV text from the page, forwards it to the Go API with dry_run=true,
// and pipes the JSON preview back to the client. The group id comes from
// the query string (?id=<uuid>) so the page can keep using a relative
// URL without router-aware glue.
export const POST: APIRoute = async ({ request, url }) => {
  const cookie = cookieFrom(request);
  const id = url.searchParams.get("id") ?? "";
  if (!/^[0-9a-fA-F-]{36}$/.test(id)) {
    return new Response("invalid group id", { status: 400 });
  }
  let payload: { csv?: string };
  try {
    payload = await request.json();
  } catch {
    return new Response("invalid request body", { status: 400 });
  }
  const csv = (payload.csv ?? "").toString();
  if (!csv.trim()) {
    return new Response("csv is required", { status: 400 });
  }

  const res = await apiFetch(`/v1/groups/${id}/imports/expenses`, {
    method: "POST",
    cookie,
    json: { csv, dry_run: true },
  });
  return passthroughJSON(res, await res.text());
};
