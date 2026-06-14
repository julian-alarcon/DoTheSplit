/*
 * Client-side 8x8 avatar pipeline.
 *
 * Pure helpers + a pixelateFile() that renders any image into an 8x8
 * pixelated + saturated canvas and returns the upload PNG (the 8x8 tile the
 * server validates) plus a nearest-neighbour preview upscale. The Vue avatar
 * UI calls these; the server never sees the original image. GDPR-friendly:
 * you can't identify a human in 64 pixels.
 *
 * The color math (rgbToHsl / hslToRgb, saturation push to 1.0) is load-bearing
 * and pinned by avatar-pixelate.test.ts - keep it in lockstep with the server.
 */

/** Source grid side length. Must stay in sync with AvatarClientSize on the server. */
export const SOURCE = 8;

export type RGB = [number, number, number];

export function rgbToHsl([r, g, b]: RGB): [number, number, number] {
  const rn = r / 255;
  const gn = g / 255;
  const bn = b / 255;
  const max = Math.max(rn, gn, bn);
  const min = Math.min(rn, gn, bn);
  const l = (max + min) / 2;
  if (max === min) return [0, 0, l];
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  let h = 0;
  switch (max) {
    case rn:
      h = (gn - bn) / d + (gn < bn ? 6 : 0);
      break;
    case gn:
      h = (bn - rn) / d + 2;
      break;
    case bn:
      h = (rn - gn) / d + 4;
      break;
  }
  return [h / 6, s, l];
}

export function hslToRgb(h: number, s: number, l: number): RGB {
  if (s === 0) {
    const v = Math.round(l * 255);
    return [v, v, v];
  }
  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;
  const hueToRgb = (t: number) => {
    if (t < 0) t += 1;
    if (t > 1) t -= 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };
  return [
    Math.round(hueToRgb(h + 1 / 3) * 255),
    Math.round(hueToRgb(h) * 255),
    Math.round(hueToRgb(h - 1 / 3) * 255),
  ];
}

/**
 * Returns a square source rect (centered crop) so the 8x8 output isn't
 * squished when the input is 16:9 or portrait.
 */
export function centerSquare(w: number, h: number): { sx: number; sy: number; s: number } {
  const s = Math.min(w, h);
  return { sx: Math.floor((w - s) / 2), sy: Math.floor((h - s) / 2), s };
}

export function toBase64NoPrefix(dataUrl: string): string {
  const i = dataUrl.indexOf(",");
  return i === -1 ? dataUrl : dataUrl.slice(i + 1);
}

const PREVIEW_SCALE = 8; // 8 * 8 = 64 preview pixels, fits the 64px slot exactly
const PREVIEW_SIZE = SOURCE * PREVIEW_SCALE;

/**
 * Produces two PNG data URLs from the source image:
 *   - `uploadDataUrl`: the 8x8 bitmap the server expects and validates.
 *   - `previewDataUrl`: a nearest-neighbour upscale so the <img> shows crisp
 *     blocks without relying on `image-rendering: pixelated`.
 */
export async function pixelateFile(
  file: File,
): Promise<{ uploadDataUrl: string; previewDataUrl: string }> {
  const bitmap = await createImageBitmap(file);
  try {
    const canvas = document.createElement("canvas");
    canvas.width = SOURCE;
    canvas.height = SOURCE;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("2D context unavailable");
    ctx.imageSmoothingEnabled = false;

    const { sx, sy, s } = centerSquare(bitmap.width, bitmap.height);
    ctx.drawImage(bitmap, sx, sy, s, s, 0, 0, SOURCE, SOURCE);

    // Saturation boost. Saturation -> 1.0 gives a "fun pixel" vibe on dull inputs.
    const img = ctx.getImageData(0, 0, SOURCE, SOURCE);
    for (let i = 0; i < img.data.length; i += 4) {
      const [h, , l] = rgbToHsl([img.data[i], img.data[i + 1], img.data[i + 2]]);
      const [r, g, b] = hslToRgb(h, 1, l);
      img.data[i] = r;
      img.data[i + 1] = g;
      img.data[i + 2] = b;
      img.data[i + 3] = 255;
    }
    ctx.putImageData(img, 0, 0);

    const previewCanvas = document.createElement("canvas");
    previewCanvas.width = PREVIEW_SIZE;
    previewCanvas.height = PREVIEW_SIZE;
    const pctx = previewCanvas.getContext("2d");
    if (!pctx) throw new Error("2D context unavailable");
    pctx.imageSmoothingEnabled = false;
    pctx.drawImage(canvas, 0, 0, PREVIEW_SIZE, PREVIEW_SIZE);

    return {
      uploadDataUrl: canvas.toDataURL("image/png"),
      previewDataUrl: previewCanvas.toDataURL("image/png"),
    };
  } finally {
    bitmap.close();
  }
}
