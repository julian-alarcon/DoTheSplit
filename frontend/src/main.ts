import { createApp } from "vue";
import { registerSW } from "virtual:pwa-register";
import App from "./App.vue";
import { router } from "./router";
import { installFormValidation } from "./lib/form-validation";
import "./styles/global.css";

installFormValidation();

// Register the service worker (read-only offline PWA). Bundled into the hashed
// entry chunk, not an injected inline snippet, so it satisfies the strict CSP
// (script-src 'self'). immediate: true activates a new SW as soon as it's ready;
// updates then apply silently on the next navigation (autoUpdate).
registerSW({ immediate: true });

const app = createApp(App);
// Last-resort handler for errors thrown in render functions, watchers, and
// lifecycle hooks that nothing else caught. Log only: there is no shared toast
// surface yet, and we never log anything beyond the error and Vue's info hint.
app.config.errorHandler = (err, _instance, info) => {
  console.error(`[vue] unhandled error (${info})`, err);
};
app.use(router).mount("#app");
