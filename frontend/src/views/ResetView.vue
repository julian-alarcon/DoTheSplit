<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

const { confirmPasswordReset, requestPasswordReset } = useAuth();
const route = useRoute();
const router = useRouter();

const email = computed(() =>
  typeof route.query.email === "string" ? route.query.email : "",
);
const fromSettings = computed(() => route.query.from === "settings");

const code = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const submitting = ref(false);
const errorCode = ref<string | null>(null);
const resent = ref(false);

const confirmField = ref<InstanceType<typeof Field> | null>(null);

const errorMessages: Record<string, string> = {
  invalid_code: "That code didn't match. Try again or request a new one.",
  code_expired: "Your code expired. Request a new one and try again.",
  too_many_attempts: "Too many attempts. Request a new code and try again.",
  invalid_credentials: "Password was reset, but auto-login failed. Try logging in.",
};

// Live confirmation match via native constraint validation (no preventDefault),
// using setCustomValidity on the confirm field.
watch([newPassword, confirmPassword], () => {
  const el = confirmField.value?.input;
  if (el) {
    el.setCustomValidity(
      confirmPassword.value === newPassword.value ? "" : "Both passwords must match",
    );
  }
});

async function onSubmit() {
  errorCode.value = null;
  submitting.value = true;
  const res = await confirmPasswordReset(email.value, code.value, newPassword.value);
  submitting.value = false;
  if (res.ok) {
    await router.replace("/groups");
  } else {
    errorCode.value = res.code ?? "invalid_code";
  }
}

async function onResend() {
  resent.value = false;
  errorCode.value = null;
  await requestPasswordReset(email.value);
  resent.value = true;
}
</script>

<template>
  <AppLayout>
    <section class="mx-auto max-w-96 py-6">
      <h1 class="mb-2 text-2xl font-semibold">Check your inbox</h1>
      <p class="mb-6 text-sm text-muted-foreground">
        {{
          fromSettings
            ? "We sent a 6-digit code to your email address. Paste it below along with the new password."
            : "If that email is registered, we sent a 6-digit code. Paste it below along with the new password."
        }}
      </p>

      <Alert v-if="errorCode" tone="error" class="mb-4">
        {{ errorMessages[errorCode] ?? "Could not reset the password. Please try again." }}
      </Alert>
      <Alert v-if="resent" tone="success" class="mb-4">
        New code sent. It may take a moment to arrive.
      </Alert>

      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
        <Field
          v-model="email"
          label="Email"
          type="email"
          required
          autocomplete="email"
          readonly
          error="Enter a valid email address"
        />
        <Field
          v-model="code"
          label="6-digit code"
          type="text"
          required
          inputmode="numeric"
          autocomplete="one-time-code"
          pattern="[0-9]{6}"
          maxlength="6"
          minlength="6"
          class="text-center tracking-[0.4em]"
          error="Enter the 6-digit code from your email"
        />
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
        <button
          type="submit"
          class="btn-primary mt-2 self-end"
          :disabled="submitting"
        >
          Set new password
        </button>
      </form>

      <button
        type="button"
        class="mt-4 cursor-pointer text-sm text-muted-foreground underline"
        @click="onResend"
      >
        Send a new code
      </button>
    </section>
  </AppLayout>
</template>
