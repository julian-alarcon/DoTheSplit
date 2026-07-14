import { describe, expect, it } from "vitest";
import {
  centerSquare,
  hslToRgb,
  rgbToHsl,
  SOURCE,
  toBase64NoPrefix,
} from "./avatar-pixelate";

describe("rgbToHsl <-> hslToRgb", () => {
  // Round-trip every primary, secondary, and a handful of greys. The pipeline
  // pushes saturation to 1.0 then converts back, so the conversion math has
  // to be tight - drift here shows up as wrong avatar colors.
  const cases: Array<[string, [number, number, number]]> = [
    ["red", [255, 0, 0]],
    ["green", [0, 255, 0]],
    ["blue", [0, 0, 255]],
    ["yellow", [255, 255, 0]],
    ["cyan", [0, 255, 255]],
    ["magenta", [255, 0, 255]],
    ["white", [255, 255, 255]],
    ["mid_grey", [128, 128, 128]],
    ["dark_grey", [42, 42, 42]],
  ];
  for (const [name, rgb] of cases) {
    it(`round-trips ${name}`, () => {
      const [h, s, l] = rgbToHsl(rgb);
      const back = hslToRgb(h, s, l);
      // Allow ±1 per channel: the HSL math is float-precision-bound and the
      // Math.round on the way back can flip a borderline value. Anything
      // larger means the conversion is wrong.
      for (let i = 0; i < 3; i++) {
        expect(Math.abs(back[i] - rgb[i])).toBeLessThanOrEqual(1);
      }
    });
  }
});

describe("hslToRgb saturation push", () => {
  it("flooring saturation to 0 yields a grey", () => {
    const [r, g, b] = hslToRgb(0.33, 0, 0.5);
    expect(r).toEqual(g);
    expect(g).toEqual(b);
  });
  it("pushing saturation to 1.0 keeps hue", () => {
    // Take a desaturated red, push S to 1, expect a vivid red back.
    const [h, , l] = rgbToHsl([200, 100, 100]);
    const [r, g, b] = hslToRgb(h, 1, l);
    expect(r).toBeGreaterThan(g);
    expect(r).toBeGreaterThan(b);
  });
});

describe("centerSquare", () => {
  it("returns the full square when width == height", () => {
    expect(centerSquare(8, 8)).toEqual({ sx: 0, sy: 0, s: 8 });
  });
  it("center-crops a landscape rectangle", () => {
    // 16 wide, 9 tall: square side = 9, x offset (16-9)/2 = 3, y offset 0.
    expect(centerSquare(16, 9)).toEqual({ sx: 3, sy: 0, s: 9 });
  });
  it("center-crops a portrait rectangle", () => {
    // 9 wide, 16 tall: square side = 9, x offset 0, y offset (16-9)/2 = 3.
    expect(centerSquare(9, 16)).toEqual({ sx: 0, sy: 3, s: 9 });
  });
  it("handles odd dimensions with floor (no NaN, no negatives)", () => {
    const r = centerSquare(15, 9);
    expect(r.sx).toEqual(3); // floor((15-9)/2) = 3
    expect(r.sy).toEqual(0);
    expect(r.s).toEqual(9);
  });
});

describe("toBase64NoPrefix", () => {
  it("strips a data URL prefix", () => {
    expect(toBase64NoPrefix("data:image/png;base64,iVBORw0KGgo=")).toEqual(
      "iVBORw0KGgo=",
    );
  });
  it("returns the input unchanged when there is no comma", () => {
    expect(toBase64NoPrefix("iVBORw0KGgo=")).toEqual("iVBORw0KGgo=");
  });
  it("handles empty input", () => {
    expect(toBase64NoPrefix("")).toEqual("");
  });
});

describe("SOURCE constant", () => {
  it("matches the server-side AvatarClientSize", () => {
    // Pinned to 8 by the server contract (server/internal/service/me.go).
    // If anyone changes this, the round-trip with the server breaks.
    expect(SOURCE).toEqual(8);
  });
});
