<script setup lang="ts">
// Currency dropdown. Two optgroups: the hand-curated common list (one click
// for the 99% case) and the full Intl-derived list. Labels localize via the
// runtime locale. Defaults to EUR. v-model binds the ISO-4217 code.
import { computed } from "vue";
import { COMMON_CURRENCIES, currencyLabel, otherCurrencies } from "@/lib/currencies";

const model = defineModel<string>({ default: "EUR" });

// Resolve labels against the browser's own locale.
const locale = typeof navigator !== "undefined" ? navigator.language : "en-US";
const others = computed(() => otherCurrencies());
</script>

<template>
  <select v-model="model" class="field-select" v-bind="$attrs">
    <optgroup label="Common">
      <option v-for="c in COMMON_CURRENCIES" :key="c" :value="c">
        {{ currencyLabel(c, locale) }}
      </option>
    </optgroup>
    <optgroup label="All currencies">
      <option v-for="c in others" :key="c" :value="c">
        {{ currencyLabel(c, locale) }}
      </option>
    </optgroup>
  </select>
</template>
