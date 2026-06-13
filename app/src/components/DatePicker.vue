<script setup lang="ts">
// Date picker. Replaces <input type="date"> (whose native popup sizes
// inconsistently across browsers). A calendar-icon trigger opens a <dialog>
// with a custom month grid; the chosen day is committed only on "Done".
//
// v-model holds the selected YYYY-MM-DD. An optional cadence v-model adds a
// "Repeat" dropdown (recurring expenses). Logic ported from the Astro tier's
// date-picker.ts: roving-tabindex keyboard nav, noon-UTC anchoring so the
// displayed weekday matches what the server stores, no backdrop-close.
import { computed, ref, watch } from "vue";
import Icon from "@/components/Icon.vue";

const props = withDefaults(
  defineProps<{
    variant?: "default" | "compact";
    caption?: string;
    ariaLabel?: string;
    /** When true, render the Repeat dropdown bound to the cadence model. */
    cadence?: boolean;
    /** 0 = Sunday-first, 1 = Monday-first. */
    weekStart?: 0 | 1;
  }>(),
  { variant: "default", ariaLabel: "Pick date", cadence: false, weekStart: 1 },
);

const value = defineModel<string>({ required: true });
const cadenceValue = defineModel<string>("cadence", { default: "" });

const labelFmt = new Intl.DateTimeFormat(undefined, {
  weekday: "short",
  year: "numeric",
  month: "short",
  day: "numeric",
});
const longDateFmt = new Intl.DateTimeFormat(undefined, {
  weekday: "long",
  year: "numeric",
  month: "long",
  day: "numeric",
});
const monthTitleFmt = new Intl.DateTimeFormat(undefined, {
  month: "long",
  year: "numeric",
});
const weekdayFmt = new Intl.DateTimeFormat(undefined, { weekday: "short" });

const cadenceLabels: Record<string, string> = {
  daily: "Repeats daily",
  weekly: "Repeats weekly",
  biweekly: "Repeats every 2 weeks",
  monthly: "Repeats monthly",
  yearly: "Repeats yearly",
};
const cadenceOptions = [
  { value: "", label: "Never" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "biweekly", label: "Every 2 weeks" },
  { value: "monthly", label: "Monthly" },
  { value: "yearly", label: "Yearly" },
];

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}
function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}
function isoFromYMD(y: number, m: number, d: number): string {
  return `${y}-${pad2(m + 1)}-${pad2(d)}`;
}
function parseISO(iso: string): { y: number; m: number; d: number } {
  const [ys, ms, ds] = iso.split("-");
  return { y: Number(ys), m: Number(ms) - 1, d: Number(ds) };
}
function addDays(iso: string, days: number): string {
  const { y, m, d } = parseISO(iso);
  const dt = new Date(Date.UTC(y, m, d));
  dt.setUTCDate(dt.getUTCDate() + days);
  return dt.toISOString().slice(0, 10);
}
function formatLabel(iso: string): string {
  const d = new Date(`${iso}T12:00:00Z`);
  return Number.isNaN(d.getTime()) ? iso : labelFmt.format(d);
}
function formatLongDate(iso: string): string {
  const d = new Date(`${iso}T12:00:00Z`);
  return Number.isNaN(d.getTime()) ? iso : longDateFmt.format(d);
}

const dialog = ref<HTMLDialogElement | null>(null);
const gridEl = ref<HTMLElement | null>(null);

// pending = what the user is looking at in the modal but hasn't committed;
// it diverges from `value` between open and Done/Cancel.
const pending = ref(value.value || todayISO());
const pendingCadence = ref(cadenceValue.value || "");
const viewYear = ref(parseISO(pending.value).y);
const viewMonth = ref(parseISO(pending.value).m);

const triggerDay = computed(() => Number(value.value.slice(8, 10)) || 1);
const triggerLabel = computed(() => formatLabel(value.value || todayISO()));
const cadenceBadge = computed(() => cadenceLabels[cadenceValue.value] ?? "");

const weekdayHeaders = computed(() => {
  const out: string[] = [];
  for (let i = 0; i < 7; i++) {
    // Anchor: Sunday 2024-01-07 (UTC). Add weekStart to start at Mon.
    const d = new Date(Date.UTC(2024, 0, 7 + ((i + props.weekStart) % 7)));
    out.push(weekdayFmt.format(d).slice(0, 2));
  }
  return out;
});

const calTitle = computed(() =>
  monthTitleFmt.format(new Date(viewYear.value, viewMonth.value, 1)),
);

type Cell = { kind: "blank" } | { kind: "day"; d: number; iso: string };

