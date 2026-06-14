<script setup lang="ts">
// Step-up password modal for destructive actions. Controlled via
// v-model:open; emits `confirm` with the typed password. The parent owns the
// action. Ported from the Astro tier's PasswordPromptDialog + password-prompt.ts.
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
  <dialog ref="dialog" class="pp" aria-modal="true" :aria-label="title" @close="onClose">
    <div class="pp-body">
      <div class="pp-head">
        <h3 class="pp-title">{{ title }}</h3>
        <button type="button" class="pp-x" aria-label="Close" title="Close" @click="onCancel">
          <Icon name="xmark" :size="14" />
        </button>
      </div>
      <p class="pp-msg">{{ message }}</p>
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
      <p v-if="showError" class="pp-err">Password is required.</p>
      <div class="pp-actions">
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

<style scoped>
.pp {
  position: fixed;
  inset: 0;
  margin: auto;
  width: calc(100% - 2rem);
  max-width: 24rem;
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  background: var(--popover);
  color: var(--popover-foreground);
  padding: 0;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.35);
}
.pp::backdrop {
  background: var(--backdrop);
}
.pp-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}
.pp-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}
.pp-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.pp-x {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.pp-x:hover {
  background: var(--muted);
}
.pp-msg {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.pp-err {
  margin-top: -0.5rem;
  font-size: 0.875rem;
  color: var(--destructive);
}
.pp-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
@media (min-width: 640px) {
  .pp-actions {
    flex-direction: row;
    justify-content: flex-end;
  }
}
</style>
