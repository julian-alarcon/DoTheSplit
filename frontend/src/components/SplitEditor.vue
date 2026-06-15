<script setup lang="ts">
// Split editor. Opens a <dialog> to choose a split mode (equal / exact /
// percent) across a subset of members, with a live per-member preview. The
// committed payload ({ mode, splits:[{user_id, value?}] }) is exposed via
// v-model so the parent posts it directly (same rounding, two-person labels,
// prefill behaviour).
//
// `amountCents` and `payerId` are reactive props from the parent form so the
// preview tracks the amount/payer live. For edit flows, pass `initialSplits`;
// an untouched editor leaves the payload null so the parent can omit splits
// (the backend then keeps/rescales the existing split).
import { computed, reactive, ref, watch } from "vue";
import { formatMoney } from "@/lib/currencies";
import Icon from "@/components/Icon.vue";

type Mode = "equal" | "exact" | "percent";

interface Member {
  user_id: string;
  display_name: string;
}
interface InitialSplit {
  user_id: string;
  share_cents: number;
}
interface DefaultSplitEntry {
  user_id: string;
  basis_points: number;
}
export interface SplitPayload {
  mode: Mode;
  splits: Array<{ user_id: string; value?: number }>;
}

const props = withDefaults(
  defineProps<{
    members: Member[];
    currency: string;
    amountCents: number;
    payerId?: string;
    currentUserId?: string;
    initialSplits?: InitialSplit[];
    defaultSplit?: DefaultSplitEntry[];
  }>(),
  { currency: "EUR", amountCents: 0 },
);

// The committed payload. null until first commit (matters for edit flows: a
// null payload tells the parent to omit splits_json).
const payload = defineModel<SplitPayload | null>({ default: null });

const initial = computed(() => props.initialSplits ?? []);
const groupDefault = computed(() => props.defaultSplit ?? []);
const hasInitial = computed(() => initial.value.length > 0);

function trimZeros(s: string): string {
  return s.includes(".") ? s.replace(/\.?0+$/, "") : s;
}
function formatPercent(bps: number): string {
  return `${trimZeros((bps / 100).toFixed(2))}%`;
}
function equalShares(totalCents: number, n: number): number[] {
  if (n <= 0 || totalCents < 0) return [];
  const base = Math.floor(totalCents / n);
  const rem = totalCents - base * n;
  return Array.from({ length: n }, (_, i) => base + (i < rem ? 1 : 0));
}

type RowState = { userID: string; included: boolean; value: number };
const mode = ref<Mode>("equal");
const state = reactive<RowState[]>([]);
const dirty = ref(false);

const memberByID = computed(() => new Map(props.members.map((m) => [m.user_id, m])));

function hasUsableDefault(): boolean {
  if (groupDefault.value.length !== 2 || props.members.length !== 2) return false;
  const ids = new Set(props.members.map((m) => m.user_id));
  return groupDefault.value.every((e) => ids.has(e.user_id));
}

function initState() {
  state.splice(0, state.length);
  if (initial.value.length === 0) {
    if (hasUsableDefault()) {
      const byID = new Map(groupDefault.value.map((e) => [e.user_id, e.basis_points]));
      for (const m of props.members)
        state.push({ userID: m.user_id, included: true, value: byID.get(m.user_id) ?? 0 });
      mode.value = "percent";
      return;
    }
    for (const m of props.members) state.push({ userID: m.user_id, included: true, value: 0 });
    mode.value = "equal";
    return;
  }
  const byID = new Map(initial.value.map((s) => [s.user_id, s.share_cents]));
  for (const m of props.members)
    state.push({ userID: m.user_id, included: byID.has(m.user_id), value: byID.get(m.user_id) ?? 0 });
  mode.value = "exact";
}

function prefillForMode() {
  const amount = props.amountCents;
  const included = state.filter((s) => s.included);
  if (mode.value === "equal") {
    for (const s of state) s.value = 0;
    return;
  }
  if (included.length === 0) return;
  if (mode.value === "exact") {
    const shares = equalShares(amount, included.length);
    let i = 0;
    for (const s of state) s.value = s.included ? (shares[i++] ?? 0) : 0;
    return;
  }
  const base = Math.floor(10000 / included.length);
  const rem = 10000 - base * included.length;
  let i = 0;
  for (const s of state) {
    if (!s.included) {
      s.value = 0;
      continue;
    }
    s.value = base + (i < rem ? 1 : 0);
    i++;
  }
}

function computedShares(amount: number): Map<string, number> {
  const out = new Map<string, number>();
  const included = state.filter((s) => s.included);
  if (included.length === 0) return out;
  if (mode.value === "equal") {
    const shares = equalShares(amount, included.length);
    let i = 0;
    for (const s of state) if (s.included) out.set(s.userID, shares[i++] ?? 0);
    return out;
  }
  if (mode.value === "exact") {
    for (const s of state) if (s.included) out.set(s.userID, s.value);
    return out;
  }
  let assigned = 0;
  for (const s of state) {
    if (!s.included) continue;
    const share = Math.floor((amount * s.value) / 10000);
    out.set(s.userID, share);
    assigned += share;
  }
  let i = 0;
  const includedList = state.filter((s) => s.included);
  while (assigned < amount && includedList.length > 0) {
    const target = includedList[i % includedList.length];
    out.set(target.userID, (out.get(target.userID) ?? 0) + 1);
    assigned++;
    i++;
  }
  return out;
}

