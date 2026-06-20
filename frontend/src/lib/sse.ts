// Minimal Server-Sent Events frame parser.
//
// We consume the activity stream via fetch + ReadableStream (not EventSource,
// which can't send the Authorization header), so we parse the wire format
// ourselves. This module is deliberately a pure function with no fetch/stream
// dependency so it's trivially unit-testable.
//
// Wire format (per the spec): frames are separated by a blank line; within a
// frame, lines are `field: value`. Lines beginning with `:` are comments
// (used here for the initial `: connected` and `: ping` heartbeats) and are
// ignored. We care about `event`, `data`, and `id`.

export interface SSEEvent {
  /** The `event:` field, defaulting to "message" per the SSE spec. */
  event: string;
  /** The `id:` field if present. */
  id?: string;
  /** Parsed JSON from the `data:` field, or undefined if absent/unparseable. */
  data: unknown;
}

export interface ParseResult {
  events: SSEEvent[];
  /** Trailing partial frame to prepend to the next chunk. */
  rest: string;
}

// parseSSEChunk appends `chunk` to the carried-over `buffer`, extracts every
// complete frame (terminated by a blank line), and returns the parsed events
// plus any incomplete trailing frame to carry into the next call. A frame whose
// `data` isn't valid JSON is still emitted with `data: undefined` rather than
// throwing, so one malformed payload can't break the stream.
export function parseSSEChunk(buffer: string, chunk: string): ParseResult {
  // Normalize CRLF/CR to LF so frame splitting is uniform.
  const combined = (buffer + chunk).replace(/\r\n|\r/g, "\n");
  const events: SSEEvent[] = [];

  let idx: number;
  let start = 0;
  // Frames are separated by a blank line ("\n\n").
  while ((idx = combined.indexOf("\n\n", start)) !== -1) {
    const frame = combined.slice(start, idx);
    start = idx + 2;
    const ev = parseFrame(frame);
    if (ev) events.push(ev);
  }

  return { events, rest: combined.slice(start) };
}

function parseFrame(frame: string): SSEEvent | null {
  let event = "message";
  let id: string | undefined;
  const dataLines: string[] = [];
  let sawField = false;

  for (const line of frame.split("\n")) {
    if (line === "" || line.startsWith(":")) continue; // blank or comment
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    // Per spec, a single leading space after the colon is stripped.
    let value = colon === -1 ? "" : line.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    switch (field) {
      case "event":
        event = value;
        sawField = true;
        break;
      case "id":
        id = value;
        sawField = true;
        break;
      case "data":
        dataLines.push(value);
        sawField = true;
        break;
      default:
        // Unknown field: ignore but count as content so we don't drop the frame.
        sawField = true;
    }
  }

  if (!sawField) return null;

  let data: unknown;
  if (dataLines.length > 0) {
    const raw = dataLines.join("\n");
    try {
      data = JSON.parse(raw);
    } catch {
      data = undefined;
    }
  }

  return { event, id, data };
}
