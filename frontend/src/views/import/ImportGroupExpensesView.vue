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
    <h1 class="mb-2 text-2xl font-semibold">Import expenses</h1>
    <p class="mb-3 text-sm text-muted-foreground">
      Append expenses to <span class="font-medium text-foreground">{{ group.name }}</span> from a DoTheSplit-shaped CSV.
      Splits will use this group's rule: <span class="font-medium text-foreground">{{ splitMode }}</span>.
    </p>
    <p class="mb-3 text-sm text-muted-foreground">
      Required per row: <code>Date</code>, <code>Description</code>, <code>Cost</code>. Empty cells
      fall back to the group currency ({{ group.default_currency }}), the importer as payer, and the
      <code>other</code> category. The optional <code>Payer</code> column matches a member's display
      name; an unknown name causes the row to be skipped.
    </p>

    <Alert v-if="importError" tone="error" class="mb-4">{{ importError }}</Alert>

    <section v-if="phase === 'pick'" class="rounded-md border border-border bg-card p-3">
      <label class="field">
        <input type="file" accept=".csv,text/csv" required class="field-input field-file" @change="onPick" />
        <span class="field-label" data-required>CSV file</span>
      </label>
      <p v-if="pickError" class="mt-2 text-sm text-[var(--destructive)]" role="alert">{{ pickError }}</p>
      <p v-if="busy" class="text-muted-foreground">Parsing…</p>
    </section>

    <section v-else-if="preview" class="flex flex-col gap-4">
      <Alert v-if="mixedCurrencies" tone="info">
        This CSV mixes multiple currencies. DoTheSplit groups are single-currency; imported amounts
        are stored under {{ group.default_currency }} regardless. Detected:
        <span class="[font-family:var(--font-mono)]">{{ preview.csv_currencies.join(", ") }}</span>.
      </Alert>

      <div class="rounded-md border border-border p-3 text-sm">
        <div class="mb-2 flex items-center justify-between">
          <span class="font-medium text-foreground">Expenses preview</span>
          <span class="text-xs text-muted-foreground">
            {{ preview.expense_count }} expense{{ preview.expense_count === 1 ? "" : "s" }}
            <template v-if="preview.skipped_count > 0"> · {{ preview.skipped_count }} skipped</template>
          </span>
        </div>
        <ul class="flex list-none flex-col gap-2">
          <li v-for="(r, i) in preview.preview" :key="i" class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2">
            <div class="flex min-w-0 flex-col">
              <span class="truncate font-medium">{{ r.description }}</span>
              <span class="truncate text-xs text-muted-foreground">{{ fmtDate(r.incurred_at) }} · {{ r.payer_display_name }} · {{ r.category_slug }}</span>
            </div>
            <span class="flex-shrink-0 [font-family:var(--font-mono)]">{{ formatMoney(r.amount_cents, r.currency) }}</span>
          </li>
        </ul>
        <p v-if="preview.expense_count > preview.preview.length" class="mt-2 text-xs text-muted-foreground">
          …and {{ preview.expense_count - preview.preview.length }} more.
        </p>
      </div>

      <details v-if="preview.skipped_count > 0" class="rounded-md border border-border p-3 text-sm">
        <summary class="font-medium text-foreground">Skipped rows <span class="text-xs text-muted-foreground">({{ preview.skipped_count }})</span></summary>
        <p class="mt-2 text-xs text-muted-foreground">Rows the importer dropped (bad date, missing cost, unknown payer name, etc.):</p>
        <pre class="mt-2 max-h-64 overflow-auto rounded-sm bg-muted p-2 text-xs leading-normal [font-family:var(--font-mono)]">{{ preview.skipped.join("\n") }}</pre>
      </details>

      <div class="flex items-center justify-between gap-3">
        <button type="button" class="btn-secondary btn-sm" @click="back">Pick another file</button>
        <button type="button" class="btn-primary" :disabled="busy" @click="onImport">Import</button>
      </div>
    </section>
  </AppLayout>
</template>
