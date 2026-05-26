import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts"],
    exclude: ["tests/e2e/**", "node_modules/**"],
    // Astro's tsconfig uses paths but our tests don't need the alias resolver.
    // jsdom lacks createImageBitmap; tests stay on pure helpers, not the
    // canvas-touching pixelateFile() entry point.
    globals: false,
    css: false,
  },
});
