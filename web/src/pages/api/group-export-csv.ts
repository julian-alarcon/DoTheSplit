import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

// Submitted as a regular HTML form from the group settings page. We
// forward a GET to the Go API with the user's cookie and stream the
// CSV response through, preserving Content-Disposition so the browser
// saves it as a file. On non-2xx we redirect back to settings with an
// error indicator (the existing settings page already renders that).
export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });

  const upstream = await apiFetch(`/v1/groups/${groupID}/export.csv`, {
    cookie: cookieFrom(request),
  });
  if (!upstream.ok) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }
  const headers = new Headers();
  const ct = upstream.headers.get("content-type") ?? "text/csv; charset=utf-8";
  headers.set("Content-Type", ct);
  const cd = upstream.headers.get("content-disposition");
  if (cd) headers.set("Content-Disposition", cd);
  return new Response(upstream.body, { status: 200, headers });
};
