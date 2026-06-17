<script setup lang="ts">
// Confirmation modal. Controlled via v-model:open. Emits `confirm` when the
// accept button is pressed; the parent owns the action that runs on confirm.
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
  // Emit before closing: parents that derive `open` from a target ref and
  // clear it on `update:open=false` (RecurringView, GroupSettingsView) read
  // that ref inside their `confirm` handler, so the close must not clear it
  // first.
  emit("confirm");
  open.value = false;
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
