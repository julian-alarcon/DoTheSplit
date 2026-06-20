<script setup lang="ts">
import { RouterLink } from "vue-router";
import AppLayout from "@/components/AppLayout.vue";

const sources = [
  { to: "/import/splitwise", title: "Splitwise", desc: "Upload a CSV export from Splitwise (any group size)." },
  { to: "/import/dothesplit", title: "DoTheSplit", desc: "Upload a CSV produced by Group settings → Export. Preserves time, payer and notes." },
];
</script>

<template>
  <AppLayout :back="{ to: '/groups', label: 'Groups' }">
    <h1 class="mb-2 text-2xl font-semibold">Import a group</h1>
    <p class="mb-6 text-sm text-muted-foreground">Bring an expense ledger over from another service. Pick the source format below.</p>
    <ul class="flex list-none flex-col gap-1">
      <li v-for="s in sources" :key="s.to">
        <RouterLink
          :to="s.to"
          class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2 transition-colors hover:bg-hover-surface"
        >
          <span class="flex min-w-0 flex-col">
            <span class="font-medium">{{ s.title }}</span>
            <span class="text-xs text-muted-foreground">{{ s.desc }}</span>
          </span>
          <span class="flex-shrink-0 text-muted-foreground">→</span>
        </RouterLink>
      </li>
    </ul>
  </AppLayout>
</template>
