import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const form = await request.formData();
  const newOwnerID = String(form.get("new_owner_id") ?? "").trim();
  if (!newOwnerID) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }

  const res = await apiFetch(`/v1/groups/${groupID}`, {
    method: "PATCH",
    cookie,
    json: { created_by: newOwnerID },
  });
  if (!res.ok) {
    let message = "transfer_failed";
    try {
      const body = (await res.json()) as { message?: string };
      if (body?.message) message = body.message;
    } catch {
      // ignore
    }
    return redirect(
      `/groups/${groupID}/settings?error=1&reason=${encodeURIComponent(message)}`,
      302,
    );
  }
  return redirect(`/groups/${groupID}/settings`, 302);
};
