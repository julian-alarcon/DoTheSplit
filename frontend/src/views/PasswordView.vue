<script setup lang="ts">
import { ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Field from "@/components/Field.vue";

const { changePassword, recoverPassword } = useAuth();
const router = useRouter();

const oldPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const error = ref<string | null>(null);
const submitting = ref(false);

const confirmField = ref<InstanceType<typeof Field> | null>(null);

watch([newPassword, confirmPassword], () => {
  const el = confirmField.value?.input;
  if (el) {
    el.setCustomValidity(confirmPassword.value === newPassword.value ? "" : "Both passwords must match");
  }
});

async function onSubmit() {
  error.value = null;
  submitting.value = true;
  const res = await changePassword(oldPassword.value, newPassword.value);
  submitting.value = false;
  if (res.ok) {
    await router.replace("/settings");
  } else {
    error.value = "Could not change the password. Check your current password and try again.";
  }
}

async function onRecover() {
  const res = await recoverPassword();
  if (res.ok && res.email) {
    await router.push({ path: "/reset", query: { email: res.email, from: "settings" } });
  }
}
</script>

<template>
  <AppLayout :back="{ to: '/settings', label: 'Settings' }">
    <div class="wrap">
      <h1 class="title">Change your password or recover your current one.</h1>

      <Alert v-if="error" tone="error">{{ error }}</Alert>

      <form class="form" @submit.prevent="onSubmit">
        <Field
          v-model="oldPassword"
          label="Current password"
          type="password"
          required
          autocomplete="current-password"
          error="Required"
        />
        <p class="hint">You must provide your current password in order to change it.</p>
        <Field
          v-model="newPassword"
          label="New password"
          type="password"
          required
          minlength="10"
          autocomplete="new-password"
          error="Password must be at least 10 characters"
        />
        <Field
          ref="confirmField"
          v-model="confirmPassword"
          label="Password confirmation"
          type="password"
          required
          minlength="10"
          autocomplete="new-password"
          error="Both passwords must match"
        />
        <div class="actions">
          <button type="button" class="btn-secondary" @click="onRecover">Recover password by email</button>
          <button type="submit" class="btn-primary" :disabled="submitting">Save password</button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  display: flex;
  max-width: 36rem;
  flex-direction: column;
  gap: 0.75rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
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
.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
  padding-top: 0.5rem;
}
</style>
