import { defineConfig, fontProviders } from "astro/config";
import node from "@astrojs/node";
import icon from "astro-icon";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  output: "server",
  adapter: node({ mode: "standalone" }),
  integrations: [icon()],
  // Self-hosted Inter - keeps font bytes on our origin so the app never reaches
  // fonts.googleapis.com / fonts.gstatic.com. License text and attribution live
  // in src/assets/fonts/inter/OFL.txt and the /about page.
  fonts: [
    {
      provider: fontProviders.local(),
      name: "Inter",
      cssVariable: "--font-inter",
      fallbacks: ["system-ui", "sans-serif"],
      options: {
        // Only the weights we actually use: 400 Regular and 600 SemiBold.
        // `font-medium` (500) is synthesized by the browser from 400 - no
        // separate file needed. Italics and 700-Bold were preloaded but
        // never actually rendered, costing ~345 KB on every page load.
        variants: [
          { weight: 400, style: "normal", src: ["./src/assets/fonts/inter/Inter-Regular.woff2"] },
          { weight: 600, style: "normal", src: ["./src/assets/fonts/inter/Inter-SemiBold.woff2"] },
        ],
      },
    },
  ],
  server: {
    host: "0.0.0.0",
    port: 3000,
  },
  security: {
    checkOrigin: false,
    // CSP: Astro auto-hashes its own bundled scripts (client islands,
    // <script src> modules) but NOT author-written <script is:inline> blocks -
    // those need their hash added by hand (see scriptDirective below).
    // Tailwind-injected inline <style> tags still need 'unsafe-inline'.
    csp: {
      algorithm: "SHA-256",
      styleDirective: {
        resources: ["'self'", "'unsafe-inline'"],
      },
      // Hash of the synchronous theme-boot is:inline block in Base.astro.
      // Astro auto-hashes its own bundled scripts but not is:inline blocks, so
      // this one must be registered by hand. Recompute if that script changes.
      scriptDirective: {
        hashes: ["sha256-9Jmc8B18ot7J+hV7HcN7yjjh7HnGwBbvXBSgLb3rKs0="],
      },
    },
  },
  vite: {
    plugins: [tailwindcss()],
  },
});
