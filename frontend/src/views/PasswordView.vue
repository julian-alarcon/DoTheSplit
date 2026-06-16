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
    <div class="mx-auto flex max-w-xl flex-col gap-3">
      <h1 class="text-2xl font-semibold">Change your password or recover your current one.</h1>

      <Alert v-if="error" tone="error">{{ error }}</Alert>

      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
        <Field
          v-model="oldPassword"
          label="Current password"
          type="password"
          required
          autocomplete="current-password"
          error="Required"
        />
        <p class="-mt-1 text-xs text-muted-foreground">You must provide your current password in order to change it.</p>
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
        <div class="flex flex-wrap justify-end gap-2 pt-2">
          <button type="button" class="btn-secondary" @click="onRecover">Recover password by email</button>
          <button type="submit" class="btn-primary" :disabled="submitting">Save password</button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>
