import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// CSR SPA. Production output is static files in dist/, embedded into the Go
// binary (see api/internal/webui). In dev, /v1 and /healthz are proxied to the
// Go API on :8080 so the SPA runs same-origin against a real backend.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 4321,
    proxy: {
      "/v1": { target: "http://localhost:8080", changeOrigin: true },
      "/healthz": { target: "http://localhost:8080", changeOrigin: true },
      "/readyz": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  build: {
    // Hashed assets so the service worker / HTTP caching can cache aggressively.
    sourcemap: false,
  },
});
