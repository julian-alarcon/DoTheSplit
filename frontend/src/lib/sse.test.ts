import { describe, expect, it } from "vitest";
import { parseSSEChunk } from "./sse";

describe("parseSSEChunk", () => {
  it("parses a single complete frame with JSON data", () => {
    const frame = 'event: activity\ndata: {"action":"expense.created"}\n\n';
    const { events, rest } = parseSSEChunk("", frame);
    expect(rest).toBe("");
    expect(events).toHaveLength(1);
    expect(events[0].event).toBe("activity");
    expect(events[0].data).toEqual({ action: "expense.created" });
  });

  it("carries an incomplete frame across two chunks", () => {
    const first = parseSSEChunk("", 'event: activity\ndata: {"a":');
    expect(first.events).toHaveLength(0);
    expect(first.rest).not.toBe("");

    const second = parseSSEChunk(first.rest, '1}\n\n');
    expect(second.events).toHaveLength(1);
    expect(second.events[0].data).toEqual({ a: 1 });
    expect(second.rest).toBe("");
  });

  it("parses multiple frames in one chunk", () => {
    const chunk =
      'event: activity\ndata: {"n":1}\n\n' + 'event: activity\ndata: {"n":2}\n\n';
    const { events } = parseSSEChunk("", chunk);
    expect(events).toHaveLength(2);
    expect(events[0].data).toEqual({ n: 1 });
    expect(events[1].data).toEqual({ n: 2 });
  });

  it("ignores comment lines (heartbeats) and the connected ping", () => {
    const chunk = ": connected\n\n" + ": ping\n\n";
    const { events, rest } = parseSSEChunk("", chunk);
    expect(events).toHaveLength(0);
    expect(rest).toBe("");
  });

  it("parses the id field", () => {
    const frame = "id: abc-123\nevent: activity\ndata: {}\n\n";
    const { events } = parseSSEChunk("", frame);
    expect(events[0].id).toBe("abc-123");
  });

  it("emits the event with undefined data when JSON is malformed", () => {
    const frame = "event: activity\ndata: {not json}\n\n";
    const { events } = parseSSEChunk("", frame);
    expect(events).toHaveLength(1);
    expect(events[0].data).toBeUndefined();
  });

  it("normalizes CRLF line endings", () => {
    const frame = 'event: activity\r\ndata: {"ok":true}\r\n\r\n';
    const { events } = parseSSEChunk("", frame);
    expect(events).toHaveLength(1);
    expect(events[0].data).toEqual({ ok: true });
  });

  it("defaults the event type to message when no event field is present", () => {
    const frame = 'data: {"x":1}\n\n';
    const { events } = parseSSEChunk("", frame);
    expect(events[0].event).toBe("message");
  });

  it("joins multi-line data fields with newlines before parsing", () => {
    // Per the SSE spec, repeated `data:` lines are concatenated with "\n".
    const frame = 'event: activity\ndata: {"a":1,\ndata: "b":2}\n\n';
    const { events } = parseSSEChunk("", frame);
    expect(events).toHaveLength(1);
    expect(events[0].data).toEqual({ a: 1, "b": 2 });
  });

  it("carries a frame whose blank-line terminator arrives in the next chunk", () => {
    // The JSON is complete but the frame isn't terminated yet (only one "\n").
    const first = parseSSEChunk("", 'event: activity\ndata: {"a":1}\n');
    expect(first.events).toHaveLength(0);
    expect(first.rest).not.toBe("");

    const second = parseSSEChunk(first.rest, "\n");
    expect(second.events).toHaveLength(1);
    expect(second.events[0].data).toEqual({ a: 1 });
    expect(second.rest).toBe("");
  });

  it("emits an event with no data field (data undefined) when only id/event present", () => {
    const frame = "id: abc\nevent: activity\n\n";
    const { events } = parseSSEChunk("", frame);
    expect(events).toHaveLength(1);
    expect(events[0].id).toBe("abc");
    expect(events[0].data).toBeUndefined();
  });

  it("normalizes lone-CR line endings", () => {
    const frame = 'event: activity\rdata: {"ok":true}\r\r';
    const { events } = parseSSEChunk("", frame);
    expect(events).toHaveLength(1);
    expect(events[0].data).toEqual({ ok: true });
  });
});
