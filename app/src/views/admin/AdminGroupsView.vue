<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import { ADMIN_PAGE, deleteGroup, listGroups, type AdminGroup } from "@/composables/useAdmin";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import PasswordPromptDialog from "@/components/PasswordPromptDialog.vue";

const offset = ref(0);
const items = ref<AdminGroup[]>([]);
const total = ref(0);
const okMsg = ref<string | null>(null);
const errMsg = ref<string | null>(null);
const deleteTarget = ref<string | null>(null);

const deleteErr: Record<string, string> = {
  step_up: "Step-up password did not match.",
  locked: "Too many failed step-up attempts; wait a minute.",
  error: "Delete failed.",
};

function fmtDate(s: string) {
  return new Date(s).toISOString().slice(0, 10);
}

async function load() {
  const res = await listGroups(offset.value);
  items.value = res.items;
  total.value = res.total;
}

async function onDelete(password: string) {
  const id = deleteTarget.value;
  deleteTarget.value = null;
  if (!id) return;
  okMsg.value = null;
  errMsg.value = null;
  const res = await deleteGroup(id, password);
  if (res.ok) {
    okMsg.value = "Group deleted.";
    await load();
  } else {
    errMsg.value = deleteErr[res.code ?? "error"];
  }
}

watch(offset, load);
onMounted(load);
</script>

<template>
  <AppLayout :back="{ to: '/admin', label: 'Admin' }">
    <h1 class="title">All groups ({{ total }})</h1>

    <Alert v-if="okMsg" tone="success" class="banner">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="banner">{{ errMsg }}</Alert>

    <ul class="groups">
      <li v-for="g in items" :key="g.id" class="group">
        <div class="group-row">
          <div class="group-main">
            <div class="group-name">
              <RouterLink :to="`/groups/${g.id}`" class="link">{{ g.name }}</RouterLink>
            </div>
            <div class="group-meta">
              {{ g.member_count }} members · {{ g.expense_count }} expenses · {{ g.default_currency }} · created {{ fmtDate(g.created_at) }}
            </div>
          </div>
          <button type="button" class="btn-danger btn-sm" @click="deleteTarget = g.id">
            <Icon name="trash" /><span>Delete</span>
          </button>
        </div>
      </li>
    </ul>
    <nav class="pager">
      <button v-if="offset > 0" type="button" class="link" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
      <button v-if="offset + ADMIN_PAGE < total" type="button" class="link" @click="offset += ADMIN_PAGE">Next →</button>
    </nav>

    <PasswordPromptDialog
      :open="deleteTarget !== null"
      title="Delete this group?"
      message="This permanently removes the group along with every expense, settlement, and recurring template it contains. Members keep their accounts. Re-enter your password to confirm."
      confirm-label="Delete group"
      confirm-icon="trash"
      @update:open="(v) => { if (!v) deleteTarget = null; }"
      @confirm="onDelete"
    />
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 1rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.banner {
  margin-bottom: 1rem;
}
.groups {
  display: grid;
  gap: 0.5rem;
  list-style: none;
}
.group {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.group-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}
.group-main {
  min-width: 0;
}
.group-name {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.group-meta {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.pager {
  margin-top: 1rem;
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
}
.link {
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}
.link:hover {
  text-decoration: underline;
}
.pager .link {
  text-decoration: underline;
}
</style>