const cells = computed<Cell[]>(() => {
  const y = viewYear.value;
  const m = viewMonth.value;
  const firstWeekday = new Date(y, m, 1).getDay(); // 0 = Sun
  const leading = (firstWeekday - props.weekStart + 7) % 7;
  const daysInMonth = new Date(y, m + 1, 0).getDate();
  const out: Cell[] = [];
  for (let i = 0; i < leading; i++) out.push({ kind: "blank" });
  for (let d = 1; d <= daysInMonth; d++)
    out.push({ kind: "day", d, iso: isoFromYMD(y, m, d) });
  // Pad to a fixed 6x7 grid so the modal height doesn't jump.
  const trailing = 6 * 7 - leading - daysInMonth;
  for (let i = 0; i < trailing; i++) out.push({ kind: "blank" });
  return out;
});

const today = todayISO();

function focusPending() {
  // Defer until the grid re-renders with the new pending highlight.
  requestAnimationFrame(() => {
    gridEl.value
      ?.querySelector<HTMLButtonElement>(`[data-iso="${pending.value}"]`)
      ?.focus();
  });
}

function open() {
  pending.value = value.value || todayISO();
  pendingCadence.value = cadenceValue.value || "";
  const p = parseISO(pending.value);
  viewYear.value = p.y;
  viewMonth.value = p.m;
  dialog.value?.showModal();
  focusPending();
}

function pickDay(iso: string) {
  pending.value = iso;
  focusPending();
}

function goToday() {
  const t = todayISO();
  pending.value = t;
  const p = parseISO(t);
  viewYear.value = p.y;
  viewMonth.value = p.m;
  focusPending();
}

function prevMonth() {
  if (viewMonth.value === 0) {
    viewMonth.value = 11;
    viewYear.value--;
  } else {
    viewMonth.value--;
  }
}
function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0;
    viewYear.value++;
  } else {
    viewMonth.value++;
  }
}

function movePending(deltaDays: number) {
  pending.value = addDays(pending.value, deltaDays);
  const p = parseISO(pending.value);
  if (p.y !== viewYear.value || p.m !== viewMonth.value) {
    viewYear.value = p.y;
    viewMonth.value = p.m;
  }
  focusPending();
}

function onGridKeydown(e: KeyboardEvent) {
  const target = e.target;
  if (!(target instanceof HTMLButtonElement) || !target.dataset.iso) return;
  let handled = true;
  switch (e.key) {
    case "ArrowLeft":
      movePending(-1);
      break;
    case "ArrowRight":
      movePending(1);
      break;
    case "ArrowUp":
      movePending(-7);
      break;
    case "ArrowDown":
      movePending(7);
      break;
    case "PageUp":
      movePending(e.shiftKey ? -365 : -28);
      break;
    case "PageDown":
      movePending(e.shiftKey ? 365 : 28);
      break;
    case "Home": {
      const p = parseISO(pending.value);
      pending.value = isoFromYMD(p.y, p.m, 1);
      focusPending();
      break;
    }
    case "End": {
      const p = parseISO(pending.value);
      const last = new Date(p.y, p.m + 1, 0).getDate();
      pending.value = isoFromYMD(p.y, p.m, last);
      focusPending();
      break;
    }
    case "Enter":
      commit();
      break;
    default:
      handled = false;
  }
  if (handled) e.preventDefault();
}

function commit() {
  value.value = pending.value;
  if (props.cadence) cadenceValue.value = pendingCadence.value;
  dialog.value?.close();
}
function cancel() {
  dialog.value?.close();
}

// Keep the visible month in sync if the bound value changes externally.
watch(value, (v) => {
  if (!v) return;
  const p = parseISO(v);
  if (Number.isFinite(p.y)) {
    pending.value = v;
  }
});
</script>

