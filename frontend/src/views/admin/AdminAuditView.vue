<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { ADMIN_PAGE, listAudit, type AdminAuditEntry } from "@/composables/useAdmin";
import AppLayout from "@/components/AppLayout.vue";

const offset = ref(0);
const actionInput = ref("");
const action = ref("");
const items = ref<AdminAuditEntry[]>([]);
const total = ref(0);

function fmtTs(s: string) {
  return new Date(s).toISOString().replace("T", " ").slice(0, 19) + "Z";
}

async function load() {
  const res = await listAudit(offset.value, action.value);
  items.value = res.items;
  total.value = res.total;
}

function onFilter() {
  action.value = actionInput.value.trim();
  offset.value = 0;
  load();
}

watch(offset, load);
onMounted(load);
</script>

<template>
  <AppLayout :back="{ to: '/admin', label: 'Admin' }">
    <h1 class="title">Audit log ({{ total }})</h1>

    <form class="filter" @submit.prevent="onFilter">
      <label class="filter-label">Action</label>
      <input v-model="actionInput" placeholder="e.g. admin_delete_user" class="filter-input" />
      <button type="submit" class="btn-secondary btn-sm">Filter</button>
    </form>

    <ul class="entries">
      <li v-for="e in items" :key="e.id" class="entry" :class="{ failed: !e.success }">
        <div class="entry-head">
          <code class="action">{{ e.action }}</code>
          <span class="ts">{{ fmtTs(e.created_at) }}</span>
          <span v-if="!e.success" class="failed-tag">FAILED</span>
        </div>
        <div class="entry-meta">
          actor <code>{{ e.actor_user_id.slice(0, 8) }}</code>
          <template v-if="e.target_user_id"> · target user <code>{{ e.target_user_id.slice(0, 8) }}</code></template>
          <template v-if="e.target_group_id"> · target group <code>{{ e.target_group_id.slice(0, 8) }}</code></template>
          <template v-if="e.ip"> · ip {{ e.ip }}</template>
        </div>
        <pre v-if="e.metadata" class="meta-json">{{ JSON.stringify(e.metadata, null, 2) }}</pre>
      </li>
    </ul>
    <nav class="pager">
      <button v-if="offset > 0" type="button" class="link" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
      <button v-if="offset + ADMIN_PAGE < total" type="button" class="link" @click="offset += ADMIN_PAGE">Next →</button>
    </nav>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 1rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.filter {
  margin-bottom: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.filter-label {
  font-size: 0.875rem;
}
.filter-input {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}
.entries {
  display: grid;
  gap: 0.5rem;
  list-style: none;
}
.entry {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
  font-size: 0.875rem;
}
.entry.failed {
  border-color: color-mix(in oklch, var(--destructive) 50%, var(--border));
  background: color-mix(in oklch, var(--destructive) 8%, var(--card));
}
.entry-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.action {
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.0625rem 0.375rem;
  font-size: 0.75rem;
  font-family: var(--font-mono);
}
.ts {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.failed-tag {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--destructive);
}
.entry-meta {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.entry-meta code {
  font-family: var(--font-mono);
}
.meta-json {
  margin-top: 0.5rem;
  overflow-x: auto;
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.5rem;
  font-size: 0.75rem;
  font-family: var(--font-mono);
}
.pager {
  margin-top: 1rem;
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
}
.link {
  cursor: pointer;
  text-decoration: underline;
  color: inherit;
}
</style>
