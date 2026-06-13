// Timezone resolution for SSR. The display follows a 3-tier waterfall:
//   1. User override (user.timezone)
//   2. Device-detected dts_tz cookie set by an inline script in Base.astro
//   3. "UTC" fallback (the very first request from a new browser, before the
//      cookie roundtrip)
// All inputs are validated against the runtime tzdata; invalid candidates fall
// through to the next tier so a tampered cookie can never crash the formatter.

const validZones: Set<string> | null = (() => {
  try {
    const fn = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf;
    if (typeof fn === "function") return new Set(fn("timeZone"));
  } catch {
    // Fall through to the per-call probe below.
  }
  return null;
})();

function isValidTimezone(tz: string): boolean {
  if (!tz) return false;
  if (validZones) return validZones.has(tz);
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

function readDtsTzCookie(cookie: string): string | null {
  for (const part of cookie.split(";")) {
    const trimmed = part.trim();
    if (trimmed.startsWith("dts_tz=")) {
      return decodeURIComponent(trimmed.slice("dts_tz=".length));
    }
  }
  return null;
}

export function resolveTimezone(
  userTimezone: string | null | undefined,
  cookie: string,
): string {
  if (userTimezone && isValidTimezone(userTimezone)) return userTimezone;
  const cookieTz = readDtsTzCookie(cookie);
  if (cookieTz && isValidTimezone(cookieTz)) return cookieTz;
  return "UTC";
}
