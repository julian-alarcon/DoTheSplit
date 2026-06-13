<script setup lang="ts">
// Currency-aware amount input. While focused it shows the raw editable number
// ("1234.5"); on blur it shows a fully localized formatted string ("1,234.50
// €"). v-model is the canonical amount in CENTS (integer), so parents post
// amount_cents directly. Ported from the Astro tier's currency-input.ts.
//
// Rendered inside a .field wrapper with a floating label, matching the
// add-expense form. Native constraint validation still applies (required).
import { onMounted, ref, useId, watch } from "vue";
import { moneyFormatter } from "@/lib/currencies";

const props = withDefaults(
  defineProps<{
    label: string;
    currency: string;
    required?: boolean;
    error?: string;
  }>(),
  { required: false },
);

// Model is integer cents (0 = empty/invalid).
const cents = defineModel<number>({ default: 0 });

const inputId = useId();
const visible = ref("");
const focused = ref(false);

function parse(raw: string): number | null {
  if (!raw) return null;
  const cleaned = raw.replace(/[^0-9.,-]/g, "");
  if (!cleaned) return null;
  const lastDot = cleaned.lastIndexOf(".");
  const lastComma = cleaned.lastIndexOf(",");
  let normalized = cleaned;
  if (lastDot !== -1 && lastComma !== -1) {
    normalized =
      lastComma > lastDot
        ? cleaned.replace(/\./g, "").replace(",", ".")
        : cleaned.replace(/,/g, "");
  } else if (lastComma !== -1) {
    normalized = cleaned.replace(/\./g, "").replace(",", ".");
  } else {
    normalized = cleaned.replace(/,/g, "");
  }
  const n = Number(normalized);
  return Number.isFinite(n) ? n : null;
}

function format(n: number): string {
  return moneyFormatter(props.currency).format(n);
}

function rawForEdit(n: number): string {
  return n.toFixed(2).replace(/\.?0+$/, "");
}

function commit(): number | null {
  const n = parse(visible.value);
  if (n === null || n <= 0) {
    cents.value = 0;
    return null;
  }
  cents.value = Math.round(n * 100);
  return n;
}

function onFocus(e: FocusEvent) {
  focused.value = true;
  const n = parse(visible.value);
  if (n !== null) visible.value = rawForEdit(n);
  (e.target as HTMLInputElement).select();
}
function onInput() {
  commit();
}
function onBlur() {
  focused.value = false;
  const n = commit();
  visible.value = n === null ? "" : format(n);
}

// Re-render formatted when the currency changes (e.g. group switch) or when an
// external value is injected (edit flows) while not actively editing.
watch(
  () => props.currency,
  () => {
    if (!focused.value && cents.value > 0) visible.value = format(cents.value / 100);
  },
);
watch(cents, (v) => {
  if (!focused.value) visible.value = v > 0 ? format(v / 100) : "";
});

onMounted(() => {
  if (cents.value > 0) visible.value = format(cents.value / 100);
});
</script>

<template>
  <div>
    <label class="field" :for="inputId">
      <input
        :id="inputId"
        v-model="visible"
        type="text"
        inputmode="decimal"
        :required="required"
        placeholder=" "
        class="field-input field-input-currency"
        @focus="onFocus"
        @input="onInput"
        @blur="onBlur"
      />
      <span class="field-label" :data-required="required ? '' : undefined">{{ label }}</span>
    </label>
    <p v-if="error" class="field-error">{{ error }}</p>
  </div>
</template>
