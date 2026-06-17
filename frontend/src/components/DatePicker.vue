<script setup lang="ts">
// Date picker. Replaces <input type="date"> (whose native popup sizes
// inconsistently across browsers). A calendar-icon trigger opens a <dialog>
// with a custom month grid; the chosen day is committed only on "Done".
//
// v-model holds the selected YYYY-MM-DD. An optional cadence v-model adds a
// "Repeat" dropdown (recurring expenses): roving-tabindex keyboard nav,
// noon-UTC anchoring so the displayed weekday matches what the server stores,
// no backdrop-close.
import { computed, ref, watch } from "vue";
import Icon from "@/components/Icon.vue";

const props = withDefaults(
  defineProps<{
    variant?: "default" | "compact";
    caption?: string;
    ariaLabel?: string;
    /** When true, render the Repeat dropdown bound to the cadence model. */
    showCadence?: boolean;
    /** 0 = Sunday-first, 1 = Monday-first. */
    weekStart?: 0 | 1;
  }>(),
  { variant: "default", ariaLabel: "Pick date", showCadence: false, weekStart: 1 },
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
  if (props.showCadence) cadenceValue.value = pendingCadence.value;
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
  <div :class="variant === 'compact' ? 'inline-flex items-center gap-2' : 'flex flex-col gap-1'">
    <template v-if="variant === 'compact'">
      <button
        type="button"
        class="btn-icon"
        :aria-label="ariaLabel"
        :title="ariaLabel"
        @click="open"
      >
        <span class="relative inline-block h-7 w-7" aria-hidden="true">
          <Icon name="layer-group" :size="28" class="absolute inset-0 text-muted-foreground" />
          <span class="absolute inset-x-0 top-[12px] flex h-[14px] items-center justify-center text-[10px] font-semibold leading-none text-foreground">{{ triggerDay }}</span>
        </span>
      </button>
      <span v-if="showCadence && cadenceBadge" class="text-xs text-muted-foreground">{{
        cadenceBadge
      }}</span>
    </template>

    <template v-else>
      <div class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2">
        <button type="button" class="flex cursor-pointer items-center gap-2 rounded-md text-left text-sm hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring" @click="open">
          <span class="relative inline-block h-7 w-7" aria-hidden="true">
            <Icon name="layer-group" :size="28" class="absolute inset-0 text-muted-foreground" />
            <span class="absolute inset-x-0 top-[12px] flex h-[14px] items-center justify-center text-[10px] font-semibold leading-none text-foreground">{{ triggerDay }}</span>
          </span>
          <span class="font-medium">{{ triggerLabel }}</span>
        </button>
      </div>
      <span v-if="caption" class="text-xs text-muted-foreground">{{ caption }}</span>
    </template>

    <dialog
      ref="dialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-lg border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Pick date"
    >
      <div class="flex flex-col gap-4 p-5">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-medium">Pick date</h3>
          <button
            type="button"
            class="cursor-pointer rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-muted"
            aria-label="Close"
            title="Close"
            @click="cancel"
          >
            <Icon name="xmark" :size="14" />
          </button>
        </div>

        <div class="flex items-center justify-between">
          <h4 class="text-sm font-medium">{{ calTitle }}</h4>
          <div class="flex items-center gap-2">
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

        <div class="grid grid-cols-7 gap-1 text-center text-xs text-muted-foreground">
          <div v-for="(w, i) in weekdayHeaders" :key="i">{{ w }}</div>
        </div>

        <div ref="gridEl" class="grid grid-cols-7 gap-1" role="grid" @keydown="onGridKeydown">
          <template v-for="(cell, i) in cells" :key="i">
            <div v-if="cell.kind === 'blank'" class="h-9" aria-hidden="true" />
            <button
              v-else
              type="button"
              class="h-9 cursor-pointer rounded-md text-sm tabular-nums hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              :class="{
                'bg-primary text-primary-foreground hover:brightness-[0.92]': cell.iso === pending,
                'shadow-[inset_0_0_0_1px_var(--border)]': cell.iso === today && cell.iso !== pending,
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

        <div class="flex flex-col gap-3 border-t border-border pt-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <button type="button" class="btn-secondary btn-sm" @click="goToday">
              Today
            </button>
            <label v-if="showCadence" class="ml-auto flex items-center gap-1.5 text-sm">
              <span class="flex items-center gap-1 text-muted-foreground">
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
          <div class="flex items-center justify-end gap-2">
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
