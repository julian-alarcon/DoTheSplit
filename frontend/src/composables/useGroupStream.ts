// Real-time group activity stream (module singleton, like useNetworkStatus).
//
// Opens the group's SSE endpoint (GET /v1/groups/{id}/events) and invokes a
// caller-supplied handler on each `activity` event. We use fetch + a
// ReadableStream reader rather than the native EventSource because EventSource
// can't set the Authorization header, and the API authenticates this stream
// with the bearer token like every other request (the access token must never
// go in the URL, which the access logger records).
//
// Only one group is streamed at a time (the dashboard views a single group),
// so the state is a singleton. The stream auto-reconnects with backoff, pauses
// while offline, and surfaces a reactive `connected` flag for an optional live
// indicator.
import { readonly, ref } from "vue";

import { baseUrl, ensureFreshToken } from "@/lib/api/client";
import { getAccessToken } from "@/lib/api/token-store";
import { parseSSEChunk, type SSEEvent } from "@/lib/sse";
import { useNetworkStatus } from "@/composables/useNetworkStatus";

// The minimal signal the server sends (mirrors realtime.Event). Clients react
// by re-fetching, so this carries ids only.
export interface ActivitySignal {
  id: string;
  group_id: string;
  actor_id: string | null;
  action: string;
  expense_id: string | null;
  settlement_id: string | null;
  created_at: string;
}

type SignalHandler = (signal: ActivitySignal) => void;

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

const connected = ref(false);

// Module-level connection state. A run token guards against races: each start()
// bumps it, and any in-flight loop bound to a stale token exits quietly.
let currentGroupId: string | null = null;
let handler: SignalHandler | null = null;
let controller: AbortController | null = null;
let runToken = 0;
let reconnectDelay = RECONNECT_BASE_MS;
let networkWatchAttached = false;

const { online } = useNetworkStatus();

function attachNetworkWatch(): void {
  if (networkWatchAttached) return;
  networkWatchAttached = true;
  // Re-open when connectivity returns; close when it drops.
  window.addEventListener("online", () => {
    if (currentGroupId && handler) void connect(runToken);
  });
  window.addEventListener("offline", () => {
    controller?.abort();
  });
}

function fullJitter(ms: number): number {
  // Deterministic-enough jitter without Math.random dependency concerns: spread
  // within [ms/2, ms]. Uses a rotating offset seeded by the run token.
  const frac = ((runToken * 2654435761) % 1000) / 1000;
  return Math.floor(ms / 2 + (ms / 2) * frac);
}

async function connect(token: number): Promise<void> {
  if (token !== runToken || !currentGroupId || !handler) return;
  if (!online.value) return; // resume on the `online` event

  const ok = await ensureFreshToken();
  if (token !== runToken) return;
  if (!ok) return; // logged out; the router guard will handle it

  // Abort any prior stream before opening a new one so a spurious `online`
  // event (some browsers fire it without a preceding `offline`) can't leave two
  // concurrent connections delivering duplicate events.
  controller?.abort();
  controller = new AbortController();
  const url = `${baseUrl}/v1/groups/${currentGroupId}/events`;
  try {
    const res = await fetch(url, {
      headers: { Authorization: `Bearer ${getAccessToken()}` },
      credentials: "include",
      signal: controller.signal,
    });
    if (token !== runToken) return;
    if (res.status === 401) {
      // Token expired between refresh and connect; refresh once and retry.
      const refreshed = await ensureFreshToken();
      if (refreshed && token === runToken) return scheduleReconnect(token, true);
      return;
    }
    if (!res.ok || !res.body) {
      return scheduleReconnect(token);
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    connected.value = true;
    reconnectDelay = RECONNECT_BASE_MS;

    for (;;) {
      const { value, done } = await reader.read();
      if (done || token !== runToken) break;
      const { events, rest } = parseSSEChunk(buffer, decoder.decode(value, { stream: true }));
      buffer = rest;
      for (const ev of events) dispatch(ev, token);
    }
  } catch {
    // Network error / abort: fall through to reconnect handling below.
  } finally {
    if (token === runToken) connected.value = false;
  }

  if (token === runToken) scheduleReconnect(token);
}

function dispatch(ev: SSEEvent, token: number): void {
  if (token !== runToken || ev.event !== "activity" || !handler) return;
  const signal = ev.data as ActivitySignal | undefined;
  if (signal && signal.group_id === currentGroupId) handler(signal);
}

function scheduleReconnect(token: number, immediate = false): void {
  if (token !== runToken || !online.value) return;
  const delay = immediate ? 0 : fullJitter(reconnectDelay);
  reconnectDelay = Math.min(reconnectDelay * 2, RECONNECT_MAX_MS);
  window.setTimeout(() => {
    void connect(token);
  }, delay);
}

export function useGroupStream() {
  /**
   * Begin streaming `groupId`, invoking `onSignal` for each activity event in
   * that group. Replaces any existing stream. Idempotent for the same group +
   * handler is NOT assumed; callers should stop() before switching groups.
   */
  function start(groupId: string, onSignal: SignalHandler): void {
    attachNetworkWatch();
    controller?.abort();
    runToken += 1;
    currentGroupId = groupId;
    handler = onSignal;
    reconnectDelay = RECONNECT_BASE_MS;
    void connect(runToken);
  }

  /** Close the stream and clear state. Safe to call when not streaming. */
  function stop(): void {
    runToken += 1; // invalidate any in-flight loop / pending reconnect
    currentGroupId = null;
    handler = null;
    connected.value = false;
    controller?.abort();
    controller = null;
  }

  return { connected: readonly(connected), start, stop };
}
