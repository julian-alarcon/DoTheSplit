import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router";
import { installFormValidation } from "./lib/form-validation";
import "./styles/global.css";

installFormValidation();

createApp(App).use(router).mount("#app");
