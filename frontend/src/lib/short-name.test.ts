import { describe, expect, it } from "vitest";
import { shortName } from "./short-name";

describe("shortName", () => {
  it("keeps first name + last initial when it fits under 10 chars", () => {
    expect(shortName("Diana Yermanos")).toBe("Diana Y.");
  });

  it("falls back to a truncated first name when the short form is too long", () => {
    expect(shortName("Maximiliano")).toBe("Maximilia...");
  });

  it("handles single short names without truncation", () => {
    expect(shortName("Ana")).toBe("Ana");
  });

  it("returns ? for empty or missing names", () => {
    expect(shortName("")).toBe("?");
    expect(shortName(null)).toBe("?");
    expect(shortName(undefined)).toBe("?");
  });
});
