<script setup lang="ts">
// Inline SVG icon. Renders a Font Awesome 7 Solid glyph from the generated
// ICON_PATHS map (see lib/icons.ts) as inline SVG markup, which the strict
// CSP permits (only inline <script>/<style> are blocked). Decorative by
// default (aria-hidden); pass `label` to make it announced.
import { computed } from "vue";
import { ICON_PATHS, ICON_VIEWBOX } from "@/lib/icons";

const props = withDefaults(
  defineProps<{
    name: string;
    size?: number;
    label?: string;
  }>(),
  { size: 16 },
);

const body = computed(() => ICON_PATHS[props.name] ?? ICON_PATHS["meteor"]);
</script>

<template>
  <svg
    :width="size"
    :height="size"
    :viewBox="ICON_VIEWBOX"
    :role="label ? 'img' : undefined"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
    v-html="body"
  />
</template>
