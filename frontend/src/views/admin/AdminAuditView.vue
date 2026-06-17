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
    <h1 class="mb-4 text-2xl font-semibold">Audit log ({{ total }})</h1>

    <form class="mb-4 flex flex-wrap items-center gap-2" @submit.prevent="onFilter">
      <label class="text-sm">Action</label>
      <input v-model="actionInput" placeholder="e.g. admin_delete_user" class="rounded-md border border-border bg-card px-2 py-1 text-sm" />
      <button type="submit" class="btn-secondary btn-sm">Filter</button>
    </form>

    <ul class="grid list-none gap-2">
      <li v-for="e in items" :key="e.id" class="rounded-md border p-3 text-sm" :class="e.success ? 'border-border bg-card' : 'border-[color-mix(in_oklch,var(--destructive)_50%,var(--border))] bg-[color-mix(in_oklch,var(--destructive)_8%,var(--card))]'">
        <div class="flex flex-wrap items-center gap-2">
          <code class="rounded-sm bg-muted px-1.5 py-px text-xs [font-family:var(--font-mono)]">{{ e.action }}</code>
          <span class="text-xs text-muted-foreground">{{ fmtTs(e.created_at) }}</span>
          <span v-if="!e.success" class="text-xs font-medium text-destructive">FAILED</span>
        </div>
        <div class="mt-1 text-xs text-muted-foreground [&_code]:[font-family:var(--font-mono)]">
          actor <code>{{ e.actor_user_id.slice(0, 8) }}</code>
          <template v-if="e.target_user_id"> · target user <code>{{ e.target_user_id.slice(0, 8) }}</code></template>
          <template v-if="e.target_group_id"> · target group <code>{{ e.target_group_id.slice(0, 8) }}</code></template>
          <template v-if="e.ip"> · ip {{ e.ip }}</template>
        </div>
        <pre v-if="e.metadata" class="mt-2 overflow-x-auto rounded-sm bg-muted p-2 text-xs [font-family:var(--font-mono)]">{{ JSON.stringify(e.metadata, null, 2) }}</pre>
      </li>
    </ul>
    <nav class="mt-4 flex gap-4 text-sm">
      <button v-if="offset > 0" type="button" class="cursor-pointer text-inherit underline" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
      <button v-if="offset + ADMIN_PAGE < total" type="button" class="cursor-pointer text-inherit underline" @click="offset += ADMIN_PAGE">Next →</button>
    </nav>
  </AppLayout>
</template>
