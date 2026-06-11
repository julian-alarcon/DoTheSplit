import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, passthroughJSON } from "@/lib/api/forward";

const HEADER_PREFIX = ["Date", "Description", "Category", "Cost", "Currency"] as const;

// Inspect the CSV header just enough to know how many user columns there
// are; the backend re-parses the whole file and is the source of truth.
function userColumnCount(csv: string): number {
  const firstLine = csv.split(/\r?\n/, 1)[0] ?? "";
  const cols = firstLine.split(",").map((c) => c.trim());
  if (cols.length < HEADER_PREFIX.length + 2) return 2;
  for (let i = 0; i < HEADER_PREFIX.length; i++) {
    if (cols[i] !== HEADER_PREFIX[i]) return 2;
  }
  return cols.length - HEADER_PREFIX.length;
}

// Pick the first non-empty 3-letter ISO code in the Currency column. We
// only need it as a sensible default for the dry-run; the backend re-parses
// and reports every distinct currency back as `csv_currencies` so the page
// can warn on mixed-currency inputs.
function firstCurrency(csv: string): string {
  const lines = csv.split(/\r?\n/);
  for (let i = 1; i < lines.length; i++) {
    const cols = lines[i].split(",");
    if (cols.length <= HEADER_PREFIX.length) continue;
    const cur = (cols[4] ?? "").trim().toUpperCase();
    if (cur.length === 3 && /^[A-Z]{3}$/.test(cur)) return cur;
  }
  return "EUR";
}

// Phase-1 endpoint. Receives JSON {csv, group_name_hint} from the client,
// fans out to the Go API with dry_run=true, and returns the JSON preview
// directly. The Go service is the only place the CSV is parsed - the client
// is just a transport here, so we don't trust any of its derived fields.
export const POST: APIRoute = async ({ request }) => {
  const cookie = cookieFrom(request);
  let payload: { csv?: string; group_name_hint?: string };
  try {
    payload = await request.json();
  } catch {
    return new Response("invalid request body", { status: 400 });
  }
  const csv = (payload.csv ?? "").toString();
  if (!csv.trim()) {
    return new Response("csv is required", { status: 400 });
  }
  const groupName = (payload.group_name_hint ?? "Imported group").toString().slice(0, 80) || "Imported group";

  // The Go service requires the member count to match the CSV. Generate
  // throwaway placeholder emails for the dry-run; nothing is persisted
  // because dry_run=true short-circuits before any DB writes.
  const n = Math.max(2, Math.min(32, userColumnCount(csv)));
  const placeholders = Array.from({ length: n }, (_, i) => ({
    csv_name: `_${i}`,
    email: `preview-${i}@example.invalid`,
  }));

  const res = await apiFetch("/v1/imports/splitwise", {
    method: "POST",
    cookie,
    json: {
      csv,
      group_name: groupName,
      default_currency: firstCurrency(csv),
      members: placeholders,
      dry_run: true,
    },
  });
  return passthroughJSON(res, await res.text());
};
