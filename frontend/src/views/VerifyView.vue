<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

const { verifyEmail, resendVerification } = useAuth();
const route = useRoute();
const router = useRouter();

const email = computed(() =>
  typeof route.query.email === "string" ? route.query.email : "",
);

const code = ref("");
const submitting = ref(false);
const errorCode = ref<string | null>(null);
const resent = ref(false);

const errorMessages: Record<string, string> = {
  invalid: "That code didn't match. Try again or request a new one.",
  expired: "Your code expired. Tap 'Send a new code' to receive a fresh one.",
  rate_limited: "Too many attempts. Request a new code and try again.",
};

async function onSubmit() {
  errorCode.value = null;
  submitting.value = true;
  const res = await verifyEmail(email.value, code.value);
  submitting.value = false;
  if (res.ok) {
    await router.replace({ path: "/login", query: { verified: "1" } });
  } else {
    errorCode.value = res.code ?? "invalid";
  }
}

async function onResend() {
  resent.value = false;
  errorCode.value = null;
  await resendVerification(email.value);
  resent.value = true;
}
</script>

<template>
  <AppLayout>
    <section class="mx-auto max-w-96 py-6">
      <h1 class="mb-2 text-2xl font-semibold">Check your inbox</h1>
      <p class="mb-6 text-sm text-muted-foreground">
        We sent a 6-digit code to <strong>{{ email || "your email" }}</strong>.
        Paste it below to finish creating your account.
      </p>

      <Alert v-if="errorCode" tone="error" class="mb-4">
        {{ errorMessages[errorCode] ?? "Could not verify. Please try again." }}
      </Alert>
      <Alert v-if="resent" tone="success" class="mb-4">
        New code sent. It may take a moment to arrive.
      </Alert>

      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
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
          class="code-input"
          error="Enter the 6-digit code from your email"
        />
        <button
          type="submit"
          class="btn-primary mt-2 self-end"
          :disabled="submitting"
        >
          Verify
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

<style scoped>
/* Targets the inner <input> rendered by the Field child component. */
:deep(.code-input) {
  text-align: center;
  letter-spacing: 0.4em;
}
</style>
