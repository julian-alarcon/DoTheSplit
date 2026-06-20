import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

// CSR SPA. Production output is static files in dist/, embedded into the Go
// binary (see api/internal/webui). In dev, /v1 and /healthz are proxied to the
// Go API on :8080 so the SPA runs same-origin against a real backend.
// Build identity. CI/Make pass these as env vars at image-build time; they're
// surfaced to the SPA as import.meta.env.VITE_BUILD_* so the footer can show
// which revision is live. Default "dev" for local builds.
const buildCommit = process.env.BUILD_COMMIT ?? "dev";
const buildVersion = process.env.BUILD_VERSION ?? "dev";

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    // Installable PWA with read-only offline. generateSW (Workbox) precaches the
    // shell + hashed assets and serves /v1 GETs network-first (cache fallback when
    // offline); mutations and the auth endpoints always hit the network. We register the SW manually
    // from main.ts (injectRegister: null) because the strict CSP (script-src
    // 'self') forbids the inline registration snippet the plugin injects by
    // default; the bundled registerSW import is CSP-clean. Updates apply silently
    // on the next navigation (autoUpdate).
    VitePWA({
      registerType: "autoUpdate",
      injectRegister: null,
      manifest: {
        name: "DoTheSplit",
        short_name: "DoTheSplit",
        description: "Share expenses with friends.",
        start_url: "/",
        scope: "/",
        display: "standalone",
        // Matches the default (dark) theme --background so the splash/system UI
        // doesn't flash. Manifest needs a literal color, so this is the hex of
        // oklch(0.165 0.04 266).
        theme_color: "#070d1f",
        background_color: "#070d1f",
        icons: [
          { src: "/pwa-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
          { src: "/pwa-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
          {
            src: "/pwa-maskable-512.png",
            sizes: "512x512",
            type: "image/png",
            purpose: "maskable",
          },
        ],
      },
      workbox: {
        globPatterns: ["**/*.{js,css,html,svg,png,ico,woff2}"],
        // Deep links / reloads to client routes get the shell; API paths never do.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/v1\//, /^\/healthz/, /^\/readyz/],
        runtimeCaching: [
          {
            // Read-only offline: cache /v1 GETs network-first. NetworkFirst (not
            // StaleWhileRevalidate) so a read issued right after a mutation - e.g.
            // the dashboard reload() after creating an expense - reflects the
            // write instead of returning a stale cached page while revalidating in
            // the background. We still fall back to the cache when the network
            // fails (offline) or stalls past the timeout, preserving read-only
            // offline. Workbox runtime caching only matches GET, so mutations pass
            // straight to the network. Exclude /v1/auth/* so tokens are never
            // cached, and the SSE stream (/v1/groups/{id}/events) so the SW never
            // tees its never-ending body into the cache - doing so buffers frames
            // and stalls real-time delivery after a few minutes.
            urlPattern: ({ url, sameOrigin }) =>
              sameOrigin &&
              url.pathname.startsWith("/v1/") &&
              !url.pathname.startsWith("/v1/auth/") &&
              !/^\/v1\/groups\/[^/]+\/events$/.test(url.pathname),
            handler: "NetworkFirst",
            options: {
              cacheName: "dts-v1-get",
              networkTimeoutSeconds: 3,
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
      devOptions: { enabled: false },
    }),
  ],
  define: {
    "import.meta.env.VITE_BUILD_COMMIT": JSON.stringify(buildCommit),
    "import.meta.env.VITE_BUILD_VERSION": JSON.stringify(buildVersion),
  },
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