function twoPersonLabel(userID: string): string {
  const m = memberByID.value.get(userID);
  const fallback = m?.display_name ?? userID;
  if (!props.currentUserId || state.length !== 2) return fallback;
  const payerID = props.payerId ?? "";
  if (userID === payerID) return `${fallback} paid`;
  if (payerID === props.currentUserId) return `${fallback} owes you`;
  if (userID === props.currentUserId) {
    const other = memberByID.value.get(payerID);
    return other ? `You owe ${other.display_name}` : fallback;
  }
  return fallback;
}

// Derived view model for the rows.
const shares = computed(() => computedShares(props.amountCents));
function rowPreview(s: RowState): string {
  return s.included ? formatMoney(shares.value.get(s.userID) ?? 0, props.currency) : "-";
}
function rowInputValue(s: RowState): string {
  if (mode.value === "equal" || !s.included) return "";
  if (mode.value === "exact") return (s.value / 100).toFixed(2);
  return trimZeros((s.value / 100).toFixed(2));
}

const totalText = computed(() => {
  const amount = props.amountCents;
  if (mode.value === "equal") return formatMoney(amount, props.currency);
  if (mode.value === "exact") {
    let sum = 0;
    for (const s of state) if (s.included) sum += s.value;
    return `${formatMoney(sum, props.currency)} / ${formatMoney(amount, props.currency)}`;
  }
  let bps = 0;
  for (const s of state) if (s.included) bps += s.value;
  return `${formatPercent(bps)} / 100%`;
});

const remainingText = computed(() => {
  const amount = props.amountCents;
  if (mode.value === "equal") return "";
  if (mode.value === "exact") {
    let sum = 0;
    for (const s of state) if (s.included) sum += s.value;
    const remaining = amount - sum;
    return remaining === 0 ? "" : `Remaining: ${formatMoney(remaining, props.currency)}`;
  }
  let bps = 0;
  for (const s of state) if (s.included) bps += s.value;
  const remaining = 10000 - bps;
  return remaining === 0 ? "" : `Remaining: ${formatPercent(remaining)}`;
});

const errorText = computed(() => {
  if (!state.some((s) => s.included)) return "Select at least one member.";
  if (props.amountCents <= 0) return "Enter an amount first.";
  return "";
});

const valid = computed(() => {
  const amount = props.amountCents;
  if (errorText.value) return false;
  if (mode.value === "equal") return amount > 0 && state.some((s) => s.included);
  if (mode.value === "exact") {
    let sum = 0;
    for (const s of state) if (s.included) sum += s.value;
    return sum === amount && amount > 0;
  }
  let bps = 0;
  for (const s of state) if (s.included) bps += s.value;
  return bps === 10000 && amount > 0;
});

const summary = computed(() => {
  if (mode.value === "equal")
    return `Split equally between ${state.filter((s) => s.included).length} member(s)`;
  const parts: string[] = [];
  for (const s of state) {
    if (!s.included) continue;
    const name = memberByID.value.get(s.userID)?.display_name ?? s.userID;
    parts.push(`${name}: ${formatMoney(shares.value.get(s.userID) ?? 0, props.currency)}`);
  }
  return parts.join(" · ");
});

function buildPayload(): SplitPayload {
  return {
    mode: mode.value,
    splits: state
      .filter((s) => s.included)
      .map((s) =>
        mode.value === "equal" ? { user_id: s.userID } : { user_id: s.userID, value: s.value },
      ),
  };
}

function commitPayload() {
  payload.value = buildPayload();
  dirty.value = true;
}

// Dialog wiring.
const dialog = ref<HTMLDialogElement | null>(null);
function open() {
  dialog.value?.showModal();
}
function done() {
  if (!valid.value) return;
  commitPayload();
  dialog.value?.close();
}
function cancel() {
  dialog.value?.close();
}

function onModeChange(next: Mode) {
  mode.value = next;
  prefillForMode();
}
function onToggle(s: RowState, checked: boolean) {
  s.included = checked;
  prefillForMode();
}
function onValueInput(s: RowState, raw: string) {
  const n = Number(raw);
  s.value = !Number.isFinite(n) || n < 0 ? 0 : Math.round(n * 100);
}

// Keep equal/percent previews in sync with a changing parent amount; exact
// preserves the user's typed cents.
watch(
  () => props.amountCents,
  () => {
    if (mode.value !== "exact") prefillForMode();
  },
);

// Initialize. For create flows, auto-commit so the parent always has a valid
// payload; for edit flows, leave payload null until the user touches it.
initState();
if (!hasInitial.value && !hasUsableDefault()) prefillForMode();
if (!hasInitial.value) {
  commitPayload();
  dirty.value = false;
}

