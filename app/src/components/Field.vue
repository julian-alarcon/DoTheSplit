<script lang="ts">
// Attrs (required, type, pattern, autocomplete, etc.) go on the inner
// <input>, not the wrapper, so native constraint validation targets the
// control directly.
export default { inheritAttrs: false };
</script>

<script setup lang="ts">
// Floating-label text field. Wraps a native <input> in the .field bottom-line
// treatment: the label morphs up when the field is focused or filled, and a
// sibling .field-error appears once the input is :user-invalid (the only
// visible validation cue on Firefox for Android, which doesn't render the
// native constraint-validation tooltip).
//
// Relies entirely on native HTML constraint validation - pass `required`,
// `type`, `pattern`, `minlength`, etc. straight through. No preventDefault,
// no JS validation library.
//
// v-model binds the input value. `error` is the user-facing hint shown when
// the field is invalid; omit it to suppress the sibling error line.
import { useId } from "vue";

defineProps<{
  label: string;
  error?: string;
}>();

const model = defineModel<string>({ default: "" });
const inputId = useId();
</script>

<template>
  <div>
    <label class="field" :for="inputId">
      <input
        :id="inputId"
        v-model="model"
        class="field-input"
        placeholder=" "
        v-bind="$attrs"
      />
      <span class="field-label" :data-required="($attrs.required ?? false) !== false ? '' : undefined">
        {{ label }}
      </span>
    </label>
    <p v-if="error" class="field-error">{{ error }}</p>
  </div>
</template>
