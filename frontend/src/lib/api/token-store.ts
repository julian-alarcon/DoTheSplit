// In-memory access-token holder. The access token never touches
// localStorage on web (XSS-minimization, per the migration security brief):
// it lives only in this module's closure for the tab's lifetime. The refresh
// token lives in the httpOnly `dts_refresh` cookie, invisible to JS.
//
// On native (Capacitor) a different persistence layer wires in via
// setTokens/onCleared, but the in-memory access token is still the single
// source of truth the API client reads on every request.

let accessToken: string | null = null;
let expiresAt = 0; // epoch ms; 0 = unknown/none

export function getAccessToken(): string | null {
  return accessToken;
}

/** True when we hold a token that is not within `skewMs` of expiry. */
export function hasValidAccessToken(skewMs = 5000): boolean {
  return accessToken !== null && Date.now() < expiresAt - skewMs;
}

export function setAccessToken(token: string, expiresInSeconds: number): void {
  accessToken = token;
  expiresAt = Date.now() + expiresInSeconds * 1000;
}

export function clearAccessToken(): void {
  accessToken = null;
  expiresAt = 0;
}
