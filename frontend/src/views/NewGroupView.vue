<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { createGroup, inviteMembers } from "@/composables/useGroups";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import CurrencySelect from "@/components/CurrencySelect.vue";
import Alert from "@/components/Alert.vue";

const router = useRouter();

const name = ref("");
const currency = ref("EUR");
const memberEmails = ref("");
const submitting = ref(false);
const error = ref(false);

async function onSubmit() {
  error.value = false;
  submitting.value = true;
  const res = await createGroup({ name: name.value, default_currency: currency.value });
  if (!res.ok || !res.group) {
    submitting.value = false;
    error.value = true;
    return;
  }
  const groupId = res.group.id;

  const emails = memberEmails.value
    .split(/[\n,;]+/)
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean);

  let failed = 0;
  if (emails.length > 0) failed = await inviteMembers(groupId, emails);

  submitting.value = false;
  await router.replace({
    path: `/groups/${groupId}`,
    query: failed > 0 ? { invite_failed: String(failed) } : {},
  });
}
</script>

<template>
  <AppLayout>
    <div class="wrap">
      <h1 class="title">New group</h1>
      <Alert v-if="error" tone="error" class="mb">
        Could not create the group. Please try again.
      </Alert>
      <form class="form" @submit.prevent="onSubmit">
        <Field
          v-model="name"
          label="Group name"
          type="text"
          required
          maxlength="80"
          error="Required"
        />

        <label class="field-select-row">
          <span>Default currency</span>
          <CurrencySelect v-model="currency" />
        </label>
        <p class="hint">
          Each group uses a single currency. DoTheSplit does not support
          multi-currency groups; use separate groups if you need to track
          expenses in different currencies.
        </p>

        <label class="field">
          <textarea
            v-model="memberEmails"
            rows="4"
            placeholder=" "
            class="field-input"
          ></textarea>
          <span class="field-label">Add members (optional)</span>
        </label>
        <p class="hint">
          One email per line. Only registered users can be added: others are
          skipped and you can retry from group settings.
        </p>

        <button type="submit" class="btn-primary submit" :disabled="submitting">
          Create group
        </button>
      </form>
    </div>
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  max-width: 28rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 1.5rem;
}
.mb {
  margin-bottom: 1rem;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.hint {
  margin-top: -0.25rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.submit {
  margin-top: 0.5rem;
  align-self: flex-end;
}
</style>
