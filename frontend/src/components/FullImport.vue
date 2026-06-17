<script setup lang="ts">
// Shared two-phase importer for the full-group flows (Splitwise + DoTheSplit,
// which use byte-identical request/response shapes). Phase 1 picks a CSV and
// runs a dry-run preview with placeholder member emails to discover the member
// columns. Phase 2 maps each CSV name to a real email, re-previews live, and
// commits to create the new group. The Go service remains the source of truth.
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import {
  commitFullImport,
  csvTooLarge,
  firstCsvCurrency,
  groupNameFromFilename,
  memberNamesFromCsv,
  previewFullImport,
  type SplitwiseResponse,
} from "@/composables/useImport";
import { formatMoney } from "@/lib/currencies";
import Alert from "@/components/Alert.vue";
import CurrencySelect from "@/components/CurrencySelect.vue";

const props = defineProps<{
  source: "splitwise" | "dothesplit";
}>();

const router = useRouter();

const phase = ref<"pick" | "review">("pick");
const csv = ref("");
const pickError = ref<string | null>(null);
const importError = ref<string | null>(null);
const busy = ref(false);

const groupName = ref("");
const defaultCurrency = ref("EUR");
const memberEmails = ref<Record<string, string>>({});
const preview = ref<SplitwiseResponse | null>(null);

const mixedCurrencies = computed(() => (preview.value?.csv_currencies ?? []).length > 1);
const memberNames = computed(() => preview.value?.members.map((m) => m.csv_name) ?? []);
// Net balance keyed by csv_name for the projected-balances list.
const balanceByName = computed(
  () => new Map((preview.value?.balances ?? []).map((b) => [b.csv_name, b.net_cents])),
);

function placeholders(names: string[]) {
  return names.map((csv_name, i) => ({ csv_name, email: `preview-${i}@example.invalid` }));
}

function currentMembers() {
  return memberNames.value.map((csv_name) => ({
    csv_name,
    email: (memberEmails.value[csv_name] ?? "").trim(),
  }));
}

async function onPick(e: Event) {
  pickError.value = null;
  importError.value = null;
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
  // First preview: discover member columns with placeholder emails.
  const cur = firstCsvCurrency(text);
  busy.value = true;
  // We don't yet know the member names; the server derives them from the
  // header. Send the right count of placeholders by parsing the header here,
  // skipping the optional DoTheSplit metadata columns exactly as the server does.
  const headerNames = memberNamesFromCsv(text);
  const res = await previewFullImport(props.source, {
    csv: text,
    group_name: "Imported group",
    default_currency: cur,
    members: placeholders(headerNames.length >= 2 ? headerNames : ["_0", "_1"]),
    dry_run: true,
  });
  busy.value = false;
  if (!res.data) {
    pickError.value = res.message ?? "Could not parse the CSV. Check the format and try again.";
    return;
  }
  csv.value = text;
  preview.value = res.data;
  defaultCurrency.value = res.data.default_currency || cur;
  // The first preview is sent with a placeholder group_name, so the server
  // echoes that back rather than a real name; derive a default from the file
  // name instead (the user can still edit it before importing).
  groupName.value = groupNameFromFilename(file.name);
  memberEmails.value = {};
  phase.value = "review";
}

// Re-run the preview with the current mapping/currency so balances reflect
// real members. Called on demand (currency change, manual refresh).
async function refreshPreview() {
  if (!csv.value) return;
  busy.value = true;
  const res = await previewFullImport(props.source, {
    csv: csv.value,
    group_name: groupName.value || "Imported group",
    default_currency: defaultCurrency.value,
    members: currentMembers().map((m, i) => ({
      csv_name: m.csv_name,
      email: m.email || `preview-${i}@example.invalid`,
    })),
    dry_run: true,
  });
  busy.value = false;
  if (res.data) {
    preview.value = res.data;
    importError.value = null;
  } else {
    // A live re-preview can fail (e.g. a mistyped email rejected by the
    // server). Surface why so the user can fix it before committing.
    importError.value = res.message ?? null;
  }
}

async function onImport() {
  importError.value = null;
  const members = currentMembers();
  if (members.some((m) => !m.email)) {
    importError.value = "Map every member to an email before importing.";
    return;
  }
  busy.value = true;
  const res = await commitFullImport(props.source, {
    csv: csv.value,
    group_name: groupName.value,
    default_currency: defaultCurrency.value,
    members,
    dry_run: false,
  });
  busy.value = false;
  if (res.ok && res.groupId) await router.replace(`/groups/${res.groupId}`);
  else
    importError.value =
      res.message ?? "The import failed. Check the file and member emails, then try again.";
}

function back() {
  phase.value = "pick";
  csv.value = "";
  preview.value = null;
  pickError.value = null;
  importError.value = null;
}

