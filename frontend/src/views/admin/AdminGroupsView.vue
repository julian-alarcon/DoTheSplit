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
    <h1 class="mb-4 text-2xl font-semibold">All groups ({{ total }})</h1>

    <Alert v-if="okMsg" tone="success" class="mb-4">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="mb-4">{{ errMsg }}</Alert>

    <ul class="grid list-none gap-2">
      <li v-for="g in items" :key="g.id" class="rounded-md border border-border bg-card p-3">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="truncate font-medium">
              <RouterLink :to="`/groups/${g.id}`" class="text-inherit no-underline hover:underline">{{ g.name }}</RouterLink>
            </div>
            <div class="text-xs text-muted-foreground">
              {{ g.member_count }} members · {{ g.expense_count }} expenses · {{ g.default_currency }} · created {{ fmtDate(g.created_at) }}
            </div>
          </div>
          <button type="button" class="btn-danger btn-sm ml-auto" @click="deleteTarget = g.id">
            <Icon name="trash" /><span>Delete</span>
          </button>
        </div>
      </li>
    </ul>
    <nav class="mt-4 flex gap-4 text-sm">
      <button v-if="offset > 0" type="button" class="cursor-pointer text-inherit underline" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
      <button v-if="offset + ADMIN_PAGE < total" type="button" class="cursor-pointer text-inherit underline" @click="offset += ADMIN_PAGE">Next →</button>
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