// Expose so the parent's submit handler can decide whether to send splits.
defineExpose({ dirty, hasInitial });
</script>

<template>
  <div class="split">
    <button type="button" class="split-trigger" aria-label="Edit details" @click="open">
      <span class="split-summary">{{ summary }}</span>
      <span class="split-trigger-end">
        <span class="details-label">Details</span>
        <Icon name="chevron-right" />
      </span>
    </button>

    <dialog ref="dialog" class="split-dialog" aria-modal="true" aria-label="Details">
      <div class="split-body">
        <div class="split-head">
          <h3 class="split-title">Details</h3>
          <button type="button" class="split-close" aria-label="Close" title="Close" @click="cancel">
            <Icon name="xmark" :size="14" />
          </button>
        </div>

        <fieldset class="split-modes">
          <legend class="sr-only">Split mode</legend>
          <label class="split-mode-opt">
            <input
              type="radio"
              name="split_mode"
              value="equal"
              :checked="mode === 'equal'"
              @change="onModeChange('equal')"
            />
            <span>Equal</span>
          </label>
          <label class="split-mode-opt">
            <input
              type="radio"
              name="split_mode"
              value="exact"
              :checked="mode === 'exact'"
              @change="onModeChange('exact')"
            />
            <span>Exact amounts</span>
          </label>
          <label class="split-mode-opt">
            <input
              type="radio"
              name="split_mode"
              value="percent"
              :checked="mode === 'percent'"
              @change="onModeChange('percent')"
            />
            <span>Percentage</span>
          </label>
        </fieldset>

        <ul class="split-rows">
          <li v-for="s in state" :key="s.userID" class="split-row">
            <input
              type="checkbox"
              :checked="s.included"
              :aria-label="`Include ${memberByID.get(s.userID)?.display_name ?? s.userID}`"
              @change="onToggle(s, ($event.target as HTMLInputElement).checked)"
            />
            <span class="split-label">{{ twoPersonLabel(s.userID) }}</span>
            <input
              type="number"
              step="0.01"
              min="0"
              class="field-input-num split-value"
              :class="{ 'split-value-hidden': mode === 'equal' }"
              :disabled="!s.included || mode === 'equal'"
              :value="rowInputValue(s)"
              @input="onValueInput(s, ($event.target as HTMLInputElement).value)"
            />
            <span class="split-preview">{{ rowPreview(s) }}</span>
          </li>
        </ul>

        <div class="split-totals">
          <span class="split-remaining">{{ remainingText }}</span>
          <span class="split-total">{{ totalText }}</span>
        </div>

        <p class="split-err">{{ errorText }}</p>

        <div class="split-actions">
          <button type="button" class="btn-secondary btn-sm" @click="cancel">Cancel</button>
          <button type="button" class="btn-primary btn-sm" :disabled="!valid" @click="done">Done</button>
        </div>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.split {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.split-trigger {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.5rem 0.75rem;
  text-align: left;
  font-size: 0.875rem;
  cursor: pointer;
}
.split-trigger:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .split-trigger:hover,
:root[data-theme="high-contrast"] .split-trigger:hover {
  background: var(--accent);
}
.split-trigger:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.split-summary {
  min-width: 0;
  flex: 1;
  color: var(--muted-foreground);
}
.split-trigger-end {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.375rem;
  color: var(--muted-foreground);
}
.details-label {
  display: none;
}
@media (min-width: 640px) {
  .details-label {
    display: inline;
  }
}
.split-dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  width: calc(100% - 2rem);
  max-width: 28rem;
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  background: var(--popover);
  color: var(--popover-foreground);
  padding: 0;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.35);
}
.split-dialog::backdrop {
  background: var(--backdrop);
}
.split-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}
.split-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.split-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.split-close {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.split-close:hover {
  background: var(--muted);
}
.split-modes {
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
  border: 0;
}
.split-mode-opt {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  cursor: pointer;
}
.split-mode-opt input {
  accent-color: var(--primary);
}
.split-rows {
  display: flex;
  max-height: 20rem;
  flex-direction: column;
  gap: 0.25rem;
  overflow: auto;
  list-style: none;
}
.split-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  padding: 0.375rem 0.5rem;
}
.split-row:hover {
  background: var(--muted);
}
.split-row input[type="checkbox"] {
  flex-shrink: 0;
  accent-color: var(--primary);
}
.split-label {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.875rem;
}
.split-value {
  width: 5rem;
  flex-shrink: 0;
}
.split-value-hidden {
  visibility: hidden;
}
.split-preview {
  width: 4rem;
  flex-shrink: 0;
  text-align: right;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.split-totals {
  display: flex;
  min-height: 1.25em;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
  font-size: 0.875rem;
}
.split-remaining {
  color: var(--muted-foreground);
}
.split-total {
  font-family: var(--font-mono);
}
.split-err {
  min-height: 1.25em;
  font-size: 0.75rem;
  color: var(--destructive);
}
.split-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
