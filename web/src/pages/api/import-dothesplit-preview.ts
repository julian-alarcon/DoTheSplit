import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, passthroughJSON } from "@/lib/api/forward";

const HEADER_PREFIX = ["Date", "Description", "Category", "Cost", "Currency"] as const;
// Optional named columns the dothesplit exporter inserts between
// Currency and the per-member block. Anything matching one of these
// (case-insensitive) does not count toward the user-column total.
const OPTIONAL_COLUMNS = new Set(["time", "payer", "notes", "created", "createdby"]);

// Inspect the header just enough to count user columns. The backend
// re-parses the whole file and is the source of truth.
function userColumnCount(csv: string): number {
  const firstLine = csv.split(/\r?\n/, 1)[0] ?? "";
  const cols = firstLine.split(",").map((c) => c.trim());
  if (cols.length < HEADER_PREFIX.length + 2) return 2;
  for (let i = 0; i < HEADER_PREFIX.length; i++) {
    if (cols[i] !== HEADER_PREFIX[i]) return 2;
  }
  let i = HEADER_PREFIX.length;
  while (i < cols.length && OPTIONAL_COLUMNS.has(cols[i].toLowerCase())) i++;
  return cols.length - i;
}

function firstCurrency(csv: string): string {
  const lines = csv.split(/\r?\n/);
  for (let i = 1; i < lines.length; i++) {
    const cols = lines[i].split(",");
    if (cols.length <= 4) continue;
    const cur = (cols[4] ?? "").trim().toUpperCase();
    if (cur.length === 3 && /^[A-Z]{3}$/.test(cur)) return cur;
  }
  return "EUR";
}

// Phase-1 endpoint. Receives JSON {csv, group_name_hint} from the
// client, forwards to the Go API with dry_run=true, returns the JSON
// preview. Mirrors import-splitwise-preview.ts but routes to
// /v1/imports/dothesplit so the richer parser is used.
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
  const groupName =
    (payload.group_name_hint ?? "Imported group").toString().slice(0, 80) ||
    "Imported group";

  const n = Math.max(2, Math.min(32, userColumnCount(csv)));
  const placeholders = Array.from({ length: n }, (_, i) => ({
    csv_name: `_${i}`,
    email: `preview-${i}@example.invalid`,
  }));

  const res = await apiFetch("/v1/imports/dothesplit", {
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
