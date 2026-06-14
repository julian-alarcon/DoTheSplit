<script setup lang="ts">
// Status banner. Replaces the Astro tier's inline role="status" message
// blocks. `tone` picks the color treatment; the slot holds the message.
withDefaults(
  defineProps<{
    tone?: "error" | "success" | "info";
  }>(),
  { tone: "error" },
);
</script>

<template>
  <p class="alert" :class="`alert-${tone}`" role="status" aria-live="polite">
    <slot />
  </p>
</template>

<style scoped>
.alert {
  border-radius: 0.375rem;
  border: 1px solid;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
}
.alert-error {
  border-color: color-mix(in oklch, var(--destructive) 50%, transparent);
  background: color-mix(in oklch, var(--destructive) 12%, var(--card));
  color: var(--destructive);
}
.alert-success {
  border-color: color-mix(in oklch, var(--primary) 50%, transparent);
  background: color-mix(in oklch, var(--primary) 12%, var(--card));
  color: var(--foreground);
}
.alert-info {
  border-color: var(--border);
  background: var(--muted);
  color: var(--muted-foreground);
}
</style>
