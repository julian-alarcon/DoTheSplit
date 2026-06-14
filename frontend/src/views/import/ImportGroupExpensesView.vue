<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getGroup, type Group } from "@/composables/useGroups";
import {
  commitGroupExpenses,
  csvTooLarge,
  previewGroupExpenses,
  type GroupExpensesResponse,
} from "@/composables/useImport";
import { formatMoney } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";

const route = useRoute();
const router = useRouter();
const groupId = computed(() => String(route.params.id));

const group = ref<Group | null>(null);
const phase = ref<"pick" | "review">("pick");
const csv = ref("");
const preview = ref<GroupExpensesResponse | null>(null);
const pickError = ref<string | null>(null);
const importError = ref<string | null>(null);
const busy = ref(false);

const splitMode = computed(() => {
  const g = group.value;
  if (!g) return "";
  const isPair = g.members.length === 2;
  const hasDefault = (g.default_split?.length ?? 0) === 2;
  return isPair && hasDefault
    ? "Pinned percentage split"
    : `Equal split across all ${g.members.length} members`;
});

const mixedCurrencies = computed(() => (preview.value?.csv_currencies ?? []).length > 1);

async function onPick(e: Event) {
  pickError.value = null;
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (!/\.csv$/i.test(file.name)) {
    pickError.value = "File must have a .csv extension.";
    return;
  }
  const text = await file.text();
  if (!text.trim()) {
    pickError.value = "File is empty.";
    return;
  }
  if (csvTooLarge(text)) {
    pickError.value = "File exceeds the 256 KiB limit.";
    return;
  }
  busy.value = true;
  const res = await previewGroupExpenses(groupId.value, text);
  busy.value = false;
  if (!res.data) {
    pickError.value = "Could not parse the CSV. Check the format and try again.";
    return;
  }
  csv.value = text;
  preview.value = res.data;
  phase.value = "review";
}

async function onImport() {
  importError.value = null;
  busy.value = true;
  const res = await commitGroupExpenses(groupId.value, csv.value);
  busy.value = false;
  if (res.ok) await router.replace(`/groups/${groupId.value}`);
  else importError.value = "Import failed. Try again or check the file.";
}

function back() {
  phase.value = "pick";
  csv.value = "";
  preview.value = null;
}

function fmtDate(s: string) {
  return new Date(s).toISOString().slice(0, 10);
}

onMounted(async () => {
  const { group: g } = await getGroup(groupId.value);
  if (!g) {
    await router.replace("/groups");
    return;
  }
  group.value = g;
});
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}/settings`, label: 'Settings' }">
    <h1 class="title">Import expenses</h1>
    <p class="lead">
      Append expenses to <span class="strong">{{ group.name }}</span> from a DoTheSplit-shaped CSV.
      Splits will use this group's rule: <span class="strong">{{ splitMode }}</span>.
    </p>
    <p class="lead">
      Required per row: <code>Date</code>, <code>Description</code>, <code>Cost</code>. Empty cells
      fall back to the group currency ({{ group.default_currency }}), the importer as payer, and the
      <code>other</code> category. The optional <code>Payer</code> column matches a member's display
      name; an unknown name causes the row to be skipped.
    </p>

    <Alert v-if="importError" tone="error" class="banner">{{ importError }}</Alert>

    <section v-if="phase === 'pick'" class="panel">
      <label class="field">
        <input type="file" accept=".csv,text/csv" required class="field-input file" @change="onPick" />
        <span class="field-label" data-required>CSV file</span>
      </label>
      <p v-if="pickError" class="err" role="alert">{{ pickError }}</p>
      <p v-if="busy" class="muted">Parsing…</p>
    </section>

    <section v-else-if="preview" class="review">
      <Alert v-if="mixedCurrencies" tone="info">
        This CSV mixes multiple currencies. DoTheSplit groups are single-currency; imported amounts
        are stored under {{ group.default_currency }} regardless. Detected:
        <span class="mono">{{ preview.csv_currencies.join(", ") }}</span>.
      </Alert>

      <div class="box">
        <div class="box-head">
          <span class="strong">Expenses preview</span>
          <span class="muted small">
            {{ preview.expense_count }} expense{{ preview.expense_count === 1 ? "" : "s" }}
            <template v-if="preview.skipped_count > 0"> · {{ preview.skipped_count }} skipped</template>
          </span>
        </div>
        <ul class="rows">
          <li v-for="(r, i) in preview.preview" :key="i" class="prow">
            <div class="prow-main">
              <span class="prow-desc">{{ r.description }}</span>
              <span class="prow-meta">{{ fmtDate(r.incurred_at) }} · {{ r.payer_display_name }} · {{ r.category_slug }}</span>
            </div>
            <span class="prow-amt">{{ formatMoney(r.amount_cents, r.currency) }}</span>
          </li>
        </ul>
        <p v-if="preview.expense_count > preview.preview.length" class="muted small mt">
          …and {{ preview.expense_count - preview.preview.length }} more.
        </p>
      </div>

      <details v-if="preview.skipped_count > 0" class="box">
        <summary class="strong">Skipped rows <span class="muted small">({{ preview.skipped_count }})</span></summary>
        <p class="muted small mt">Rows the importer dropped (bad date, missing cost, unknown payer name, etc.):</p>
        <pre class="skipped">{{ preview.skipped.join("\n") }}</pre>
      </details>

      <div class="actions">
        <button type="button" class="btn-secondary btn-sm" @click="back">Pick another file</button>
        <button type="button" class="btn-primary" :disabled="busy" @click="onImport">Import</button>
      </div>
    </section>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 0.5rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.strong {
  font-weight: 500;
  color: var(--foreground);
}
.banner {
  margin-bottom: 1rem;
}
.panel {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.file::file-selector-button {
  margin-right: 0.75rem;
  border: 0;
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
}
.err {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: var(--destructive);
}
.muted {
  color: var(--muted-foreground);
}
.small {
  font-size: 0.75rem;
}
.mt {
  margin-top: 0.5rem;
}
.mono {
  font-family: var(--font-mono);
}
.review {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.box {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  padding: 0.75rem;
  font-size: 0.875rem;
}
.box-head {
  margin-bottom: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}
.prow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.5rem 0.75rem;
}
.prow-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.prow-desc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.prow-meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.prow-amt {
  flex-shrink: 0;
  font-family: var(--font-mono);
}
.skipped {
  margin-top: 0.5rem;
  max-height: 16rem;
  overflow: auto;
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.5rem;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  line-height: 1.5;
}
.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
</style>
