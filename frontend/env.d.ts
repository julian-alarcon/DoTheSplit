/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

interface ImportMetaEnv {
  /** Base URL of the Go API. Empty string = same origin (production, served by the API binary). */
  readonly VITE_API_BASE_URL?: string;
  /** Git short SHA, baked in at image-build time; "dev" for local dev. */
  readonly VITE_BUILD_COMMIT?: string;
  /** release-please semver, baked in at image-build time; "dev" for local dev. */
  readonly VITE_BUILD_VERSION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}
