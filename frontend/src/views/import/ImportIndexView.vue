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
    <h1 class="title">Import a group</h1>
    <p class="lead">Bring an expense ledger over from another service. Pick the source format below.</p>
    <ul class="list">
      <li v-for="s in sources" :key="s.to">
        <RouterLink :to="s.to" class="row">
          <span class="row-main">
            <span class="row-title">{{ s.title }}</span>
            <span class="row-desc">{{ s.desc }}</span>
          </span>
          <span class="row-arrow">→</span>
        </RouterLink>
      </li>
    </ul>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 0.5rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  list-style: none;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.5rem 0.75rem;
  transition: background-color 120ms ease;
}
.row:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .row:hover,
:root[data-theme="high-contrast"] .row:hover {
  background: var(--accent);
}
.row-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.row-title {
  font-weight: 500;
}
.row-desc {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.row-arrow {
  flex-shrink: 0;
  color: var(--muted-foreground);
}
</style>
