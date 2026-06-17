<script setup lang="ts">
// Step-up password modal for destructive actions. Controlled via
// v-model:open; emits `confirm` with the typed password. The parent owns the
// action.
import { ref, watch } from "vue";
import Icon from "@/components/Icon.vue";

withDefaults(
  defineProps<{
    title: string;
    message: string;
    confirmLabel: string;
    confirmVariant?: "danger" | "primary";
    cancelLabel?: string;
    confirmIcon?: string;
  }>(),
  { confirmVariant: "danger", cancelLabel: "Cancel" },
);

const open = defineModel<boolean>("open", { default: false });
const emit = defineEmits<{ confirm: [password: string] }>();

const dialog = ref<HTMLDialogElement | null>(null);
const password = ref("");
const showError = ref(false);

watch(open, (isOpen) => {
  const el = dialog.value;
  if (!el) return;
  if (isOpen && !el.open) {
    password.value = "";
    showError.value = false;
    el.showModal();
  } else if (!isOpen && el.open) {
    el.close();
  }
});

function onConfirm() {
  if (!password.value) {
    showError.value = true;
    return;
  }
  open.value = false;
  emit("confirm", password.value);
}
function onCancel() {
  open.value = false;
}
function onClose() {
  open.value = false;
}
</script>

<template>
  <dialog
    ref="dialog"
    class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
    aria-modal="true"
    :aria-label="title"
    @close="onClose"
  >
    <div class="flex flex-col gap-4 p-5">
      <div class="flex items-start justify-between gap-3">
        <h3 class="text-lg font-medium">{{ title }}</h3>
        <button type="button" class="cursor-pointer rounded-md px-2 py-1 text-muted-foreground hover:bg-muted" aria-label="Close" title="Close" @click="onCancel">
          <Icon name="xmark" :size="14" />
        </button>
      </div>
      <p class="text-sm text-muted-foreground">{{ message }}</p>
      <label class="field">
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          class="field-input"
          placeholder=" "
          @keydown.enter.prevent="onConfirm"
        />
        <span class="field-label" data-required>Your password</span>
      </label>
      <p v-if="showError" class="-mt-2 text-sm text-destructive">Password is required.</p>
      <div class="flex flex-col gap-2 sm:flex-row sm:justify-end">
        <button type="button" class="btn-secondary" @click="onCancel">{{ cancelLabel }}</button>
        <button
          type="button"
          :class="confirmVariant === 'danger' ? 'btn-danger' : 'btn-primary'"
          @click="onConfirm"
        >
          <Icon v-if="confirmIcon" :name="confirmIcon" />
          <span>{{ confirmLabel }}</span>
        </button>
      </div>
    </div>
  </dialog>
</template>