<template>
  <div :class="['dp', variant === 'compact' ? 'dp-compact' : 'dp-default']">
    <template v-if="variant === 'compact'">
      <button
        type="button"
        class="btn-icon"
        :aria-label="ariaLabel"
        :title="ariaLabel"
        @click="open"
      >
        <span class="dp-icon" aria-hidden="true">
          <Icon name="layer-group" :size="28" class="dp-cal" />
          <span class="dp-day">{{ triggerDay }}</span>
        </span>
      </button>
      <span v-if="cadence && cadenceBadge" class="dp-cadence-badge">{{
        cadenceBadge
      }}</span>
    </template>

    <template v-else>
      <div class="dp-trigger-row">
        <button type="button" class="dp-trigger" @click="open">
          <span class="dp-icon" aria-hidden="true">
            <Icon name="layer-group" :size="28" class="dp-cal" />
            <span class="dp-day">{{ triggerDay }}</span>
          </span>
          <span class="dp-label">{{ triggerLabel }}</span>
        </button>
      </div>
      <span v-if="caption" class="dp-caption">{{ caption }}</span>
    </template>

    <dialog ref="dialog" class="dp-dialog" aria-modal="true" aria-label="Pick date">
      <div class="dp-body">
        <div class="dp-head">
          <h3 class="dp-title">Pick date</h3>
          <button
            type="button"
            class="dp-close"
            aria-label="Close"
            title="Close"
            @click="cancel"
          >
            <Icon name="xmark" :size="14" />
          </button>
        </div>

        <div class="dp-monthnav">
          <h4 class="dp-monthtitle">{{ calTitle }}</h4>
          <div class="dp-monthbtns">
            <button
              type="button"
              class="btn-icon"
              aria-label="Previous month"
              title="Previous month"
              @click="prevMonth"
            >
              <Icon name="chevron-left" />
            </button>
            <button
              type="button"
              class="btn-icon"
              aria-label="Next month"
              title="Next month"
              @click="nextMonth"
            >
              <Icon name="chevron-right" />
            </button>
          </div>
        </div>

        <div class="dp-weekdays">
          <div v-for="(w, i) in weekdayHeaders" :key="i">{{ w }}</div>
        </div>

        <div ref="gridEl" class="dp-grid" role="grid" @keydown="onGridKeydown">
          <template v-for="(cell, i) in cells" :key="i">
            <div v-if="cell.kind === 'blank'" class="dp-blank" aria-hidden="true" />
            <button
              v-else
              type="button"
              class="dp-cell"
              :class="{
                'dp-cell-pending': cell.iso === pending,
                'dp-cell-today': cell.iso === today && cell.iso !== pending,
              }"
              :data-iso="cell.iso"
              :aria-label="formatLongDate(cell.iso)"
              :aria-pressed="cell.iso === pending ? 'true' : undefined"
              :tabindex="cell.iso === pending ? 0 : -1"
              @click="pickDay(cell.iso)"
            >
              {{ cell.d }}
            </button>
          </template>
        </div>

        <div class="dp-foot">
          <div class="dp-foot-top">
            <button type="button" class="btn-secondary btn-sm" @click="goToday">
              Today
            </button>
            <label v-if="cadence" class="dp-repeat">
              <span class="dp-repeat-lbl">
                <Icon name="arrows-rotate" />
                <span>Repeat</span>
              </span>
              <select v-model="pendingCadence" class="field-select">
                <option v-for="o in cadenceOptions" :key="o.value" :value="o.value">
                  {{ o.label }}
                </option>
              </select>
            </label>
          </div>
          <div class="dp-foot-actions">
            <button type="button" class="btn-secondary btn-sm" @click="cancel">
              Cancel
            </button>
            <button type="button" class="btn-primary btn-sm" @click="commit">
              Done
            </button>
          </div>
        </div>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.dp-default {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.dp-compact {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.dp-trigger-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.5rem 0.75rem;
}
.dp-trigger {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-align: left;
  font-size: 0.875rem;
  cursor: pointer;
  border-radius: 0.375rem;
}
.dp-trigger:hover {
  opacity: 0.8;
}
.dp-trigger:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.dp-icon {
  position: relative;
  display: inline-block;
  height: 1.75rem;
  width: 1.75rem;
}
.dp-cal {
  position: absolute;
  inset: 0;
  color: var(--muted-foreground);
}
.dp-day {
  position: absolute;
  inset-inline: 0;
  top: 12px;
  display: flex;
  height: 14px;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
  color: var(--foreground);
}
.dp-label {
  font-weight: 500;
}
.dp-caption {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.dp-cadence-badge {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}

.dp-dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  width: calc(100% - 2rem);
  max-width: 24rem;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--popover);
  color: var(--popover-foreground);
  padding: 0;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.35);
}
.dp-dialog::backdrop {
  background: rgba(20, 20, 20, 0.4);
}
.dp-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}
.dp-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.dp-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.dp-close {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.dp-close:hover {
  background: var(--muted);
}
.dp-monthnav {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.dp-monthtitle {
  font-size: 0.875rem;
  font-weight: 500;
}
.dp-monthbtns {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.dp-weekdays {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 0.25rem;
  text-align: center;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.dp-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 0.25rem;
}
.dp-blank {
  height: 2.25rem;
}
.dp-cell {
  height: 2.25rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}
.dp-cell:hover {
  background: var(--muted);
}
.dp-cell:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.dp-cell-today {
  box-shadow: inset 0 0 0 1px var(--border);
}
.dp-cell-pending {
  background: var(--primary);
  color: var(--primary-foreground);
}
.dp-cell-pending:hover {
  background: var(--primary);
  filter: brightness(0.92);
}
.dp-foot {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
}
.dp-foot-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.dp-repeat {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.875rem;
}
.dp-repeat-lbl {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  color: var(--muted-foreground);
}
.dp-foot-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
