import { defineMiddleware } from "astro/middleware";
import { apiFor } from "@/lib/api/client";
import { resolveTimezone } from "@/lib/timezone";
import { resolveLocale } from "@/lib/locale";

export const onRequest = defineMiddleware(async (ctx, next) => {
  const cookie = ctx.request.headers.get("cookie") ?? "";
  ctx.locals.cookie = cookie;
  ctx.locals.user = null;

  if (cookie.includes("dts_session=")) {
    const api = apiFor(cookie);
    const { data } = await api.GET("/v1/me");
    if (data) ctx.locals.user = data;
  }

  ctx.locals.timezone = resolveTimezone(ctx.locals.user?.timezone, cookie);
  ctx.locals.locale = resolveLocale(ctx.request.headers.get("accept-language"));

  const path = ctx.url.pathname;
  const isPublic =
    path === "/login" ||
    path === "/register" ||
    path === "/credits" ||
    path.startsWith("/api/") ||
    path === "/favicon.ico";

  if (!ctx.locals.user && !isPublic) {
    return ctx.redirect("/login");
  }
  if (ctx.locals.user && (path === "/login" || path === "/register")) {
    return ctx.redirect("/groups");
  }

  // Force-password-change: an admin reset this user's password. Only the
  // dedicated change-password page and the SSR API forwarders (one of which
  // is the password-change endpoint) are reachable until they pick a new one.
  const forcePath = "/account/force-password-change";
  if (
    ctx.locals.user?.must_change_password &&
    path !== forcePath &&
    !path.startsWith("/api/") &&
    path !== "/credits" &&
    path !== "/favicon.ico"
  ) {
    return ctx.redirect(forcePath);
  }

  // Admin guard: any /admin/* page requires the role flag on the resolved
  // user. Non-admins land on /groups instead of seeing a 403 leak.
  if (path.startsWith("/admin") && !ctx.locals.user?.is_admin) {
    return ctx.redirect("/groups");
  }

  // Admin SSR responses must not be cached by intermediaries.
  if (path.startsWith("/admin")) {
    const res = await next();
    res.headers.set("Cache-Control", "no-store");
    res.headers.set("X-Frame-Options", "DENY");
    return res;
  }
  return next();
});
