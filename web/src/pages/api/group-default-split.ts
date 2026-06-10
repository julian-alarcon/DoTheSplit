import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });

  const form = await request.formData();
  const cookie = cookieFrom(request);

  const action = String(form.get("action") ?? "set");
  let body: Record<string, unknown>;
  if (action === "clear") {
    body = { default_split: [] };
  } else {
    // The form posts member_id_1, percent_1, member_id_2, percent_2.
    // Percent comes in as a 0..100 number; we convert to basis points.
    const entries: { user_id: string; basis_points: number }[] = [];
    for (let i = 1; i <= 2; i++) {
      const id = String(form.get(`member_id_${i}`) ?? "").trim();
      const pctRaw = String(form.get(`percent_${i}`) ?? "").trim();
      const pct = Number(pctRaw);
      if (!id || !Number.isFinite(pct)) {
        return redirect(`/groups/${groupID}/settings?error=1`, 302);
      }
      entries.push({ user_id: id, basis_points: Math.round(pct * 100) });
    }
    body = { default_split: entries };
  }

  const res = await apiFetch(`/v1/groups/${groupID}`, {
    method: "PATCH",
    cookie,
    json: body,
  });
  if (!res.ok) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }
  return redirect(`/groups/${groupID}/settings`, 302);
};