function fmtDate(s: string) {
  return new Date(s).toISOString().slice(0, 10);
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <section v-if="phase === 'pick'" class="rounded-md border border-border bg-card p-3">
      <label class="field">
        <input type="file" accept=".csv,text/csv" required class="field-input field-file" @change="onPick" />
        <span class="field-label" data-required>CSV file</span>
      </label>
      <Alert v-if="pickError" tone="error" class="mt-2">{{ pickError }}</Alert>
      <p v-if="busy" class="text-muted-foreground">Parsing…</p>
    </section>

    <section v-else-if="preview" class="flex flex-col gap-4">
      <label class="field">
        <input v-model="groupName" required maxlength="80" class="field-input" placeholder=" " />
        <span class="field-label" data-required>Group name</span>
      </label>

      <label class="field-select-row">
        <span>Default currency</span>
        <CurrencySelect v-model="defaultCurrency" @change="refreshPreview" />
      </label>

      <p v-if="mixedCurrencies" class="rounded-md border border-[color-mix(in_oklch,oklch(0.7_0.15_80)_50%,var(--border))] bg-[color-mix(in_oklch,oklch(0.7_0.15_80)_12%,var(--card))] p-3 text-sm">
        This CSV mixes multiple currencies. DoTheSplit groups use a single currency; amounts are kept
        as-is but stored under the chosen one. Detected:
        <span class="[font-family:var(--font-mono)]">{{ preview.csv_currencies.join(", ") }}</span>.
      </p>

      <fieldset class="flex flex-col gap-3 rounded-md border border-border p-3">
        <legend class="px-1 text-sm font-medium">Members</legend>
        <p class="text-xs text-muted-foreground">
          Map each CSV name to the email of a registered DoTheSplit user. Unknown emails work too:
          expenses are kept against a placeholder account the real owner can claim later.
        </p>
        <div v-for="name in memberNames" :key="name" class="flex items-center gap-2">
          <span class="w-32 flex-shrink-0 truncate text-sm">{{ name }}</span>
          <input
            v-model="memberEmails[name]"
            type="email"
            class="field-input flex-1 rounded-md border border-border bg-card"
            placeholder="email@example.com"
            @blur="refreshPreview"
          />
        </div>
      </fieldset>

      <div class="rounded-md border border-border p-3 text-sm">
        <div class="mb-2 flex items-center justify-between"><span class="font-medium">Balances preview</span></div>
        <ul class="flex list-none flex-col gap-1">
          <li v-for="name in memberNames" :key="name" class="flex items-center justify-between gap-2">
            <span class="truncate">{{ name }}</span>
            <span class="[font-family:var(--font-mono)]" :class="(balanceByName.get(name) ?? 0) >= 0 ? 'text-primary' : 'text-destructive'">
              {{ formatMoney(balanceByName.get(name) ?? 0, defaultCurrency) }}
            </span>
          </li>
        </ul>
      </div>

      <div class="rounded-md border border-border p-3 text-sm">
        <div class="mb-2 flex items-center justify-between">
          <span class="font-medium">Expenses preview</span>
          <span class="text-xs text-muted-foreground">
            {{ preview.expense_count }} expense{{ preview.expense_count === 1 ? "" : "s" }}
            <template v-if="preview.skipped_count > 0"> · {{ preview.skipped_count }} skipped</template>
          </span>
        </div>
        <ul class="flex list-none flex-col gap-2">
          <li v-for="(r, i) in preview.preview" :key="i" class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2">
            <div class="flex min-w-0 flex-col">
              <span class="truncate font-medium">{{ r.description }}</span>
              <span class="truncate text-xs text-muted-foreground">{{ fmtDate(r.incurred_at) }} · {{ r.payer_csv_name }} · {{ r.category_slug }}</span>
            </div>
            <span class="flex-shrink-0 [font-family:var(--font-mono)]">{{ formatMoney(r.amount_cents, r.currency) }}</span>
          </li>
        </ul>
        <p v-if="preview.expense_count > preview.preview.length" class="mt-2 text-xs text-muted-foreground">
          …and {{ preview.expense_count - preview.preview.length }} more.
        </p>
      </div>

      <div v-if="preview.settlement_count > 0" class="rounded-md border border-border p-3 text-sm">
        <div class="mb-2 flex items-center justify-between">
          <span class="font-medium">Settlements preview</span>
          <span class="text-xs text-muted-foreground">{{ preview.settlement_count }}</span>
        </div>
        <ul class="flex list-none flex-col gap-2">
          <li v-for="(s, i) in preview.settlement_preview" :key="i" class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2">
            <div class="flex min-w-0 flex-col">
              <span class="truncate font-medium">{{ s.from_csv_name }} → {{ s.to_csv_name }}</span>
              <span class="truncate text-xs text-muted-foreground">{{ fmtDate(s.settled_at) }}<template v-if="s.note"> · {{ s.note }}</template></span>
            </div>
            <span class="flex-shrink-0 [font-family:var(--font-mono)]">{{ formatMoney(s.amount_cents, s.currency) }}</span>
          </li>
        </ul>
      </div>

      <details v-if="preview.skipped_count > 0" class="rounded-md border border-border p-3 text-sm">
        <summary class="font-medium">Skipped rows <span class="text-xs text-muted-foreground">({{ preview.skipped_count }})</span></summary>
        <pre class="mt-2 max-h-64 overflow-auto rounded-sm bg-muted p-2 text-xs leading-normal [font-family:var(--font-mono)]">{{ preview.skipped.join("\n") }}</pre>
      </details>

      <Alert v-if="importError" tone="error">{{ importError }}</Alert>

      <div class="flex items-center justify-between gap-3">
        <button type="button" class="btn-secondary btn-sm" @click="back">Pick another file</button>
        <button type="button" class="btn-primary" :disabled="busy" @click="onImport">Import</button>
      </div>
    </section>
  </div>
</template>
