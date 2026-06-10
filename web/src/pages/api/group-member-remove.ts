import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = cookieFrom(request);
  const form = await request.formData();
  const userID = String(form.get("user_id") ?? "").trim();
  const isLeave = String(form.get("leave") ?? "") === "1";
  if (!userID) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }

  const res = await apiFetch(
    `/v1/groups/${groupID}/members/${userID}`,
    {
      method: "DELETE",
      cookie,
    },
  );
  if (!res.ok) {
    let message = "remove_failed";
    try {
      const body = (await res.json()) as { message?: string };
      if (body?.message) message = body.message;
    } catch {
      // ignore - we'll surface the generic flag
    }
    return redirect(
      `/groups/${groupID}/settings?error=1&reason=${encodeURIComponent(message)}`,
      302,
    );
  }
  // Leaving the group → redirect to /groups; otherwise stay on settings.
  if (isLeave) return redirect(`/groups`, 302);
  return redirect(`/groups/${groupID}/settings`, 302);
};
