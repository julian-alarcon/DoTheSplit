import type { APIRoute } from "astro";
import { apiFetch, cookieFrom, redirectWithCookies } from "@/lib/api/forward";

export const POST: APIRoute = async ({ request }) => {
  const res = await apiFetch("/v1/auth/logout", {
    method: "POST",
    cookie: cookieFrom(request),
  });
  return redirectWithCookies(res, "/login");
};
