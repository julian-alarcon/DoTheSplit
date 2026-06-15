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
  previewFullImport,
  type SplitwiseResponse,
} from "@/composables/useImport";
import { formatMoney } from "@/lib/currencies";
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
  // header. Send the right count of placeholders by parsing the header here.
  const headerNames = text.split(/\r?\n/, 1)[0]?.split(",").slice(5).map((s) => s.trim()) ?? [];
  const res = await previewFullImport(props.source, {
    csv: text,
    group_name: "Imported group",
    default_currency: cur,
    members: placeholders(headerNames.length >= 2 ? headerNames : ["_0", "_1"]),
    dry_run: true,
  });
  busy.value = false;
  if (!res.data) {
    pickError.value = "Could not parse the CSV. Check the format and try again.";
    return;
  }
  csv.value = text;
  preview.value = res.data;
  defaultCurrency.value = res.data.default_currency || cur;
  groupName.value = res.data.group_name && res.data.group_name !== "Imported group" ? res.data.group_name : "";
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
  if (res.data) preview.value = res.data;
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
  else importError.value = "The import failed. Check the file and member emails, then try again.";
}

function back() {
  phase.value = "pick";
  csv.value = "";
  preview.value = null;
}

function fmtDate(s: string) {
  return new Date(s).toISOString().slice(0, 10);
}
</script>

<template>
  <div>
    <section v-if="phase === 'pick'" class="panel">
      <label class="field">
        <input type="file" accept=".csv,text/csv" required class="field-input file" @change="onPick" />
        <span class="field-label" data-required>CSV file</span>
      </label>
      <p v-if="pickError" class="err" role="alert">{{ pickError }}</p>
      <p v-if="busy" class="muted">Parsing…</p>
    </section>

    <section v-else-if="preview" class="review">
      <label class="field">
        <input v-model="groupName" required maxlength="80" class="field-input" placeholder=" " />
        <span class="field-label" data-required>Group name</span>
      </label>

      <label class="field-select-row">
        <span>Default currency</span>
        <CurrencySelect v-model="defaultCurrency" @change="refreshPreview" />
      </label>

      <p v-if="mixedCurrencies" class="warn">
        This CSV mixes multiple currencies. DoTheSplit groups use a single currency; amounts are kept
        as-is but stored under the chosen one. Detected:
        <span class="mono">{{ preview.csv_currencies.join(", ") }}</span>.
      </p>

      <fieldset class="members">
        <legend class="members-legend">Members</legend>
        <p class="muted small">
          Map each CSV name to the email of a registered DoTheSplit user. Unknown emails work too:
          expenses are kept against a placeholder account the real owner can claim later.
        </p>
        <div v-for="name in memberNames" :key="name" class="member-row">
          <span class="member-name">{{ name }}</span>
          <input
            v-model="memberEmails[name]"
            type="email"
            class="field-input member-email"
            placeholder="email@example.com"
            @blur="refreshPreview"
          />
        </div>
      </fieldset>

      <div class="box">
        <div class="box-head"><span class="strong">Balances preview</span></div>
        <ul class="bal-list">
          <li v-for="name in memberNames" :key="name" class="bal-row">
            <span class="trunc">{{ name }}</span>
            <span class="mono" :class="(balanceByName.get(name) ?? 0) >= 0 ? 'pos' : 'neg'">
              {{ formatMoney(balanceByName.get(name) ?? 0, defaultCurrency) }}
            </span>
          </li>
        </ul>
      </div>

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
              <span class="prow-meta">{{ fmtDate(r.incurred_at) }} · {{ r.payer_csv_name }} · {{ r.category_slug }}</span>
            </div>
            <span class="prow-amt">{{ formatMoney(r.amount_cents, r.currency) }}</span>
          </li>
        </ul>
        <p v-if="preview.expense_count > preview.preview.length" class="muted small mt">
          …and {{ preview.expense_count - preview.preview.length }} more.
        </p>
      </div>

      <div v-if="preview.settlement_count > 0" class="box">
        <div class="box-head">
          <span class="strong">Settlements preview</span>
          <span class="muted small">{{ preview.settlement_count }}</span>
        </div>
        <ul class="rows">
          <li v-for="(s, i) in preview.settlement_preview" :key="i" class="prow">
            <div class="prow-main">
              <span class="prow-desc">{{ s.from_csv_name }} → {{ s.to_csv_name }}</span>
              <span class="prow-meta">{{ fmtDate(s.settled_at) }}<template v-if="s.note"> · {{ s.note }}</template></span>
            </div>
            <span class="prow-amt">{{ formatMoney(s.amount_cents, s.currency) }}</span>
          </li>
        </ul>
      </div>

      <details v-if="preview.skipped_count > 0" class="box">
        <summary class="strong">Skipped rows <span class="muted small">({{ preview.skipped_count }})</span></summary>
        <pre class="skipped">{{ preview.skipped.join("\n") }}</pre>
      </details>

      <p v-if="importError" class="err" role="alert">{{ importError }}</p>

      <div class="actions">
        <button type="button" class="btn-secondary btn-sm" @click="back">Pick another file</button>
        <button type="button" class="btn-primary" :disabled="busy" @click="onImport">Import</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
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
.strong {
  font-weight: 500;
}
.review {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.warn {
  border-radius: 0.375rem;
  border: 1px solid color-mix(in oklch, oklch(0.7 0.15 80) 50%, var(--border));
  background: color-mix(in oklch, oklch(0.7 0.15 80) 12%, var(--card));
  padding: 0.75rem;
  font-size: 0.875rem;
}
.members {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  padding: 0.75rem;
}
.members-legend {
  padding: 0 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
}
.member-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.member-name {
  width: 8rem;
  flex-shrink: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.875rem;
}
.member-email {
  flex: 1;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
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
.bal-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  list-style: none;
}
.bal-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.trunc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pos {
  color: var(--primary);
}
.neg {
  color: var(--destructive);
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
