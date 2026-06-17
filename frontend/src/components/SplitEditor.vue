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
  <div class="flex flex-col gap-2">
    <button
      type="button"
      class="btn-secondary w-full justify-between gap-3 px-3 text-left font-normal whitespace-normal"
      aria-label="Edit details"
      @click="open"
    >
      <span class="min-w-0 flex-1 text-muted-foreground">{{ summary }}</span>
      <span class="flex shrink-0 items-center gap-1.5 text-muted-foreground">
        <span class="hidden sm:inline">Details</span>
        <Icon name="chevron-right" />
      </span>
    </button>

    <dialog
      ref="dialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-md rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Details"
    >
      <div class="flex flex-col gap-4 p-5">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-medium">Details</h3>
          <button type="button" class="cursor-pointer rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-muted" aria-label="Close" title="Close" @click="cancel">
            <Icon name="xmark" :size="14" />
          </button>
        </div>

        <fieldset class="flex gap-4 border-0 text-sm">
          <legend class="sr-only">Split mode</legend>
          <label class="flex cursor-pointer items-center gap-1.5">
            <input
              type="radio"
              name="split_mode"
              value="equal"
              class="accent-primary"
              :checked="mode === 'equal'"
              @change="onModeChange('equal')"
            />
            <span>Equal</span>
          </label>
          <label class="flex cursor-pointer items-center gap-1.5">
            <input
              type="radio"
              name="split_mode"
              value="exact"
              class="accent-primary"
              :checked="mode === 'exact'"
              @change="onModeChange('exact')"
            />
            <span>Exact amounts</span>
          </label>
          <label class="flex cursor-pointer items-center gap-1.5">
            <input
              type="radio"
              name="split_mode"
              value="percent"
              class="accent-primary"
              :checked="mode === 'percent'"
              @change="onModeChange('percent')"
            />
            <span>Percentage</span>
          </label>
        </fieldset>

        <ul class="flex max-h-80 list-none flex-col gap-1 overflow-auto">
          <li v-for="s in state" :key="s.userID" class="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted">
            <input
              type="checkbox"
              class="shrink-0 accent-primary"
              :checked="s.included"
              :aria-label="`Include ${memberByID.get(s.userID)?.display_name ?? s.userID}`"
              @change="onToggle(s, ($event.target as HTMLInputElement).checked)"
            />
            <span class="min-w-0 flex-1 truncate text-sm">{{ twoPersonLabel(s.userID) }}</span>
            <input
              type="number"
              step="0.01"
              min="0"
              class="field-input-num w-20 shrink-0"
              :class="{ invisible: mode === 'equal' }"
              :disabled="!s.included || mode === 'equal'"
              :value="rowInputValue(s)"
              @input="onValueInput(s, ($event.target as HTMLInputElement).value)"
            />
            <span class="w-16 shrink-0 text-right text-xs text-muted-foreground [font-family:var(--font-mono)]">{{ rowPreview(s) }}</span>
          </li>
        </ul>

        <div class="flex min-h-[1.25em] items-center justify-between gap-3 border-t border-border pt-3 text-sm">
          <span class="text-muted-foreground">{{ remainingText }}</span>
          <span class="[font-family:var(--font-mono)]">{{ totalText }}</span>
        </div>

        <p class="min-h-[1.25em] text-xs text-destructive">{{ errorText }}</p>

        <div class="flex justify-end gap-2">
          <button type="button" class="btn-secondary btn-sm" @click="cancel">Cancel</button>
          <button type="button" class="btn-primary btn-sm" :disabled="!valid" @click="done">Done</button>
        </div>
      </div>
    </dialog>
  </div>
</template>
