<script setup lang="ts">
// Status banner. `tone` picks the color treatment; the slot holds the message.
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    tone?: "error" | "success" | "info";
  }>(),
  { tone: "error" },
);

const toneClass = computed(
  () =>
    ({
      error:
        "border-red-300 bg-red-50 text-red-900 dark:border-red-900 dark:bg-red-950 dark:text-red-200",
      success:
        "border-emerald-300 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200",
      info: "border-amber-300 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
    })[props.tone],
);
</script>

<template>
  <p
    class="rounded-md border px-3 py-2 text-sm"
    :class="toneClass"
    role="status"
    aria-live="polite"
  >
    <slot />
  </p>
</template>
