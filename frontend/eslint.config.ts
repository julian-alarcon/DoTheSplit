import pluginVue from "eslint-plugin-vue";
import {
  defineConfigWithVueTs,
  vueTsConfigs,
} from "@vue/eslint-config-typescript";

// Flat config. Strict TypeScript + vue-tsc already cover type safety; this adds
// the Vue-specific static analysis they cannot see (v-for keys, reactivity
// pitfalls, template correctness). We lint src only; generated, build, and
// public classic-script output stay out of scope.
export default defineConfigWithVueTs(
  {
    name: "app/ignores",
    ignores: ["dist/**", "public/**", "src/lib/api/schema.d.ts"],
  },
  {
    name: "app/files",
    files: ["src/**/*.{ts,vue}"],
  },
  pluginVue.configs["flat/recommended"],
  vueTsConfigs.recommended,
  {
    name: "app/rule-overrides",
    files: ["src/**/*.{ts,vue}"],
    rules: {
      // Formatting-only template rules that fight the project's deliberate
      // house style: long single-line Tailwind class strings, single-line
      // elements, and no Prettier (a stated project choice). They produce pure
      // noise here, so they are off. Type/correctness rules stay on.
      "vue/max-attributes-per-line": "off",
      "vue/singleline-html-element-content-newline": "off",
      "vue/multiline-html-element-content-newline": "off",
      "vue/html-self-closing": "off",
      "vue/html-indent": "off",
      "vue/html-closing-bracket-newline": "off",
      // Type-based optional props (`prop?: T`) need no runtime default in
      // <script setup>; this rule misfires on every one of them.
      "vue/require-default-prop": "off",
      // Single-word components (Icon, Alert, Field, Avatar) are intentional and
      // never collide with native elements in our templates.
      "vue/multi-word-component-names": "off",
      // Icon.vue renders trusted, generated inline SVG path data (CSP-clean per
      // the project docs); v-html is deliberate and the input is not user data.
      "vue/no-v-html": "off",
    },
  },
);
