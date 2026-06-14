<script setup lang="ts">
// Confirmation modal. Controlled via v-model:open. Emits `confirm` when the
// accept button is pressed. Ported from the Astro tier's ConfirmDialog +
// confirm-dialog.ts; the parent owns the action that runs on confirm.
import { ref, watch } from "vue";
import Icon from "@/components/Icon.vue";

const props = withDefaults(
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
const emit = defineEmits<{ confirm: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);

watch(open, (isOpen) => {
  const el = dialog.value;
  if (!el) return;
  if (isOpen && !el.open) el.showModal();
  else if (!isOpen && el.open) el.close();
});

function onConfirm() {
  open.value = false;
  emit("confirm");
}
function onCancel() {
  open.value = false;
}
// Native dialog close (Escape, backdrop) keeps the model in sync.
function onClose() {
  open.value = false;
}
</script>

<template>
  <dialog
    ref="dialog"
    class="confirm"
    aria-modal="true"
    :aria-label="title"
    @close="onClose"
  >
    <div class="confirm-body">
      <div class="confirm-head">
        <h3 class="confirm-title">{{ title }}</h3>
        <button type="button" class="confirm-x" aria-label="Close" title="Close" @click="onCancel">
          <Icon name="xmark" :size="14" />
        </button>
      </div>
      <p class="confirm-msg">{{ message }}</p>
      <div class="confirm-actions">
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
.confirm {
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
.confirm::backdrop {
  background: var(--backdrop);
}
.confirm-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}
.confirm-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}
.confirm-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.confirm-x {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.confirm-x:hover {
  background: var(--muted);
}
.confirm-msg {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.confirm-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
@media (min-width: 640px) {
  .confirm-actions {
    flex-direction: row;
    justify-content: flex-end;
  }
}
</style>
