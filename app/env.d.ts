/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the Go API. Empty string = same origin (production, served by the API binary). */
  readonly VITE_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}
